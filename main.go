// Robot City Builder — starter controller (Go).
//
// Robots start EMPTY. The ONLY way to shape the city is with the code below: it
// reacts to `idle` events (a robot is free) and hands each robot its next command.
//
// The economy in one breath:
//   - A pre-placed Storage next to the Base holds your starting capital
//     (30 ore / 15 metal). Robots PickUp from it to get build materials.
//   - Mines you build (place a site on a resource spot with World().Build(
//     "mining", x, y), then Drop its 6-ore / 3-metal cost onto the site)
//     auto-produce ore or metal. Robots PickUp from a mine and haul it off.
//   - The Base is the quest hub: Drop ore/metal on it to fill the current quest;
//     meet it and the Base LEVELS UP to a harder one. Highest level = score. You
//     can't pick up from the Base; it doubles as a charging pad.
//   - A Flying Station (World().Build("flying_station", x, y)) is a charging pad
//     AND a robot factory: stock it (Drop ore/metal), then station.BuildRobot(1).
//   - Flying burns ENERGY; Charge() on any pad (Base or Flying Station). A robot
//     that runs dry mid-flight is destroyed — so approach() below never starts a
//     trip the robot can't finish and still reach a pad.
//
// Priority per idle robot:
//  1. energy guard    — get to a pad and charge before the battery dies,
//  2. place mines     — ensure an ore mine AND a metal mine exist,
//  3. fund sites      — carry materials to any half-built site (mine / station),
//  4. grow (once)     — after level 1, one robot builds a Flying Station + 1 robot,
//  5. haul the quest  — carry mine output to the Base to climb levels,
//  6. explore         — reveal the fog when there's nothing better to do.
//
// Persistent state lives in the durable, city-wide store (SetStore/GetStore) and
// per-robot memory — never package globals, which a code push resets on reload.
package main

import (
	"fmt"
	"math"

	sc "github.com/lyabah/simcode-sdk-go"
)

const (
	kitOre, kitMetal         = 6, 3  // the Mining recipe (what one mine costs)
	stationOre, stationMetal = 4, 2  // the Flying Station recipe
	robotOre, robotMetal     = 12, 6 // what a station spends to build one robot
	carry                    = 10    // robot inventory capacity (ore + metal)
	energyMargin             = 20    // battery kept spare on top of a round trip
)

// Compass headings for exploration; a robot advances one per trip so the fleet
// fans out across the map instead of re-treading a single line.
var dirs = [8][2]int{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}}

var city *sc.City

func main() {
	city = sc.New()
	city.On(sc.EventIdle, onIdle)
	_ = city.Run()
}

// --------------------------------------------------------------------------- //
// the controller — one handler drives the whole fleet
// --------------------------------------------------------------------------- //
func onIdle(e sc.Event) {
	r := city.Robot(e.Robot)
	base := city.Base()
	if base == nil {
		return
	}
	pos := robotPos(r)

	// 1) ENERGY GUARD — if we're low and away from a pad, get to one and charge.
	if pads := chargingPads(); len(pads) > 0 {
		pad := nearestPad(pos, pads)
		if r.Energy() <= distf(pos, pad)+energyMargin {
			if robotCell(r) == cellOf(pad) {
				r.Charge()
			} else {
				r.MoveTo(pad[0], pad[1])
			}
			return
		}
	}

	// 2) PLACE MINES — make sure an ore mine AND a metal mine exist. Placing a
	//    site is free and robot-independent; a later step funds it.
	have := mineResources()
	for _, res := range []string{"ore", "metal"} {
		if have[res] {
			continue
		}
		if spot, ok := freeSpot(pos, res); ok {
			city.World().Build(sc.BuildingMining, spot[0], spot[1])
			r.Log(fmt.Sprintf("placing a %s mine at %v", res, spot))
			return
		}
		// No known spot for this resource yet -> we'll explore below to find one.
	}

	// 3) FUND SITES — carry materials to any half-built mine or station.
	if fundSites(r) {
		return
	}

	// 4) GROW (optional) — one robot expands the fleet once we're past level 1.
	if grow(r, base) {
		return
	}

	// 5) HAUL THE QUEST — carry mine output to the Base to climb levels. Deliver
	//    a carried load first, then pick up more of whatever the quest needs most.
	q := base.Quest()
	needO := cint(q, "required", "ore") - cint(q, "progress", "ore")
	needM := cint(q, "required", "metal") - cint(q, "progress", "metal")
	inv := r.Inventory()
	if inv.Ore > 0 || inv.Metal > 0 {
		if (inv.Ore > 0 && needO > 0) || (inv.Metal > 0 && needM > 0) {
			if approach(r, bpos(base)) == "moving" {
				return
			}
			r.Drop() // Base accepts up to the requirement; excess stays with us
			return
		}
		if bank := nearestBuilding(pos, sc.BuildingStorage, nil); bank != nil {
			if approach(r, bpos(bank)) != "moving" {
				r.Drop() // Base wants none of it -> bank it so Storage can fund kits
			}
		}
		return
	}
	order := []string{"ore", "metal"}
	if needM > needO {
		order = []string{"metal", "ore"}
	}
	for _, res := range order {
		need := needO
		if res == "metal" {
			need = needM
		}
		if need > 0 && haul(r, res, bpos(base), need) {
			return
		}
	}

	// 6) Nothing useful to do -> explore and reveal more of the map.
	explore(r)
}

// --------------------------------------------------------------------------- //
// reusable jobs (each returns true when it has claimed this robot's turn)
// --------------------------------------------------------------------------- //

// fundSites carries materials to the nearest half-built construction site (a mine
// or a Flying Station). Empty robots fetch the exact recipe from Storage; loaded
// robots drop what a site still needs. Sites self-complete once supplied.
func fundSites(r *sc.Robot) bool {
	var sites []*sc.Building
	for _, b := range city.Buildings() {
		if b.Status() == sc.StatusConstructing && b.Construction() != nil {
			sites = append(sites, b)
		}
	}
	if len(sites) == 0 {
		return false
	}
	pos := robotPos(r)
	inv := r.Inventory()

	if inv.Ore > 0 || inv.Metal > 0 { // loaded -> deliver to a site that wants it
		var target *sc.Building
		for _, b := range sites {
			con := b.Construction()
			needO := cint(con, "required", "ore") - cint(con, "delivered", "ore")
			needM := cint(con, "required", "metal") - cint(con, "delivered", "metal")
			if (inv.Ore > 0 && needO > 0) || (inv.Metal > 0 && needM > 0) {
				if target == nil || distf(pos, bpos(b)) < distf(pos, bpos(target)) {
					target = b
				}
			}
		}
		if target == nil {
			return false // nothing here needs our load -> let haul use it
		}
		if approach(r, bpos(target)) == "moving" {
			return true
		}
		r.Drop() // site takes only its recipe; excess stays with us
		return true
	}

	// empty -> fetch the nearest site's remaining recipe from a stocked Storage
	site := sites[0]
	for _, b := range sites[1:] {
		if distf(pos, bpos(b)) < distf(pos, bpos(site)) {
			site = b
		}
	}
	con := site.Construction()
	needO := cint(con, "required", "ore") - cint(con, "delivered", "ore")
	needM := cint(con, "required", "metal") - cint(con, "delivered", "metal")
	src := nearestBuilding(pos, sc.BuildingStorage, func(b *sc.Building) bool {
		return b.Storage().Ore >= needO && b.Storage().Metal >= needM
	})
	if src == nil {
		return false // Storage can't fund it yet -> do other work
	}
	if approach(r, bpos(src)) == "moving" {
		return true
	}
	r.PickUp(needO, needM)
	return true
}

// haul picks up `resource` from the nearest stocked mine and drops it at `target`
// (the Base quest or a station being stocked). Returns true while busy.
func haul(r *sc.Robot, resource string, target [2]float64, amount int) bool {
	inv := r.Inventory()
	have := inv.Ore
	if resource == "metal" {
		have = inv.Metal
	}
	if have > 0 {
		if approach(r, target) == "moving" {
			return true
		}
		r.Drop(inv.Ore, inv.Metal)
		return true
	}
	mine := nearestBuilding(robotPos(r), sc.BuildingMining, func(b *sc.Building) bool {
		sp := b.Spot()
		return sp != nil && sp.Resource == resource && b.Storage().Ore+b.Storage().Metal > 0
	})
	if mine == nil {
		return false
	}
	if approach(r, bpos(mine)) == "moving" {
		return true
	}
	take := amount
	if take > carry {
		take = carry
	}
	if take < 1 {
		take = 1
	}
	if resource == "ore" {
		r.PickUp(take, 0)
	} else {
		r.PickUp(0, take)
	}
	return true
}

// grow is OPTIONAL fleet growth: after level 1, ONE robot (the "grow lead")
// builds a Flying Station, stocks it, and produces one extra robot. Everyone else
// keeps feeding the quest, so growth never starves it. State is in the store.
func grow(r *sc.Robot, base *sc.Building) bool {
	if base.Level() < 2 || storeBool("grow_done") {
		return false
	}
	lead, ok := storeStr("grow_lead")
	if !ok {
		lead = r.ID
		city.SetStore("grow_lead", lead)
	}
	if r.ID != lead {
		return false // everyone else keeps hauling
	}

	stations := city.Stations()
	if len(stations) == 0 {
		if !storeBool("station_placed") { // place the site once (fundSites builds it)
			bx, by := base.Position()
			site := freeCellNear([2]int{bx, by})
			city.World().Build(sc.BuildingFlyingStation, site[0], site[1])
			city.SetStore("station_placed", true)
			r.Log(fmt.Sprintf("growth: placing a Flying Station at %v", site))
		}
		return false // let fundSites/haul drive this robot
	}
	st := stations[0]
	if st.Status() != sc.StatusActive {
		return false // still under construction -> fundSites handles it
	}
	if st.Storage().Ore >= robotOre && st.Storage().Metal >= robotMetal {
		st.BuildRobot(1) // stocked -> manufacture a robot
		city.SetStore("grow_done", true)
		r.Log("growth: station stocked — building a new robot")
		return true
	}
	want, need := "ore", robotOre-st.Storage().Ore
	if st.Storage().Ore >= robotOre {
		want, need = "metal", robotMetal-st.Storage().Metal
	}
	return haul(r, want, bpos(st), need)
}

// --------------------------------------------------------------------------- //
// movement: fly toward a target, but charge first if we couldn't get home
// --------------------------------------------------------------------------- //

// approach flies `r` toward `target`, topping up energy first when the round trip
// would strand it. Returns "arrived" when already on the target cell (the caller
// then acts), or "moving" when a flight was issued (toward the target OR a pad).
func approach(r *sc.Robot, target [2]float64) string {
	tc := cellOf(target)
	if robotCell(r) == tc {
		return "arrived"
	}
	if pads := chargingPads(); len(pads) > 0 {
		home := math.Inf(1)
		for _, p := range pads {
			if d := distf(target, p); d < home {
				home = d
			}
		}
		if r.Energy() < distf(robotPos(r), target)+home+energyMargin {
			pad := nearestPad(robotPos(r), pads)
			if robotCell(r) == cellOf(pad) {
				r.Charge()
			} else {
				r.MoveTo(pad[0], pad[1])
			}
			return "moving"
		}
	}
	r.MoveTo(float64(tc[0]), float64(tc[1]))
	return "moving"
}

// explore flies into the fog to reveal new ground; each trip picks a fresh heading.
func explore(r *sc.Robot) {
	mem := r.Memory()
	n := toInt(mem["trip"]) + 1
	mem["trip"] = n
	r.SetMemory(mem)
	d := dirs[(idSum(r.ID)+n)%len(dirs)]
	pos := robotPos(r)
	approach(r, [2]float64{pos[0] + float64(d[0])*6, pos[1] + float64(d[1])*6})
}

// --------------------------------------------------------------------------- //
// small read-only helpers over the live world
// --------------------------------------------------------------------------- //

func chargingPads() [][2]float64 {
	var pads [][2]float64
	if b := city.Base(); b != nil {
		pads = append(pads, bpos(b))
	}
	for _, s := range city.Stations() {
		if s.Status() == sc.StatusActive {
			pads = append(pads, bpos(s))
		}
	}
	return pads
}

func occupiedCells() map[[2]int]bool {
	occ := map[[2]int]bool{}
	for _, b := range city.Buildings() {
		x, y := b.Position()
		w, h := b.Footprint()
		for dx := 0; dx < w; dx++ {
			for dy := 0; dy < h; dy++ {
				occ[[2]int{x + dx, y + dy}] = true
			}
		}
	}
	return occ
}

// mineResources returns the resources we have a PRODUCTIVE mine for (built or
// under construction, spot not yet exhausted). A mine whose spot has run dry no
// longer counts, so the controller builds a fresh one — the climb never stalls.
func mineResources() map[string]bool {
	res := map[string]bool{}
	for _, b := range city.Buildings() {
		if b.Type() != sc.BuildingMining {
			continue
		}
		if sp := b.Spot(); sp != nil && sp.Resource != "" && sp.Remaining > 0 {
			res[sp.Resource] = true
		}
	}
	return res
}

// freeSpot returns the nearest discovered, still-rich spot of `resource` with no
// building on it yet.
func freeSpot(pos [2]float64, resource string) ([2]int, bool) {
	occ := occupiedCells()
	var best [2]int
	found := false
	for _, c := range city.World().Spots() {
		if c.Spot == nil || c.Spot.Resource != resource || c.Spot.Remaining <= 0 {
			continue
		}
		cell := [2]int{c.X, c.Y}
		if occ[cell] {
			continue
		}
		cf := [2]float64{float64(c.X), float64(c.Y)}
		if !found || distf(pos, cf) < distf(pos, [2]float64{float64(best[0]), float64(best[1])}) {
			best, found = cell, true
		}
	}
	return best, found
}

// nearestBuilding returns the nearest ACTIVE building of `kind`; `want` may filter.
func nearestBuilding(pos [2]float64, kind string, want func(*sc.Building) bool) *sc.Building {
	var best *sc.Building
	for _, b := range city.Buildings() {
		if b.Type() != kind || b.Status() != sc.StatusActive {
			continue
		}
		if want != nil && !want(b) {
			continue
		}
		if best == nil || distf(pos, bpos(b)) < distf(pos, bpos(best)) {
			best = b
		}
	}
	return best
}

// freeCellNear returns a building-free cell a few steps from `center`.
func freeCellNear(center [2]int) [2]int {
	occ := occupiedCells()
	for _, radius := range []int{3, 4, 5, 6} {
		for _, d := range dirs {
			cell := [2]int{center[0] + d[0]*radius, center[1] + d[1]*radius}
			if !occ[cell] {
				return cell
			}
		}
	}
	return [2]int{center[0] + 3, center[1] + 3}
}

// --------------------------------------------------------------------------- //
// tiny math / conversion / store helpers
// --------------------------------------------------------------------------- //

func robotPos(r *sc.Robot) [2]float64 { x, y := r.Position(); return [2]float64{x, y} }
func robotCell(r *sc.Robot) [2]int    { x, y := r.Cell(); return [2]int{x, y} }
func bpos(b *sc.Building) [2]float64  { x, y := b.Position(); return [2]float64{float64(x), float64(y)} }
func distf(a, b [2]float64) float64   { return math.Abs(a[0]-b[0]) + math.Abs(a[1]-b[1]) }
func cellOf(p [2]float64) [2]int      { return [2]int{int(math.Round(p[0])), int(math.Round(p[1]))} }

func nearestPad(pos [2]float64, pads [][2]float64) [2]float64 {
	best := pads[0]
	for _, p := range pads[1:] {
		if distf(pos, p) < distf(pos, best) {
			best = p
		}
	}
	return best
}

func idSum(id string) int {
	n := 0
	for _, c := range []byte(id) {
		n += int(c)
	}
	return n
}

// cint reads m[k1][k2] as an int from a nested raw JSON bag (Quest / Construction).
func cint(m map[string]any, k1, k2 string) int {
	if m == nil {
		return 0
	}
	sub, _ := m[k1].(map[string]any)
	return toInt(sub[k2])
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	case int64:
		return int(n)
	}
	return 0
}

func storeStr(key string) (string, bool) {
	if v, ok := city.GetStore(key); ok {
		s, ok2 := v.(string)
		return s, ok2
	}
	return "", false
}

func storeBool(key string) bool {
	if v, ok := city.GetStore(key); ok {
		b, _ := v.(bool)
		return b
	}
	return false
}
