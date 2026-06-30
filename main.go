// Robot City Builder — starter controller (Go).
// One program controls the whole city; address each robot by id. The Go SDK
// mirrors the Python wire protocol. Runtime API lands in Phase 1.
package main

import sc "github.com/lyabah/simcode-sdk-go"

func main() {
	city := sc.New() // connects to GAME via the SDK runtime (Phase 1)

	// A robot entered the world — start scouting.
	city.On(sc.EventSpawn, func(e sc.Event) {
		city.Robot(e.Robot).Scan(6)
	})

	// Found tiles — head to an ore spot if there is one.
	city.On(sc.EventScanResult, func(e sc.Event) {
		r := city.Robot(e.Robot)
		if spot := r.Find(e.Cells, "ore_spot"); spot != nil {
			r.MoveTo(spot.X, spot.Y)
		}
	})

	// Standing on a spot — build a mine, then mine.
	city.On(sc.EventArrived, func(e sc.Event) {
		r := city.Robot(e.Robot)
		if r.Here().Spot != nil && r.Here().Building == nil {
			r.StartConstruction(sc.BuildingMining)
		}
	})

	city.Run()
}
