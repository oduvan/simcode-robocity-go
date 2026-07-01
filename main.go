// Robot City Builder — starter controller (Go).
//
// One program drives the whole fleet by id. Whenever a robot is free it fires
// `idle`; you read its live state and give it its next command.
//
// Robots FLY (float coords) and spend ENERGY — run dry mid-flight and a robot is
// destroyed. Mining and construction are AUTONOMOUS: place a site with
// city.World().Build(...) and robots only HAUL to it; a Mining building then digs
// by itself. Recharge with r.Charge() on the Flying Station (one sits by the Base).
//
// The loop, per robot: charge if low -> spend a starter kit to build a mine ->
// haul mine output to the Base (which builds more robots) -> repeat. Read it,
// then make it smarter. See CLAUDE.md for the full SDK.
package main

import sc "github.com/lyabah/simcode-sdk-go"

const kitOre, kitMetal = 6, 3 // a Mining site costs 6 ore + 3 metal — one starter kit

func main() {
	city := sc.New()
	city.On(sc.EventIdle, func(e sc.Event) {
		r := city.Robot(e.Robot)
		base := city.Base()
		if base == nil {
			return
		}
		x, y := r.Position()
		cx, cy := r.Cell()
		inv := r.Inventory()

		// Low battery -> land on the Flying Station and charge (one sits by the Base).
		if st := nearest(city, sc.BuildingFlyingStation, x, y); st != nil && r.Energy() < 35 {
			sx, sy := st.Position()
			if cx == sx && cy == sy {
				r.Charge()
			} else {
				r.MoveTo(float64(sx), float64(sy))
			}
			return
		}

		// Holding a starter kit -> build a mine on a spot. Split the first robots
		// by id so the Base gets BOTH ore and metal (it needs both).
		if inv.Ore >= kitOre && inv.Metal >= kitMetal {
			s, want := base.Storage(), "metal"
			switch {
			case s.Ore < s.Metal:
				want = "ore"
			case s.Ore == s.Metal && idSum(r.ID)%2 == 1:
				want = "ore"
			}
			if spot, ok := freeSpot(city, want, x, y); !ok {
				r.MoveTo(x+6, y) // nothing known -> explore
			} else if cx == spot[0] && cy == spot[1] {
				city.World().Build(sc.BuildingMining, spot[0], spot[1]) // self-builds
				r.Drop(kitOre, kitMetal)
			} else {
				r.MoveTo(float64(spot[0]), float64(spot[1]))
			}
			return
		}

		// Carrying mined output -> haul to the Base, which produces more robots.
		if inv.Ore+inv.Metal > 0 {
			bx, by := base.Position()
			if cx == bx && cy == by {
				r.Drop()
				base.BuildRobot(1)
			} else {
				r.MoveTo(float64(bx), float64(by))
			}
			return
		}

		// Empty -> haul from a stocked mine, else explore to reveal more map.
		if m := stockedMine(city, x, y); m != nil {
			mx, my := m.Position()
			if cx == mx && cy == my {
				r.PickUp()
			} else {
				r.MoveTo(float64(mx), float64(my))
			}
			return
		}
		r.MoveTo(x+6, y)
	})
	_ = city.Run()
}

// nearest returns the closest active building of a type to (x, y), or nil.
func nearest(city *sc.City, typ string, x, y float64) *sc.Building {
	var best *sc.Building
	bd := 0.0
	for _, b := range city.Buildings() {
		if b.Type() != typ || b.Status() != sc.StatusActive {
			continue
		}
		bx, by := b.Position()
		if d := dist(x, y, bx, by); best == nil || d < bd {
			best, bd = b, d
		}
	}
	return best
}

// freeSpot returns the nearest discovered, unbuilt spot, preferring `want`.
func freeSpot(city *sc.City, want string, x, y float64) ([2]int, bool) {
	built := map[[2]int]bool{}
	for _, b := range city.Buildings() {
		bx, by := b.Position()
		built[[2]int{bx, by}] = true
	}
	var best [2]int
	bd, rank, found := 0.0, 2, false
	for _, s := range city.World().Spots() {
		p := [2]int{s.X, s.Y}
		if built[p] || s.Spot == nil || s.Spot.Remaining <= 0 {
			continue
		}
		rk := 1
		if s.Spot.Resource == want {
			rk = 0
		}
		if d := dist(x, y, s.X, s.Y); !found || rk < rank || (rk == rank && d < bd) {
			best, bd, rank, found = p, d, rk, true
		}
	}
	return best, found
}

// stockedMine returns the nearest active mine holding output worth hauling, or nil.
func stockedMine(city *sc.City, x, y float64) *sc.Building {
	var best *sc.Building
	bd := 0.0
	for _, b := range city.Buildings() {
		if b.Type() != sc.BuildingMining || b.Status() != sc.StatusActive {
			continue
		}
		if s := b.Storage(); s.Ore+s.Metal < 6 {
			continue
		}
		bx, by := b.Position()
		if d := dist(x, y, bx, by); best == nil || d < bd {
			best, bd = b, d
		}
	}
	return best
}

func dist(x, y float64, cx, cy int) float64 { return abs(x-float64(cx)) + abs(y-float64(cy)) }
func idSum(id string) (n int)               { for _, c := range []byte(id) { n += int(c) }; return }
func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
