// SimCode city controller — a MINIMAL starting point.
//
// This starter does one thing on purpose: it keeps the robots alive and flies them
// around to explore the map. It does NOT mine, build, haul, or climb Base levels —
// that is for YOU to add.
//
// Read CLAUDE.md for the whole game (the goal, the buildings, the full SDK API) and
// grow this controller from here. The idea is simple: EventIdle fires whenever a robot
// needs its next order, so decide what the robot should do and issue one command.
package main

import (
	sc "github.com/lyabah/simcode-sdk-go"
)

// Compass headings. A robot advances one heading per trip (kept in its memory) so the
// fleet fans out across the map instead of re-treading a single line into the fog.
var dirs = [8][2]int{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}}

const (
	exploreHop = 5  // world units to fly per exploration step
	lowBattery = 45 // below this, head to the Base (a charging pad) and recharge
)

var city *sc.City

func main() {
	city = sc.New()
	city.On(sc.EventIdle, onIdle)
	_ = city.Run()
}

func onIdle(e sc.Event) {
	r := city.Robot(e.Robot)

	// Stay alive: a robot that runs its battery to zero mid-flight is destroyed. When
	// low, fly to the Base at the origin (it doubles as a charging pad) and recharge.
	if r.Energy() < lowBattery {
		if cx, cy := r.Cell(); cx == 0 && cy == 0 {
			r.Charge()
		} else {
			r.MoveTo(0, 0)
		}
		return
	}

	// Otherwise explore: fly a short hop along a rotating heading. Flying reveals the
	// map (~5 cells around the robot), so this is how you uncover resource spots.
	n := 0
	if v, ok := r.Memory()["hop"].(float64); ok {
		n = int(v)
	}
	n++
	r.SetMemory(map[string]any{"hop": n})
	d := dirs[n%len(dirs)]
	x, y := r.Position()
	r.MoveTo(x+float64(d[0]*exploreHop), y+float64(d[1]*exploreHop))
}
