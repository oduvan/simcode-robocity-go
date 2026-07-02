// Robot City Builder — starter controller (Go).
//
// The simplest thing that works: each robot flies OUTWARD from the Base to reveal
// the map, and flies back to recharge before its battery runs out. The Base
// doubles as a charging pad, so there's nothing to build — this is "hello, world".
//
// Whenever a robot is free it fires `idle`; you read its live state and give it
// its next command. Robots FLY over float coordinates and spend ENERGY doing it
// (run dry mid-flight and the robot is destroyed) — so we turn back to charge in
// time. Make it do more — mine, haul, build a city. See CLAUDE.md for the full SDK.
package main

import sc "github.com/lyabah/simcode-sdk-go"

// Each robot scouts a different direction, so together they fan out.
var dirs = [8][2]int{{1, 0}, {0, 1}, {-1, 0}, {0, -1}, {1, 1}, {-1, -1}, {1, -1}, {-1, 1}}

func main() {
	city := sc.New()
	city.On(sc.EventIdle, func(e sc.Event) {
		r := city.Robot(e.Robot)
		base := city.Base()
		if base == nil {
			return
		}
		bx, by := base.Position()
		x, y := r.Position()
		cx, cy := r.Cell()
		home := abs(x-float64(bx)) + abs(y-float64(by)) // ~energy needed to fly back

		// Turn back and charge while the battery can still get us home.
		if r.Energy() <= home+15 {
			if cx == bx && cy == by {
				r.Charge()
			} else {
				r.MoveTo(float64(bx), float64(by))
			}
			return
		}

		// Otherwise keep flying outward to discover new map.
		d := dirs[idSum(r.ID)%len(dirs)]
		r.MoveTo(x+float64(d[0])*5, y+float64(d[1])*5)
	})
	_ = city.Run()
}

func idSum(id string) (n int) {
	for _, c := range []byte(id) {
		n += int(c)
	}
	return
}

func abs(f float64) float64 {
	if f < 0 {
		return -f
	}
	return f
}
