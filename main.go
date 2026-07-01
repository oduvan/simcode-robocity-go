// Robot City Builder — starter controller (Go).
//
// One program drives the whole fleet by id. The game is event-driven: whenever a
// robot is free it fires `idle`, and you give it its next command.
//
// The world (redesigned): robots FLY in straight lines over float coordinates and
// spend ENERGY doing it — run out mid-flight and the robot is destroyed. Mining
// and construction are AUTONOMOUS: you place a build site with world.Build(...)
// and robots only HAUL resources to it; a Mining building then digs on its own.
// Robots recharge by landing on a Flying Station and calling Charge().
//
// This starter loop, per robot:
//
//	holds a starter kit  -> fly to a resource spot, place a Mining site, drop the kit
//	empty-handed         -> haul a Mining building's output to the Base (which builds
//	                        more robots) or to a build site that still needs it
//	low on energy        -> fly to a Flying Station and charge
//
// Read it, then make it smarter. See CLAUDE.md for the full SDK + rules.
package main

import sc "github.com/lyabah/simcode-sdk-go"

const (
	lowEnergy = 30.0 // recharge below this much battery
	mineOre   = 6    // a Mining site needs 6 ore + 3 metal
	mineMetal = 3
)

var stepCount = map[string]int{} // robot id -> explore counter (so robots fan out)

func main() {
	city := sc.New()

	city.On(sc.EventIdle, func(e sc.Event) {
		r := city.Robot(e.Robot)
		base := city.Base()
		if base == nil {
			return
		}
		rx, ry := r.Cell()
		inv := r.Inventory()

		// 0. Low battery -> land on a Flying Station and charge (if one exists).
		if st := activeStation(city, r); st != nil && r.Energy() <= lowEnergy {
			sx, sy := st.Position()
			if rx == sx && ry == sy {
				r.Charge()
			} else {
				r.MoveTo(float64(sx), float64(sy))
			}
			return
		}

		// 1. Holding a full starter kit -> turn a resource spot into a Mining site.
		if inv.Ore >= mineOre && inv.Metal >= mineMetal {
			want := "metal"
			if base.Storage().Ore <= base.Storage().Metal {
				want = "ore"
			}
			if spot, ok := unbuiltSpot(city, r, want); ok {
				if rx == spot[0] && ry == spot[1] {
					city.World().Build(sc.BuildingMining, spot[0], spot[1]) // self-builds once supplied
					r.Drop(mineOre, mineMetal)
				} else {
					r.MoveTo(float64(spot[0]), float64(spot[1]))
				}
			} else {
				explore(city, r) // nothing to claim — reveal more map
			}
			return
		}

		// 2. Carrying mined output -> deliver to a build site that needs it, else Base.
		if inv.Ore+inv.Metal > 0 {
			target, toBase := deliverTarget(city, base, inv)
			if rx == target[0] && ry == target[1] {
				r.Drop()
				if toBase {
					base.BuildRobot(1) // feed growth at the Base
				}
			} else {
				r.MoveTo(float64(target[0]), float64(target[1]))
			}
			return
		}

		// 3. Empty-handed -> haul a stocked mine's output.
		if m, ok := stockedMine(city, r); ok {
			mx, my := m.Position()
			if rx == mx && ry == my {
				r.PickUp()
			} else {
				r.MoveTo(float64(mx), float64(my))
			}
			return
		}

		// 4. Idle with nothing to haul -> ensure a Flying Station exists, else explore.
		if activeStation(city, r) == nil && !anyStation(city) {
			if p, ok := emptyNearBase(city, base); ok {
				city.World().Build(sc.BuildingFlyingStation, p[0], p[1]) // haulers will supply it
				return
			}
		}
		explore(city, r)
	})

	_ = city.Run()
}

// --- reading the world (fresh each event) ----------------------------------

func builtCells(city *sc.City) map[[2]int]bool {
	out := map[[2]int]bool{}
	for _, b := range city.Buildings() {
		bx, by := b.Position()
		out[[2]int{bx, by}] = true
	}
	return out
}

// unbuiltSpot returns the nearest discovered spot with nothing built on it,
// preferring the resource `want`.
func unbuiltSpot(city *sc.City, r *sc.Robot, want string) ([2]int, bool) {
	built := builtCells(city)
	rx, ry := r.Position()
	var best [2]int
	bestRank, bestDist, found := 2, 0, false
	for _, s := range city.World().Spots() {
		p := [2]int{s.X, s.Y}
		if built[p] || s.Spot == nil || s.Spot.Remaining <= 0 {
			continue
		}
		rank := 1
		if s.Spot.Resource == want {
			rank = 0
		}
		d := absF(rx-float64(s.X)) + absF(ry-float64(s.Y))
		if !found || rank < bestRank || (rank == bestRank && d < float64(bestDist)) {
			best, bestRank, bestDist, found = p, rank, int(d), true
		}
	}
	return best, found
}

// stockedMine returns the nearest active Mining building holding output to haul.
func stockedMine(city *sc.City, r *sc.Robot) (*sc.Building, bool) {
	rx, ry := r.Position()
	var best *sc.Building
	bestDist, found := 0.0, false
	for _, b := range city.Buildings() {
		if b.Type() != sc.BuildingMining || b.Status() != sc.StatusActive {
			continue
		}
		s := b.Storage()
		if s.Ore+s.Metal < 6 {
			continue
		}
		bx, by := b.Position()
		d := absF(rx-float64(bx)) + absF(ry-float64(by))
		if !found || d < bestDist {
			best, bestDist, found = b, d, true
		}
	}
	return best, found
}

// activeStation returns the nearest active Flying Station, or nil.
func activeStation(city *sc.City, r *sc.Robot) *sc.Building {
	rx, ry := r.Position()
	var best *sc.Building
	bestDist := 0.0
	for _, b := range city.Buildings() {
		if b.Type() != sc.BuildingFlyingStation || b.Status() != sc.StatusActive {
			continue
		}
		bx, by := b.Position()
		d := absF(rx-float64(bx)) + absF(ry-float64(by))
		if best == nil || d < bestDist {
			best, bestDist = b, d
		}
	}
	return best
}

// anyStation reports whether a Flying Station exists or is being built.
func anyStation(city *sc.City) bool {
	for _, b := range city.Buildings() {
		if b.Type() == sc.BuildingFlyingStation {
			return true
		}
	}
	return false
}

// deliverTarget returns where a hauler should drop: a construction site that
// still needs a carried resource, else the Base (toBase=true).
func deliverTarget(city *sc.City, base *sc.Building, inv sc.Inventory) (cell [2]int, toBase bool) {
	for _, b := range city.Buildings() {
		if b.Status() != sc.StatusConstructing {
			continue
		}
		needOre, needMetal := siteNeeds(b)
		if (needOre > 0 && inv.Ore > 0) || (needMetal > 0 && inv.Metal > 0) {
			bx, by := b.Position()
			return [2]int{bx, by}, false
		}
	}
	bx, by := base.Position()
	return [2]int{bx, by}, true
}

func siteNeeds(b *sc.Building) (needOre, needMetal int) {
	c := b.Construction()
	req, del := asMap(c["required"]), asMap(c["delivered"])
	return num(req["ore"]) - num(del["ore"]), num(req["metal"]) - num(del["metal"])
}

// emptyNearBase returns a nearby cell with no building and no spot (for a station).
func emptyNearBase(city *sc.City, base *sc.Building) ([2]int, bool) {
	bx, by := base.Position()
	built := builtCells(city)
	spot := map[[2]int]bool{}
	for _, s := range city.World().Spots() {
		spot[[2]int{s.X, s.Y}] = true
	}
	for radius := 1; radius < 8; radius++ {
		for dy := -radius; dy <= radius; dy++ {
			for dx := -radius; dx <= radius; dx++ {
				if abs(dx) != radius && abs(dy) != radius {
					continue
				}
				p := [2]int{bx + dx, by + dy}
				if !built[p] && !spot[p] {
					return p, true
				}
			}
		}
	}
	return [2]int{}, false
}

// explore flies outward to reveal more of the endless map (robots fan out).
func explore(city *sc.City, r *sc.Robot) {
	i := stepCount[r.ID]
	stepCount[r.ID] = i + 1
	dirs := [4][2]int{{6, 0}, {0, 6}, {-6, 0}, {0, -6}}
	d := dirs[(i+len(r.ID))%4]
	x, y := r.Position()
	r.MoveTo(x+float64(d[0]), y+float64(d[1]))
}

func num(v any) int {
	if f, ok := v.(float64); ok {
		return int(f)
	}
	return 0
}

func asMap(v any) map[string]any {
	if m, ok := v.(map[string]any); ok {
		return m
	}
	return map[string]any{}
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func absF(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
