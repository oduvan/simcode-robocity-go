// Robot City Builder — starter controller (Go).
// One program controls the whole city; address each robot by id. The Go SDK
// mirrors the Python wire protocol. Runtime API lands in a later phase.
//
// The game is event-driven: build around `idle` (fired when a robot is free) and
// issue its next command. Discovery is by MOVING — a robot reveals the area
// around it as it moves; there is no scan command.
package main

import sc "github.com/lyabah/simcode-sdk-go"

func main() {
	city := sc.New() // connects to GAME via the SDK runtime (later phase)

	// A robot is free — decide its next command from its live state. Here: if it
	// isn't on a resource spot, move into the fog to discover one.
	city.On(sc.EventIdle, func(e sc.Event) {
		r := city.Robot(e.Robot)
		if here := r.Here(); here.Spot != nil && here.Building == nil {
			r.StartConstruction(sc.BuildingMining)
		} else {
			x, y := r.Position()
			r.MoveTo(x+5, y) // explore by moving (reveals new map)
		}
	})

	city.Run()
}
