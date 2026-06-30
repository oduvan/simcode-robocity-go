# SimCode city — robot controller (Go)

This repo is the **brain of one SimCode city**. `main.go` is a single Go program that
controls *all* the robots in your city in the **Robot City Builder** game. You don't click
to place buildings — **code is the only way to influence the world**. Push to this repo's
default branch and the platform hot-reloads your code into the running city.

> ⚠️ **Status:** the **Go SDK runtime is in progress**. **Python is the supported language
> today** — if you want a city running right now, create it with the Python template. This
> file documents the *intended* Go API (the wire protocol is identical to Python's), so the
> Go starter compiles against the SDK package and is ready when the runtime ships.

> This is a **user code repo**, not the platform. You only write the controller; the
> `simcode` SDK, the world, the rules, and the robots come from the platform.

## How it works (the model)

- **One program, whole fleet.** `main.go` controls every robot, addressed by **id**.
- **Event-driven, async.** Register handlers; the game dispatches events; you react by issuing
  **commands** (intents). Data in → intents out. You never hold a live game object.
- **State is read fresh** from the world on each event.
- **Serial per robot.** One command at a time per robot; a new command replaces the active one.
- **No manifest.** The repo is just this program. Language is chosen at city creation; the
  entry is always `main.go`; the world + starting robots come from the game module.

## The game (Robot City Builder)

Grow the city: `scout → build a Mining building on a resource spot → mine ore/metal → haul to
the Base → the Base produces more robots → repeat.` Resources are **ore** and **metal** at
finite **spots**. Buildings: **Base** (pre-placed, produces robots), **Mining**, **Storage**,
**Road** — all but the Base are built via `StartConstruction → Drop (fulfill recipe) → Connect`.
A Mining building costs 6 ore + 3 metal; the fleet starts with 2 robots each carrying a 6/3 kit.
**Every city of this type starts from the identical canonical map** — only your code differs.

## SDK reference (intended Go API)

```go
import sc "github.com/lyabah/simcode-sdk-go"

func main() {
    city := sc.New()                              // connects via the SDK runtime
    city.On(sc.EventSpawn, func(e sc.Event) {     // subscribe to an event
        city.Robot(e.Robot).Scan(6)               // issue a command
    })
    city.Run()                                    // dispatch loop
}
```

- **Events** (`sc.Event` carries `e.Robot`): `EventSpawn`, `EventTick`, `EventArrived`,
  `EventBlocked`, `EventScanResult`, `EventConstructionComplete`, `EventMiningComplete`,
  `EventStorageFull`, `EventInventoryFull`, `EventRobotProduced`. (Same set as Python.)
- **Robot commands** — `r := city.Robot(id)`: `r.MoveTo(x, y)`, `r.Scan(radius)`,
  `r.StartConstruction(sc.BuildingMining)`, `r.Connect()`, `r.Mine()`, `r.PickUp(...)`,
  `r.Drop(...)`, `r.Cancel()`, `r.Log("…")`. Position-based: act on the robot's cell (Base
  drop also works from an adjacent cell).
- **Read model:** `r.Position()`, `r.State()`, `r.Inventory()`, `r.Here()` (`.Spot`,
  `.Building`); `city.Buildings()`, `city.Base()` (`.BuildRobot(n)`), `city.World()`.

The Go SDK mirrors the **same wire protocol** as Python (see the platform's
`game/core/contract` and `docs/modules/robot-city/`), so the behavior is identical.

## Constraints

- **Sandbox:** restricted runtime — no file/network/process access, no arbitrary packages
  beyond `simcode` + a safe stdlib subset. Keep helpers in this module.
- **Handlers must be fast** (tight per-invocation CPU/time budget); do a little and return.
- **State** in package vars persists while the process runs but **resets on a code reload** —
  use the SDK's provided store/memory for state that must survive a push.
- **Determinism:** no wall-clock or randomness; the world is seeded and replayable.
- **The SDK is provided** by the platform at runtime — don't vendor a different version.

## Working in this repo with Claude Code

The thing to improve is the **strategy** in `main.go`. The world is fixed, so better code =
a better city. Keep the Base fed with **both** ore and metal (it needs both to produce
robots), avoid robots blocking each other near the Base, and build Storage/Roads as you scale.
Because the Go runtime isn't live yet, validate your logic against this reference and the
Python example until the Go runtime ships.
