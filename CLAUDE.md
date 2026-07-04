# SimCode city — robot controller (Go)

This repo is the **brain of one SimCode city**. `main.go` is a single Go program that
controls *all* the robots in your city in the **Robot City Builder** game. You don't click
to place buildings — **code is the only way to influence the world**. Push to this repo's
default branch and the platform hot-reloads your code into the running city; the robots
immediately act on the new program and you watch the city evolve at your city's live page.

> This is a **user code repo**, not the platform. You only write the controller; the
> `simcode` SDK, the world, the rules, and the robots come from the platform.

## ⚡ Test locally BEFORE you push (do this every iteration)

Pushing to see the result is slow. There's a **local simulator** that runs your
`main.go` against an offline copy of the game engine, **seeded from your city's
CURRENT state**, so you can check "does this actually work if I push it *now*?" in
seconds — and only push once it behaves. **Install it and use it on every change.**

```bash
go install github.com/oduvan/simcode-robocity-go-tools/cmd/robocity-sim@latest
export SIMCODE_TOKEN=...   # your MCP token (dashboard → "Connect via MCP")

robocity-sim run .          # tests THIS city's current state (auto-detected)
robocity-sim run . --json   # machine-readable (parse summary + feed)
```

Run it **inside this repo** with your token set — the tool auto-detects which city
this repo is and fetches its live state. Your `main.go` runs **unchanged** — the tool
swaps the SDK for a local engine-backed one via a temporary `go.work` (your `go.mod`
is untouched) and drives `city.Run()` locally. Read the `SUMMARY`: `robots destroyed`
should be **0**, and `ore/metal mined` + `buildings` should grow if the city is
developing. A live run is *approximate* (a quick "does it work now" check, not a
perfect sim). Only push after a local run looks right. See that repo's `CLAUDE.md`
for full usage.

## How it works (the model)

- **One program, whole fleet.** `main.go` controls every robot, addressed by **id**.
- **Event-driven, async.** Register handlers with `city.On(...)`; the game dispatches events;
  you react by issuing **commands** (intents). Data in → intents out. You never hold a live
  game object.
- **State is read fresh** from the world on each event — `city.Robot(...)`, `city.Buildings()`,
  `city.World()` reflect the current tick when your handler runs.
- **Serial per robot.** Events for one robot arrive one at a time; a robot runs one command at
  a time (a new command replaces the active one).
- **No manifest.** The repo is just this program. Language is chosen at city creation; the
  entry is always `main.go`; the world + starting robots come from the game module.

## The game you're playing (Robot City Builder)

Goal of the reference module: **grow the city**. The loop the starter implements:

```
fly into the fog (reveal the map) → place a Mining site on a resource spot (World().Build) →
  the mine digs itself → haul its output to the Base → the Base produces more robots →
  more robots → faster growth (recharge on a Flying Station so robots keep flying)
```

- **The world is endless & continuous.** Robots have **float** `(x, y)` positions and **fly**
  in straight lines from any point to any point, ignoring terrain and each other (no
  pathfinding, multiple robots may share a spot). They interact with a building by their
  **rounded cell** (`r.Cell()`). Flying **spends energy** (∝ distance); run the battery to zero
  **mid-flight and the robot is destroyed** — its cargo vanishes. Recharge by landing on a
  **Flying Station** and calling `r.Charge()`.
- **Resources:** `ore` and `metal`, found at finite **spots**. A **Mining building mines
  autonomously** into its own storage — there is no mine command; a robot only **picks up**
  the output and **hauls** it to the Base/Storage/a build site.
- **Buildings:** **Base** (pre-placed, one; produces robots from its store — *not* withdrawable),
  **Mining** (placed on a live spot; auto-mines into cap'd storage), **Storage** (cheap, big
  buffer), **Flying Station** (robots land and recharge). Every building except the Base is
  **built autonomously**: place a site with `city.World().Build(type, x, y)`, robots **`Drop`**
  resources to fulfil the recipe, and the site **self-completes** once supplied — no connect
  step, no robot labor.
- **Construction recipe (Mining):** 6 ore + 3 metal. The fleet **starts with a couple of
  robots**, each carrying a 6/3 kit (enough to place one mine) and a **full battery**. Robots
  the Base produces also arrive with a kit and full battery.
- **Same map for everyone.** The module fixes the world seed, so *every* city of this type
  starts from the **identical canonical map** — the only variable is your code.

## SDK reference

```go
import sc "github.com/lyabah/simcode-sdk-go"

func main() {
    city := sc.New()                          // connects via the SDK runtime
    city.On(sc.EventIdle, func(e sc.Event) {  // subscribe to an event
        r := city.Robot(e.Robot)              // the robot this event is about
        x, y := r.Position()
        r.MoveTo(x+5, y)                      // move into the fog to reveal more map
    })
    city.Run()                                // dispatch loop (blocks)
}
```

### The event value — `sc.Event`
Every handler gets one `sc.Event`:

- `e.Robot` — the robot id this event concerns (use `city.Robot(e.Robot)`).
- `e.Event` — the event name (e.g. `"idle"`).
- `e.Tick` — the tick it fired on.
- `e.Payload` — a `map[string]any` of the event's extra fields (see the table). Read with
  `e.Payload["position"]`, `e.Payload["reason"]`, etc.

### Subscribe to events — `city.On(event, handler)`

| Event | `e.Payload` keys | Fires when |
| --- | --- | --- |
| **`sc.EventIdle`** | — | **a robot has no command and needs one** — after any command completes, or right after spawn. Re-fires every few ticks while it stays free (not every tick). **This is the main hook: handle it, decide, issue the next command.** |
| `sc.EventSpawn` | — | a robot enters the world (or your code reloads). |
| `sc.EventArrived` | `position` | a `MoveTo` flight reached its target. |
| `sc.EventBlocked` | `reason` | a move/action couldn't complete (e.g. `no_station`). |
| `sc.EventConstructionStarted` | `building_id`, `type` | a `World().Build(...)` placed a site. |
| `sc.EventResourceDelivered` | `building_id`, `ore`, `metal` | a `Drop` deposited into a site/store. |
| `sc.EventConstructionComplete` | `building_id`, `type` | a site finished building (now active). |
| `sc.EventSpotDepleted` | `building_id` | a Mining building's resource spot ran out. |
| `sc.EventStorageFull` | `building_id` | a building's storage is full. |
| `sc.EventInventoryFull` | — | a robot can't carry more. |
| `sc.EventRobotProduced` | `robot_id` | the Base finished a new robot. |
| `sc.EventRobotDestroyed` | `position`, `reason` | a robot ran out of energy **mid-flight** — gone, cargo lost. |
| `sc.EventChargeComplete` | — | a robot on a Flying Station finished charging (battery full). |
| `sc.EventMessage` | (your payload) | another robot sent you a message via `Send`. |

The cleanest controller is built around **`sc.EventIdle`**: it fires exactly when a robot is
free, so you don't poll and you don't have to chain every completion event by hand. The
starter is essentially one `EventIdle` handler that reads the robot's live state and issues its
next move. Subscribe to the others only when you want their payload (e.g.
`e.Payload["position"]`). Discovery happens **by flying** — a robot reveals a radius (~5) around
itself as it moves; to explore, just `MoveTo` a point in the fog. There is no separate reveal
command. **Don't subscribe to `sc.EventTick` to poll** — drive everything from `EventIdle`.

### Command a robot — `r := city.Robot(id)`
A command tells one robot to do one thing. The robot runs **only one at a time** — issuing a
new command replaces the current one. Timed commands (`MoveTo`, `Charge`) finish over several
ticks and fire a completion event; instant ones (`PickUp`, `Drop`) resolve right away.
**Either way, when the robot is free again it fires `EventIdle`** — so you rarely need the
specific completion events. All commands return `*Robot`, so they chain. Placing a building is
a **world** call, `city.World().Build(...)`, not bound to a robot.

| Call | What it does | Completes with |
| --- | --- | --- |
| `r.MoveTo(x, y float64)` | **Fly** in a straight line to float `(x, y)`, ignoring terrain/other robots. Spends energy with distance; reveals the map (radius ~5) as it goes — this is how you explore. | `arrived` / `blocked` / `robot_destroyed` |
| `city.World().Build(sc.BuildingMining\|sc.BuildingStorage\|sc.BuildingFlyingStation, x, y int)` | Place a self-building construction **site** at `(x, y)`. `mining` must be on a live resource spot; the Base isn't buildable. **Not** bound to a robot. | `construction_started` / `blocked` |
| `r.PickUp(ore, metal)` | Grab resources from the building on the robot's cell **into its inventory** (up to carry capacity). **No args = take everything that fits.** Instant. | resolves, then `idle` |
| `r.Drop(ore, metal)` | Release inventory into the building/site on the robot's cell — supply a build site, or feed the Base/Storage. **No args = drop all.** Instant. | `resource_delivered` |
| `r.Charge()` | Charge on the **Flying Station on the robot's cell**; holds the robot until the battery is full. | `charge_complete` / `blocked` (`no_station`) |
| `r.Send(targetID, payload)` | Send a message to another robot. | the peer gets an `EventMessage` |
| `r.Cancel()` | Abort the current command; the robot goes free. | `idle` |
| `r.Log("…")` | Write a line to the city log (debug your code; surfaces in the MCP tools / logs). | — |

**Position-based:** `PickUp`, `Drop`, and `Charge` act on whatever building/site is on the
robot's **current (rounded) cell** (`r.Cell()`). So to haul, `MoveTo` the mine, `PickUp`, then
`MoveTo` the Base and `Drop`; to recharge, `MoveTo` a Flying Station then `Charge()`. Mining and
construction are **autonomous**, so there are no robot-driven mining, build-wiring, site-placing,
or single-step-move commands — robots only fly, haul, and charge.

### Command the Base — `city.Base()`
The Base isn't built or moved; command it directly to grow the fleet.

| Call | What it does |
| --- | --- |
| `city.Base().BuildRobot(n)` | Queue `n` new robots. Each consumes `12 ore + 6 metal` from the Base's store and takes time; each finished one fires `EventRobotProduced` and the new robot's first `EventIdle`. Waits if the store is short. |
| `city.Base().Cancel()` | Clear the pending production queue. |

### Read the world (read fresh each event)
You never hold a live object — these read the current state when your handler runs.

- **Robots:** `city.Robot(id)` → `r.ID`, `r.Type()`, `r.Position()` → **float** `(x, y float64)`,
  `r.Cell()` → the **rounded** `(x, y int)` used for position-based actions, `r.Facing()`,
  `r.State()` (`idle`/`moving`/`charging`/`hauling`/`blocked`), `r.Command()`, `r.Energy()`
  (battery, `float64`, 0…cap), `r.Inventory()` (`.Ore`, `.Metal`, `.Capacity`, `.Free()`,
  `.IsFull()`), `r.Here()` (`.X`, `.Y`, `.Terrain`, `.Spot`, `.Building` — what's on its cell),
  and per-robot state `r.Memory()` / `r.SetMemory(map[string]any)`.
- **Buildings:** `city.Buildings()` `[]*Building`, `city.Base()`. A `*Building` exposes
  `.Type()` (`base`/`mining`/`storage`/`flying_station`), `.Position()`, `.Status()`
  (`constructing`/`active` — compare with `sc.StatusActive`/`sc.StatusConstructing`),
  `.Storage()` (`.Ore`/`.Metal`/`.Capacity`/`.Free()`), `.Spot()` (Mining — auto-mines into
  its storage), `.Production()` (Base), `.Construction()` (while building — sites self-complete,
  no connect step).
- **World:** `city.World()` → `.Tick()`, `.Size()` (bounding box of the **discovered** region,
  not a fixed extent), `.Seed()`, `.Discovered()`, `.Spots()` — the resource spots
  **discovered so far** (each `Cell` has `.X`, `.Y`, and `.Spot.Resource` / `.Spot.Remaining`) —
  and `.Build(type, x, y int)` to place a construction site. The world is **endless**, generated
  lazily as robots fly into the fog.
- **City-wide store:** `city.SetStore(key, value)` / `city.GetStore(key)` `(any, bool)` — your
  own state that survives across events (and code reloads).

> **No `nearest()` helper in Go.** To find the closest ore/metal spot, iterate
> `city.World().Spots()` and pick the nearest one yourself (compare `|dx|+|dy|` to
> `r.Position()`), filtering by `c.Spot.Resource`.

## Constraints — read before editing

- **Sandbox:** restricted runtime — no file/network/process access, no arbitrary packages
  beyond `simcode` + a safe stdlib subset. Keep helpers in this module.
- **Handlers must be fast** (tight per-invocation CPU/time budget); do a little and return.
- **State** in package-level vars persists while the process runs but **resets on a code
  reload** — use `city.SetStore/GetStore` (city-wide) or `r.SetMemory` (per robot) for state
  that must survive a push.
- **Determinism:** no wall-clock or randomness; the world is seeded and replayable.
- **The SDK is provided** by the platform at runtime — don't vendor a different version.

## Working in this repo with Claude Code

The thing to improve is the **strategy** in `main.go`. The world is fixed, so better code =
a better city. You can't run the engine locally; iterate by reading the logic carefully and
checking the live city + logs after a push (or via the platform's MCP tools). High-leverage
improvements over the starter:

- **Keep the Base fed with *both* ore and metal** — it needs both to produce robots; a fleet
  that only mines ore stalls.
- **Build on the nearest spot, not a fixed direction.** Prefer the closest known spot from
  `city.World().Spots()`, and only fly into the fog to explore when none is known — the world
  is endless, so there's no edge to wedge against, just more map to reveal.
- **Manage energy.** Build a **Flying Station** early and `r.Charge()` robots **before** they
  run dry — a robot that runs out of energy mid-flight is destroyed and its cargo lost. Add
  **Storage** as a buffer; call `city.Base().BuildRobot(...)` once resources allow.
- **Drive it purely by `sc.EventIdle`** — every handler must issue the robot's next command
  (fly / haul / `World().Build` / `Charge`), so no robot is ever left idle with no future
  event. Since `idle` re-fires while a robot stays free, a robot is never permanently stuck; if
  a path would leave one with nothing to do, fly it into unexplored ground (flying reveals new map).
