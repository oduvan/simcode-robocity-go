// SimCode city controller — a MINIMAL starting point.
//
// This starter does one thing on purpose: it keeps the robots alive and flies them
// around to explore the map. It does NOT mine, build, haul, or climb Base levels —
// that is for YOU to add.
//
// Note: robots wear out two ways — running the battery to zero mid-flight (avoidable:
// charge in time, handled below) AND simply flying too far. Every robot has a max
// cumulative flight distance (its lifespan, r.LifeRemaining() / r.LifeMax()); once it
// has flown that far it EXPIRES and is removed (EventRobotExpired). This starter does
// NOT replace expired robots, mine, process, repair, or level up the Base — growing and
// replacing the fleet and running the whole economy (robot types, mining, the factory
// tree, mechanic repairs, Base leveling) is YOUR job.
//
// Read CLAUDE.md for the whole game (the goal, the buildings, the full SDK API) and
// grow this controller from here. The idea is simple: EventIdle fires whenever a robot
// needs its next order, so decide what the robot should do and issue one command.
package main

import (
	"math"

	sc "github.com/lyabah/simcode-sdk-go"
)

// Compass headings. A robot advances one heading per trip (kept in its memory) so the
// fleet fans out across the map instead of re-treading a single line into the fog.
var dirs = [8][2]int{{1, 0}, {1, 1}, {0, 1}, {-1, 1}, {-1, 0}, {-1, -1}, {0, -1}, {1, -1}}

const (
	exploreHop   = 5  // world units to fly per exploration step
	chargeMargin = 15 // spare battery to keep beyond the planned flight
)

var city *sc.City

func main() {
	city = sc.New()
	city.On(sc.EventIdle, onIdle)
	_ = city.Run()
}

func onIdle(e sc.Event) {
	r := city.Robot(e.Robot)
	x, y := r.Position()

	// Pick the next explore target: a short hop along a rotating heading. Flying reveals
	// the map (~5 cells around the robot), so this is how you uncover resource spots.
	n := 0
	if v, ok := r.Memory()["hop"].(float64); ok {
		n = int(v)
	}
	n++
	d := dirs[n%len(dirs)]
	destX := x + float64(d[0]*exploreHop)
	destY := y + float64(d[1]*exploreHop)

	// Stay alive — budget the WHOLE ROUND TRIP, not just the way home. A robot that flies
	// out to (destX, destY) and can't get back to a charging pad dies mid-flight, so before
	// we commit to the hop we require enough battery for here→dest AND dest→pad plus a
	// margin. If it can't afford the round trip, divert to the pad and charge now. (The Base
	// at the origin doubles as a pad; you can also charge on Flying Stations / Charging Towers.)
	roundTrip := math.Hypot(destX-x, destY-y) + math.Hypot(destX, destY) + chargeMargin
	if r.Energy() < roundTrip {
		if cx, cy := r.Cell(); cx == 0 && cy == 0 {
			r.Charge()
		} else {
			r.MoveTo(0, 0)
		}
		return
	}

	// Enough battery for the round trip → commit to the explore hop.
	r.SetMemory(map[string]any{"hop": n})
	r.MoveTo(destX, destY)
}
