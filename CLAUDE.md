# SimCode city — robot controller (Go)

This repo is the **brain of one SimCode city**. `main.go` is a single Go program that
controls *all* the robots in your city in the **Robot City Builder** game. You don't click
to place buildings — **code is the only way to influence the world**. Push to this repo's
default branch and the platform hot-reloads your code into the running city; the robots
immediately act on the new program and you watch the city evolve at your city's live page.

> This is a **user code repo**, not the platform. You only write the controller; the
> `simcode` SDK, the world, the rules, and the robots come from the platform.

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
explore by moving (reveal the map) → build a Mining building on a resource spot → mine ore/metal
  → haul it to the Base → the Base produces more robots → more robots → faster growth
```

- **Resources:** `ore` and `metal`, found at finite **spots**. Mine them into a Mining
  building's local storage, then a robot **picks up** and **hauls** to the Base/Storage.
- **Buildings:** **Base** (pre-placed, one; produces robots from its store — *not* withdrawable),
  **Mining** (placed on a spot; cap'd storage), **Storage** (cheap, big buffer), **Road**
  (cheap; robots move faster on it). All but the Base are built: `StartConstruction` →
  `Drop` resources to fulfill the recipe → `Connect` (more connected robots finish faster).
- **Construction recipe (Mining):** 6 ore + 3 metal. The fleet **starts with 2 robots**, each
  carrying a 6/3 kit (enough to build one mine). Robots the Base produces also arrive with a kit.
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
| **`sc.EventIdle`** | — | **a robot has no command and needs one** — after any command completes, or right after spawn. Fires once per free transition (not every tick). **This is the main hook: handle it, decide, issue the next command.** |
| `sc.EventSpawn` | — | a robot enters the world (or your code reloads). |
| `sc.EventArrived` | `position` | a `MoveTo` reached its target. |
| `sc.EventBlocked` | `reason` | a move/action couldn't complete. |
| `sc.EventConstructionStarted` | `building_id`, `type` | a construction platform was placed. |
| `sc.EventConstructionComplete` | `building_id`, `type` | a building finished (now active). |
| `sc.EventMiningComplete` | `resource`, `amount` | a `Mine` produced into the mine's store. |
| `sc.EventResourceDelivered` | `building_id` | a `Drop` deposited into a building. |
| `sc.EventSpotDepleted` | `building_id` | the resource spot a robot was mining ran out. |
| `sc.EventStorageFull` | `building_id` | a building's storage is full. |
| `sc.EventInventoryFull` | — | a robot can't carry more. |
| `sc.EventRobotProduced` | `robot_id` | the Base finished a new robot. |
| `sc.EventMessage` | (your payload) | another robot sent you a message via `Send`. |

The cleanest controller is built around **`sc.EventIdle`**: it fires exactly when a robot is
free, so you don't poll and you don't have to chain every completion event by hand. The
starter is essentially one `EventIdle` handler that reads the robot's live state and issues its
next move. Subscribe to the others only when you want their payload (e.g.
`e.Payload["position"]`). Discovery happens **by moving** — a robot reveals a radius (~5) around
itself as it moves; to explore, just `MoveTo` a cell in the fog. There is no scan command.
**Don't subscribe to `sc.EventTick` to poll** — drive everything from `EventIdle`.

### Command a robot — `r := city.Robot(id)`
A command tells one robot to do one thing. The robot runs **only one at a time** — issuing a
new command replaces the current one. Timed commands (move/mine/connect) finish over several
ticks and fire a completion event; instant ones resolve right away. **Either way, when the
robot is free again it fires `EventIdle`** — so you rarely need the specific completion events.
All commands return `*Robot`, so they chain.

| Call | What it does | Completes with |
| --- | --- | --- |
| `r.MoveTo(x, y)` | Walk toward cell `(x, y)`, automatically routing **around** other robots. Reveals the map (radius ~5) as it goes — this is how you explore. | `arrived` (or `blocked`), then `idle` |
| `r.Step("N"\|"S"\|"E"\|"W")` | Move exactly one cell in a compass direction. | `arrived` / `blocked` |
| `r.StartConstruction(sc.BuildingMining\|sc.BuildingStorage\|sc.BuildingRoad)` | Place a construction **platform** on the robot's current cell. `mining` requires a resource spot there. Instant. | `construction_started` |
| `r.Connect()` | Join the construction platform on (or next to) the cell and build it. More robots connected → it finishes faster; the robot is busy until done. | `construction_complete` |
| `r.Mine()` | Mine the **Mining building on the robot's cell** once — ore/metal goes into *that building's* store, not the robot's inventory. | `mining_complete` (or `storage_full` / `spot_depleted`) |
| `r.PickUp(ore, metal)` | Move resources from the building on the cell **into the robot's inventory** (up to carry capacity). **No args = take everything that fits.** Instant. | resolves, then `idle` |
| `r.Drop(ore, metal)` | Deposit inventory into the building on **(or adjacent to)** the cell — supply a construction platform, or feed the Base/Storage. **No args = drop all.** Instant. | `resource_delivered` |
| `r.Send(targetID, payload)` | Send a message to another robot. | the peer gets an `EventMessage` |
| `r.Cancel()` | Abort the current command; the robot goes free. | `idle` |
| `r.Log("…")` | Write a line to the city log (debug your code; surfaces in the MCP tools / logs). | — |

**Position-based:** `Mine`, `Connect`, `StartConstruction`, `PickUp`, `Drop` act on whatever is
on the robot's **current cell** (`Drop`/`Connect` also reach an **adjacent** cell). So to mine,
first `MoveTo` the mine; to haul, `PickUp` on the mine then `MoveTo` the Base and `Drop`.

### Command the Base — `city.Base()`
The Base isn't built or moved; command it directly to grow the fleet.

| Call | What it does |
| --- | --- |
| `city.Base().BuildRobot(n)` | Queue `n` new robots. Each consumes `12 ore + 6 metal` from the Base's store and takes time; each finished one fires `EventRobotProduced` and the new robot's first `EventIdle`. Waits if the store is short. |
| `city.Base().Cancel()` | Clear the pending production queue. |

### Read the world (read fresh each event)
You never hold a live object — these read the current state when your handler runs.

- **Robots:** `city.Robot(id)` → `r.ID`, `r.Type()`, `r.Position()` `(x, y)`, `r.Facing()`,
  `r.State()` (`idle`/`moving`/`mining`/`building`/`hauling`/`blocked`), `r.Command()`,
  `r.Inventory()` (`.Ore`, `.Metal`, `.Capacity`, `.Free()`, `.IsFull()`), `r.Here()`
  (`.X`, `.Y`, `.Terrain`, `.Spot`, `.Building` — what's on its cell), and per-robot state
  `r.Memory()` / `r.SetMemory(map[string]any)`.
- **Buildings:** `city.Buildings()` `[]*Building`, `city.Base()`. A `*Building` exposes
  `.Type()` (`base`/`mining`/`storage`/`road`), `.Position()`, `.Status()`
  (`constructing`/`active`), `.Storage()` (`.Ore`/`.Metal`/`.Capacity`/`.Free()`), `.Spot()`
  (Mining), `.Production()` (Base), `.Construction()` (while building).
- **World:** `city.World()` → `.Tick()`, `.Size()` `(w, h)`, `.Seed()`, `.Discovered()`, and
  `.Spots()` — the resource spots **discovered so far** (each `Cell` has `.X`, `.Y`, and
  `.Spot.Resource` / `.Spot.Remaining`).
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
- **Build on the nearest spot, not a fixed direction.** The starter's explore-one-way logic
  can wedge a robot against the map edge — prefer the closest known spot from
  `city.World().Spots()`, and only explore when none is known.
- Reduce robots blocking each other near the Base; build **Storage** as a buffer and **Roads**
  for speed; call `city.Base().BuildRobot(...)` once resources allow.
- **Drive it purely by `sc.EventIdle`** — every handler must issue the robot's next command, so
  no robot is ever left idle with no future event. Since `idle` re-fires while a robot stays
  free, a robot is never permanently stuck; if a path would leave one with nothing to do, send
  it into unexplored ground (moving reveals new map).
