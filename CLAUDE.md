# SimCode city — robot controller (Go)

This repo is the **brain of one SimCode city**. `main.go` is a single Go program that
controls *all* the robots in your city in the **Robot City Builder** game. You don't click
to place buildings — **code is the only way to influence the world**. Push to this repo's
default branch and the platform hot-reloads your code into the running city; the robots
immediately act on the new program and you watch the city evolve at your city's live page.

> This is a **user code repo**, not the platform. You only write the controller; the
> `simcode` SDK, the world, the rules, and the robots come from the platform.

## ⚡ Test locally BEFORE you push

Pushing to see the result is slow. You can run your `main.go` against the **real game
engine** on your machine — the *exact* engine the server runs (downloaded on demand,
**not** a re-implementation) — so checking "does this actually work if I push it *now*?"
takes seconds. **Install the tool once, then run the local check on every change.**

```bash
go install github.com/oduvan/simcode-robocity-go-tools/cmd/robocity-sim@latest

robocity-sim run main.go                 # run your controller vs the REAL engine
robocity-sim run main.go --ticks 300     # simulate more ticks
robocity-sim run main.go --json          # machine-readable summary
```

The tool loads the engine over a small cgo bridge, so it needs a **C compiler**
(`CGO_ENABLED=1` + gcc/clang — the default on macOS and most Linux) at install time. The
**first run downloads the engine** from the server (`GET /api/engine/lib`) and **caches**
it under `~/.cache/simcode/`, so later runs are instant — no build step, no token. Your
`main.go` runs **unchanged**. Read the SUMMARY: `handler errors` must be **0**, `robots
destroyed` should be **0**, and `buildings` / `discovered cells` should grow if the
controller is doing something. The exit code is non-zero if any handler panicked, so you
can gate a push on it.

> **Check your code with `robocity-sim run main.go` — NOT `go run main.go`.** Running it
> directly just starts the SDK runtime with no engine to talk to. `robocity-sim` drives
> your handlers against the real engine tick by tick, so you verify **behaviour**.

> **Platform note:** the engine library is a glibc-linked Linux/macOS build, so run local
> tests on a normal glibc host (**not** Alpine/musl). To use a locally-built engine instead
> of the download, point `SIMCODE_ENGINE_SO` at a `libengine.so`; `SIMCODE_SERVER` picks a
> different server. `go build ./...` still confirms your controller **compiles** (heads-up:
> a plain build fetches the published SDK over the network, which fails in offline/CI
> sandboxes with a confusing auth error unrelated to your code).

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

Goal of the reference module: **raise the Base's level**. The Base sets a **quest**; deliver it
and the Base **levels up** to a harder quest — endlessly. The quest **climbs the supply chain**
with each level (raws → basics → parts → advanced goods), so leveling up eventually means
standing up a whole **factory tree**. Your **highest Base level is your score.**

> **This starter does NOT play the game.** It only keeps the robots alive and flies them
> around to explore the map. Building the winning loop below is **your** job — that's the point
> of a starter. Grow `main.go` from the bare explorer it ships with.

The loop you'll build toward:

```
pick up a kit from the starting Storage → fly to a resource spot →
  place a Mining site (World().Build) + Drop the kit to build it → the mine digs itself →
  haul raws to processors → processors refine them into higher tiers (autonomously) →
  haul the goods the quest asks for to the Base → Base LEVELS UP → repeat, harder
  (and: build Flying Stations to make more robots; recharge on any pad to keep flying)
```

- **Robots start EMPTY.** There's no free kit — a robot carries nothing until it picks
  something up. Your capital is a **Storage building pre-placed next to the Base**, stocked
  with **45 ore / 25 metal**; robots `PickUp` from it to get building materials.
- **The world is endless & continuous.** Robots have **float** `(x, y)` positions and **fly**
  in straight lines from any point to any point, ignoring terrain and each other (no
  pathfinding, multiple robots may share a spot). They interact with a building by their
  **rounded cell** (`r.Cell()`). Flying **spends energy** (∝ distance); run the battery to zero
  **mid-flight and the robot is destroyed** — its cargo vanishes. Recharge by landing on a
  **charging pad** (the **Base**, a **Flying Station**, or a **Charging Tower**) and calling
  `r.Charge()`.

### Resources — a 4-raw supply chain (the #5 tree)
Robots **only haul**. Mining and **all refining** are autonomous: you place buildings and feed
them; they do the work.

- **4 raws**, mined from finite **spots** (each spot yields one): `ore`, `metal`, `crystal`,
  `carbon`. A **Mining building auto-mines** its spot's raw into its own capped store; a robot
  only `PickUp`s the output and hauls it.
- **Processors** are autonomous factory buildings: a robot `Drop`s the recipe's **inputs** into
  the processor's **input** store, it converts them over a few ticks, and a robot `PickUp`s the
  **output** from its **output** store. (On a processor, direction picks the store: `Drop` →
  input, `PickUp` → output. Input/output stores hold 20 each.) The tree:

  | Tier | Building (`sc.` constant) | Build cost | Recipe (per batch) | Ticks |
  | --- | --- | --- | --- | --- |
  | T1 | Smelter `sc.BuildingSmelter` | 10 ore + 6 metal | 2 ore → 1 `plate` | 4 |
  | T1 | Wire Mill `sc.BuildingWireMill` | 6 ore + 10 metal | 2 metal → 1 `wire` | 4 |
  | T1 | Glassworks `sc.BuildingGlassworks` | 10 ore + 6 crystal | 2 crystal → 1 `glass` | 4 |
  | T1 | Kiln `sc.BuildingKiln` | 10 ore + 6 carbon | 2 carbon → 1 `coke` | 4 |
  | T2 | Assembler `sc.BuildingAssembler` | 8 plate + 6 wire | 2 plate + 1 wire → 1 `part` | 6 |
  | T2 | Electronics Lab `sc.BuildingElectronicsLab` | 8 wire + 6 glass | 2 wire + 1 glass → 1 `circuit` | 6 |
  | T2 | Alloy Furnace `sc.BuildingAlloyFurnace` | 8 plate + 6 coke | 1 plate + 2 coke → 1 `alloy` | 6 |
  | T3 | Module Assembler `sc.BuildingModuleAssembler` | 7 part + 5 circuit | 2 part + 1 circuit → 1 `module` | 8 |
  | T3 | Frame Shop `sc.BuildingFrameShop` | 7 alloy + 5 part | 1 alloy + 2 plate → 1 `frame` | 8 |

  Build costs always use **lower tiers** than a processor produces, so the tree bootstraps from
  raws with no deadlock. T2/T3 processors have a **2×2** footprint. **Every build cost exceeds a
  robot's carry capacity (10)**, so raising any structure is a **≥2-trip haul** — no site is
  funded by a single `PickUp`; sites **accumulate deliveries across trips**.

- **Base infrastructure buildings:**
  - **Base** (pre-placed, one) — the **quest hub** and a **charging pad**. `Drop` the quest's
    goods on it to progress; meet the quest and it **levels up**. You **cannot** `PickUp` from
    the Base (its store is the quest accumulator only). The Base **cannot be destroyed**.
  - **Storage** (2×2 hub, costs **8 ore + 4 metal**) — a big buffer (cap 500) robots `PickUp` from and
    `Drop` into. The starting one holds your capital; build more with
    `World().Build(sc.BuildingStorage, …)`.
  - **Mining** (costs **9 ore + 5 metal**) — placed on a live resource spot; auto-mines into a
    small capped store that robots `PickUp` from.
  - **Flying Station** (costs **10 ore + 5 metal**) — a **charging pad** *and* the **robot
    factory**: stock it, then `station.BuildRobot(n)`.

- **Upgrade buildings** (higher-tier sinks — built structures, *not* processors):
  - **Deep Mine `sc.BuildingDeepMine`** (built with **6 part + 6 plate**) — like Mining but mines **2×
    faster** into a **2× buffer** (24). Place on a spot.
  - **Warehouse `sc.BuildingWarehouse`** (built with **5 alloy + 8 plate**, 2×2) — a general store like
    Storage but **much larger** (cap 1500).
  - **Charging Tower `sc.BuildingChargingTower`** (built with **5 circuit + 8 wire**) — a remote
    **charging pad** (no haulable store); land and `r.Charge()`.

- **Everything except the Base is built autonomously:** place a site with
  `city.World().Build(type, x, y)`, robots **`Drop`** the build cost to fulfil it, and the site
  **self-completes** once supplied — no connect step, no robot labor.
- **Growing the fleet costs parts.** A Flying Station now spends **2 part + 1 circuit** from its
  own store per robot it builds — so the whole chain feeds robot production.
- **The Base quest climbs tiers.** L1 asks for **40 ore + 20 metal**; L2 **20 plate + 10 wire**;
  L3 **10 part + 5 circuit**; L4+ **4 module + 3 frame**, scaling up past the top tier. The
  objective pulls you up the whole tree.
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
| `sc.EventResourceDelivered` | `building_id`, `item`, `amount` | a `Drop` deposited into a site/store (one per item). |
| `sc.EventConstructionComplete` | `building_id`, `type` | a site finished building (now active). |
| `sc.EventResourceProduced` | `building_id`, `item`, `amount` | a processor finished a batch — its **output** store now holds `amount` of `item` to haul. |
| `sc.EventProductionBlocked` | `building_id`, `reason` | a processor stalled — `reason` is `output_full` (haul its output away) or `input_short` (feed it more inputs). Fires once per transition into blocked. |
| `sc.EventSpotDepleted` | `building_id` | a Mining building's resource spot ran out. |
| `sc.EventStorageFull` | `building_id` | a building's storage is full. |
| `sc.EventDecommissionStarted` | `building_id` | a `Destroy` began — the building holds a **recoverable** store to haul away. |
| `sc.EventBuildingDestroyed` | `building_id` | a decommissioned building's recoverable store was emptied and it was removed. |
| `sc.EventInventoryFull` | — | a robot can't carry more. |
| `sc.EventRobotProduced` | `robot_id` | a **Flying Station** finished a new robot. |
| `sc.EventRobotDestroyed` | `position`, `reason` | a robot ran out of energy **mid-flight** — gone, cargo lost. |
| `sc.EventChargeComplete` | — | a robot on a charging pad finished charging (battery full). |
| `sc.EventQuestUpdated` | `level`, `requirements` | the Base's current quest — at start and after each level-up (`building_id`). |
| `sc.EventBaseLevelUp` | `level`, `quest` | the Base cleared its quest and **leveled up** (`building_id`). |
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
| `city.World().Build(type, x, y int)` | Place a self-building construction **site** at `(x, y)` for any buildable `type` — infrastructure (`sc.BuildingMining`, `sc.BuildingStorage`, `sc.BuildingFlyingStation`), a **processor** (`sc.BuildingSmelter`, `sc.BuildingWireMill`, `sc.BuildingGlassworks`, `sc.BuildingKiln`, `sc.BuildingAssembler`, `sc.BuildingElectronicsLab`, `sc.BuildingAlloyFurnace`, `sc.BuildingModuleAssembler`, `sc.BuildingFrameShop`), or an **upgrade** (`sc.BuildingDeepMine`, `sc.BuildingWarehouse`, `sc.BuildingChargingTower`). `mining`/`deep_mine` must be on a live resource spot; the Base isn't buildable. **Not** bound to a robot. | `construction_started` / `blocked` |
| `city.World().Destroy(x, y int)` | Decommission the building at `(x, y)` (also `b.Destroy()` on a handle). It enters `decommissioning` with a **recoverable** store = its build cost (fully refunded) **＋ its current contents**; robots `PickUp` from it and haul it off, and once empty it's removed (`building_destroyed`). The Base can't be destroyed. **Not** bound to a robot. | `decommission_started` / `building_destroyed` |
| `r.PickUp(item, amount)` | Grab `amount` of `item` from the building on the robot's cell **into its inventory** (a Mining/Storage/Warehouse store, a **processor's output**, or a **recoverable** store — e.g. `r.PickUp("plate", 6)`). Use `r.PickUpItem("ore")` for all of one item, `r.PickUpAll()` for everything that fits. Instant. | resolves, then `idle` |
| `r.Drop(item, amount)` | Release `amount` of `item` into the building/site on the robot's cell — supply a build site, feed a Storage/Warehouse, feed a **processor's input**, or deliver to the Base. Use `r.DropItem("metal")` for all of one item, `r.DropAll()` for everything held. Instant. | `resource_delivered` |
| `r.Charge()` | Charge on the **Flying Station on the robot's cell**; holds the robot until the battery is full. | `charge_complete` / `blocked` (`no_station`) |
| `r.Send(targetID, payload)` | Send a message to another robot. | the peer gets an `EventMessage` |
| `r.Cancel()` | Abort the current command; the robot goes free. | `idle` |
| `r.Log("…")` | Write a line to the city log (debug your code; surfaces in the MCP tools / logs). | — |

**Position-based:** `PickUp`, `Drop`, and `Charge` act on whatever building/site is on the
robot's **current (rounded) cell** (`r.Cell()`). So to haul, `MoveTo` the mine, `PickUp`, then
`MoveTo` the Base and `Drop`; to recharge, `MoveTo` a Flying Station then `Charge()`. Mining and
construction are **autonomous**, so there are no robot-driven mining, build-wiring, site-placing,
or single-step-move commands — robots only fly, haul, and charge.

### The Base — the quest hub — `city.Base()`
There's one Base; reach it via `city.Base()`. It **isn't built or commanded** — you feed it and
read its objective:
- **Feed it:** robots `Drop(item, …)` (or `DropAll()`) the **items the current quest asks for**
  on the Base's cell — early quests want raws (`ore`/`metal`), later ones want `plate`/`wire`,
  then `part`/`circuit`, then `module`/`frame`. Its store is the **quest accumulator**, capped
  per-item at the requirement (excess stays on the robot). You **cannot `PickUp` from the
  Base.** It also doubles as a **charging pad** (`r.Charge()`).
- **Read the objective:** `city.Base().Level()` (current level, starts at 1) and
  `city.Base().Quest()` — a raw `map[string]any` `{"required":{item:qty}, "progress":{item:qty}}`
  (progress = min(delivered, required); the items depend on the level). Deliver the required
  goods and the Base **levels up** to the next, harder quest. React via `EventQuestUpdated` /
  `EventBaseLevelUp`.

### Grow the fleet — Flying Stations — `city.Stations()`
Robots are built at a **Flying Station** (not the Base). Build one with
`city.World().Build(sc.BuildingFlyingStation, x, y)`, stock it, then command it:

| Call | What it does |
| --- | --- |
| `station.BuildRobot(n)` | Queue `n` robots at **this** station. Each consumes `2 part + 1 circuit` from the station's own store and takes time; each finished one spawns **empty** at the station and fires `EventRobotProduced` + its first `EventIdle`. Waits if the store is short. |
| `station.CancelProduction()` | Clear this station's production queue. |

Get a station handle from `city.Stations()`; each exposes `.Storage()` (its production store —
`Drop` **parts + circuits** here to fuel building) and `.Production()` (`active`/`progress`/`queued`).
You **cannot `PickUp` from a station** (its store is production-only).

### Read the world (read fresh each event)
You never hold a live object — these read the current state when your handler runs.

- **Robots:** `city.Robot(id)` → `r.ID`, `r.Type()`, `r.Position()` → **float** `(x, y float64)`,
  `r.Cell()` → the **rounded** `(x, y int)` used for position-based actions, `r.Facing()`,
  `r.State()` (`idle`/`moving`/`charging`/`hauling`/`blocked`), `r.Command()`, `r.Energy()`
  (battery, `float64`, 0…cap), `r.Inventory()` (a **`Store`** item map: `.Get("ore")`, `.Has("ore")`,
  `.Items`, `.Total()`, `.Free()`, `.Capacity`, `.IsFull()`),
  `r.Here()` (`.X`, `.Y`, `.Terrain`, `.Spot`, `.Building` — what's on its cell),
  and per-robot state `r.Memory()` / `r.SetMemory(map[string]any)`.
- **Buildings:** `city.Buildings()` `[]*Building`, `city.Base()`, `city.Stations()`. A
  `*Building` exposes `.Type()` (one of `base`/`mining`/`storage`/`flying_station`/`smelter`/
  `wire_mill`/`glassworks`/`kiln`/`assembler`/`electronics_lab`/`alloy_furnace`/
  `module_assembler`/`frame_shop`/`deep_mine`/`warehouse`/`charging_tower`), `.Position()`,
  `.Footprint()` `(w, h int)`, `.Status()` (`constructing`/`active`/`decommissioning` — compare
  with `sc.StatusActive`/`sc.StatusConstructing`), `.Storage()` (a **`Store`** item map:
  `.Get("ore")`, `.Has("ore")`, `.Items`, `.Total()`, `.Free()`, `.Capacity`), `.Spot()`
  (Mining/Deep Mine — auto-mines into its storage), `.Level()` + `.Quest()` (Base),
  `.Production()` (Flying Station), `.Construction()` (while building — sites self-complete, no
  connect step). **Processors** add `.Input()` / `.Output()` (`Store`s — haul inputs into
  `.Input()`, pull products from `.Output()`) and `.Recipe()` (its `inputs`/`output`/`ticks`).
  A **decommissioning** building exposes `.Recoverable()` (a `Store` to `PickUp` and haul away).
  `.Quest()` / `.Construction()` / `.Recipe()` are raw `map[string]any` bags.
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
a better city. **Iterate with the local check:** run `robocity-sim run main.go` after every
edit (it runs your controller against the real engine — see "Test locally" above), then
confirm on the live city + logs after a push (or via the platform's MCP tools). High-leverage
improvements over the starter:

- **Bootstrap *both* an ore mine and a metal mine** — the level-1 quest needs both, so a fleet
  that only mines ore stalls. When a mine's spot runs dry (`EventSpotDepleted`), build a
  **replacement** so production never stops and the level keeps climbing.
- **Then climb the supply chain.** Past level 1 the quest asks for refined goods
  (plate/wire → part/circuit → module/frame), so you'll need to place **processors**
  (`sc.BuildingSmelter`, `sc.BuildingAssembler`, …) and keep them fed: haul raws into a
  processor's input, haul its output onward. Watch for `EventResourceProduced` (a batch is
  ready to haul) and `EventProductionBlocked` (`output_full` → clear its output;
  `input_short` → feed it). Also mine **crystal** and **carbon** — the glass/coke branches
  need them.
- **Build on the nearest spot, not a fixed direction.** Prefer the closest known spot from
  `city.World().Spots()`, and only fly into the fog to explore when none is known — the world
  is endless, so there's no edge to wedge against, just more map to reveal.
- **Relocate with `Destroy` when a layout stops working** — `city.World().Destroy(x, y)` (or
  `b.Destroy()`) refunds the build cost **plus** contents into a recoverable store you haul off,
  so a mine on a depleted spot or a misplaced factory isn't a dead loss.
- **Manage energy.** Build **Flying Stations** near your mining frontier (extra charging pads)
  and `r.Charge()` robots **before** they run dry — a robot that runs out of energy mid-flight
  is destroyed and its cargo lost. Add **Storage** as a buffer; grow the fleet with
  `station.BuildRobot(...)` once resources allow. Mind the tension: a station's store pays for
  robots, the Base's store pays for quests — balance growing the fleet against leveling up.
- **Drive it purely by `sc.EventIdle`** — every handler must issue the robot's next command
  (fly / haul / `World().Build` / `Charge`), so no robot is ever left idle with no future
  event. Since `idle` re-fires while a robot stays free, a robot is never permanently stuck; if
  a path would leave one with nothing to do, fly it into unexplored ground (flying reveals new map).

## Report back — turn each session into a forum post

The platform has a small **community forum** (browse it at `/forum`; posts are written through the
platform's **MCP** tools). Treat reporting as part of the job: **when a working session wraps up —
or while you monitor the city grow — analyse what happened and file a forum post.** Good reports
are how the game gets better.

**First, interview your human — thoroughly.** Before posting, ask targeted questions and keep
asking follow-ups:
- What were they trying to achieve, and did it happen?
- What surprised or confused them (in the API, the docs, or the sim)?
- What was missing — and **what would they want next** (features, quests, mechanics)? Capture
  *ideas*, not just bugs.

**Then file a forum post per distinct issue or idea** via the MCP forum tools, in this format so a
post is easy to reproduce and close:
- **kind:** `bug`, `idea` (features/quests go here), or `question`.
- **city (required):** at least the **city slug** (plus its type / relevant state if useful).
- **for a `bug`:** what you did → **what you saw** → **what you expected to see** → repro steps →
  how often it happens.
- **for an `idea`/quest:** the proposal + why it matters + how it would play.

Never file a bare "it did not work" — always **slug + saw + expected + repro**. Report both real
bugs *and* future ideas: the forum is where the roadmap comes from.
