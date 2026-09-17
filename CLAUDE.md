# SimCode city — robot controller (Go)

This repo is the **brain of one SimCode city**. `main.go` is a single Go program that
controls *all* the robots in your city in the **Robot City Builder** game. You don't click
to place buildings — **code is the only way to influence the world**. Push to this repo's
default branch and the platform hot-reloads your code into the running city; the robots
immediately act on the new program and you watch the city evolve at your city's live page.

> This is a **user code repo**, not the platform. You only write the controller; the
> `simcode` client library, the world, the rules, and the robots come from the platform.

## ⚡ Test locally BEFORE you push

Pushing to see the result is slow. You can run your `main.go` against the **real game
engine** on your machine — the *exact* engine the server runs (downloaded on demand,
**not** a re-implementation) — so checking "does this actually work if I push it *now*?"
takes seconds.

**Step 1 — in a new environment, run the one-command setup. This is the FIRST thing to
run; nothing else is needed before it.**

```bash
./setup.sh                               # installs the test tooling + warms the engine cache
```

`./setup.sh` installs the local test tooling (the `robocity-sim` CLI, which carries the Go
client library) if it isn't installed yet, then runs a 1-tick local test that downloads and
caches the game engine — so a real run right afterwards is instant. It is **idempotent and
fast to re-run**: start every session with it without wondering whether it already ran;
when everything is in place it skips the install and returns in a moment. If setup can't
complete, it stops and names what was missing (no Go on `PATH`, no C compiler, no network).
It is **local testing only** — it never deploys anything to your city (deploying is commit
+ push + resync, below).

> `robocity-sim run main.go` compiles your controller, so the **Go toolchain must be on
> `PATH` every time** — setup.sh checks that first and says so plainly if it's missing. And
> if `go install` puts the tool in a directory that isn't on your `PATH` (typically
> `$(go env GOPATH)/bin`), setup.sh prints the one `export PATH=…` line to paste into the
> shell you're in right now, and adds that same line **once** to the shell startup files you
> already have (`~/.profile`, `~/.bashrc`, `~/.zshrc`) so a *new* shell needs no step at all
> — it names every file it changed, and those files are the only thing it writes outside
> this repo.

**Step 2 — after every edit, run the local check:**

```bash
robocity-sim run main.go                 # run your controller vs the REAL engine, on YOUR city's world
robocity-sim run main.go --ticks 300     # simulate more ticks
robocity-sim run main.go --json          # machine-readable summary
robocity-sim run main.go --from-live     # start from your city AS IT IS NOW (what a push meets)
robocity-sim run main.go --canonical     # the canonical map (use this before your city exists)
robocity-sim check main.go               # would a deploy ACCEPT this code? (no simulation)
```

**Testing what a real push actually meets.** A push never starts a new world — it loads new code into a running city, where in-memory values are reset, saved values survive, and robots are part-way through journeys carrying cargo. `--from-live` reproduces exactly that: it resumes your city's saved world and its saved store, continuing the city's own tick numbering. Code that behaves perfectly from an empty world can fail immediately there. A fresh world stays the default (it is reproducible, and it works before your city exists).

**Which world it runs, and why it sometimes refuses.** With no flags it uses **your
city's** world — its seed and its per-city config — read from the public snapshot. If that
cannot be obtained (offline, this repo not linked to a city yet), the run **stops** with
exit code `6` instead of quietly running a different map, because a result from someone
else's world tells you nothing about yours. Ask for another world by name: `--city
<slug>`, `--seed <N>`, or `--canonical`. Every run prints which world it used, in the
banner and again in the summary.

**It accepts exactly what a deploy accepts.** Before simulating, `run` asks the server
whether a real push would accept this repo — the same rule the server runs on push. Exit
`4` = a deploy would reject it (fix it before pushing: a rejected release never loads and
your city silently keeps running the previous code), `5` = the rule could not be consulted.

**Expired is not destroyed.** The summary reports these separately, and only one is a
problem: `robots expired` = flew past its lifespan (normal end of life — build
replacements), `robots destroyed` = battery hit 0 mid-flight (avoidable, and a bug in your
code). A long run turning over many robots is a healthy fleet.

**Check your LIVE city after a push** — same tool, no token, no MCP (reads the server's
public REST API; the city is auto-detected from this repo's git remote):

```bash
robocity-sim inspect            # compact status of your live city
robocity-sim inspect --state    # full current world state
robocity-sim inspect --logs 100 # recent activity + your r.Log() output
robocity-sim inspect --errors   # unhandled exceptions (panics) since your last push
```

**If your city looks "frozen" (nothing moving), run `inspect --errors` FIRST.** A panic in
a handler leaves that robot uncommanded, so a bug in your code looks like a stuck city — this
lists every unhandled exception since your last release, grouped by `type` + `file:line`,
with a sample traceback and the log lines leading up to it. `handler errors` in a local
`run` catches most of these before you push; `inspect --errors` catches what only happens live.

**Background (what `./setup.sh` does for you — you don't have to run these by hand).** The
tooling is one package,
`go install github.com/oduvan/simcode-robocity-go-tools/cmd/robocity-sim@latest`. It loads
the engine over a small cgo bridge, so it needs a **C compiler** (`CGO_ENABLED=1` +
gcc/clang — the default on macOS and most Linux) at install time. The **first run downloads
the engine** from the server (`GET /api/engine/lib`) and **caches** it under
`~/.cache/simcode/`, so later runs are instant — no build step, no token. Your
`main.go` runs **unchanged**. Read the SUMMARY: `handler errors` must be **0** and `robots
destroyed` should be **0** (`robots expired` may be any number — that is normal end of
life), and `buildings` / `discovered cells` should grow if the controller is doing
something. The exit code is non-zero if any handler panicked, so you
can gate a push on it.

> **Check your code with `robocity-sim run main.go` — NOT `go run main.go`.** Running it
> directly just starts the client runtime with no engine to talk to. `robocity-sim` drives
> your handlers against the real engine tick by tick, so you verify **behaviour**.

> **Platform note:** the engine library is a glibc-linked Linux/macOS build, so run local
> tests on a normal glibc host (**not** Alpine/musl). To use a locally-built engine instead
> of the download, point `SIMCODE_ENGINE_SO` at a `libengine.so`; `SIMCODE_SERVER` picks a
> different server. `go build ./...` still confirms your controller **compiles** (heads-up:
> a plain build fetches the published client library over the network, which fails in offline/CI
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

### ⚖️ Balance lives in the config — read it, don't hardcode

This doc describes **mechanics, roles, and the API** — deliberately **without balance numbers**
(cargo sizes, speeds, lifespans, costs, recipe amounts, store caps, quest quantities, wear/repair
rates, energy, start capital). Those are **not** fixed: the same module is **tuned per city** (#35)
and **rebalanced over time**, so any number written in a doc goes stale. **The config is the source
of truth; this doc is not.** Always derive balance from the live game:

- **At runtime, read what the game exposes** rather than using constants:
  - `b.Recipe()` — a built processor's inputs / output / out-amount / ticks.
  - `city.Base().Unlocks()` — the building + robot types buildable at the current level.
  - `city.Base().Level()` and `city.Base().Quest()` (`required` / `progress`).
  - a robot's `r.Type()`, `r.LifeRemaining()`, `r.LifeMax()`.
  - a store's capacity: `b.Storage().Capacity`, `r.Inventory().Capacity`, etc.

> ### ⚠️ "Do I already have one?" must count SITES, not just finished buildings
> A construction site **is** a building — it is already in `city.Buildings()` / `OfType(t)`,
> with `Status() == "constructing"`. So filtering to `"active"` for a "do I have one of these
> yet?" check answers **no** while yours is still being built, and you order a second, then a
> third. Count the site too, or you will fund three of everything and stall them all by
> spreading your materials.


  Prefer these live handles over any hardcoded number.
- **The authoritative full balance for the city** is its **world config**, surfaced by the
  language-agnostic MCP tool **`get_world_config`**. It returns `robot_types` (cargo / speed /
  lifespan / cost / unlock-level per class), per-building `cost` / `build_ticks`, `tunables` (carry
  capacity, speeds, all store caps, mining, energy, start capital), the `unlocks` ladder, the
  `maintenance` dials (wear / repair rates), and the `quest` formula. When you (or an assistant)
  need an **exact** number, read it from there. Numbers can differ **per city** and **change over
  time** — so never copy a magnitude out of this doc; read it from the config.

Goal of the reference module: **raise the Base's level**. The Base sets a **quest**; deliver it
and the Base **levels up** — and **each level unlocks the next tier of buildings and robot
types** (product-based leveling, see below). Your **highest Base level is your score.** This is a
**living economy**: your fleet **wears out and must be replaced**, and your higher-tier factories
**decay and need servicing** — you can't set it and forget it.

> **This starter does NOT play the game — it does nothing at all.** Robots stay PARKED at the
> Base until your code moves them. That is deliberate: a parked robot spends no energy and no
> lifespan, so the city stays stable however long you leave it, and it can never age its fleet
> out into a dead end. Building the winning loop below is **your** job — that's the point of a
> starter. Grow `main.go` from the empty handler it ships with.

The loop you'll build toward:

```
pick up a kit from the starting Storage → fly to a resource spot →
  place a Mining site (World().Build) + Drop the kit to build it → the mine digs itself →
  haul raws to processors → processors refine them into higher tiers (autonomously) →
  haul the PRODUCTS the quest asks for to the Base → Base LEVELS UP + UNLOCKS the next tier → repeat
  (and: build Flying Stations to make more robots — they cost raw ore + metal per type;
   robots EXPIRE by distance flown, so keep building replacements;
   T2/T3 processors WEAR OUT — send a Mechanic carrying metal to Repair() them;
   recharge on any pad to keep flying)
```

**The four new "living economy" loops (#42) — the heart of the mid/late game:**
1. **Robot types.** Robots come in **classes** built via `station.BuildRobot(type, n)`, each
   with different cargo / speed / lifespan / cost, **unlocked by Base level** (below).
2. **Robots expire.** Every robot has a **max cumulative flight distance** (its lifespan). Fly
   past it and the robot is **removed** (`sc.EventRobotExpired`) — separate from energy-death,
   and **unavoidable**. Plan replacements so the fleet doesn't age out from under you.
3. **Buildings wear.** T2/T3 processors lose **condition** with use; past the halfway mark they
   slow, at empty they **stop**. A **Mechanic** robot carrying metal flies to the building and
   runs `Repair()`.
4. **Product-based leveling + unlocks.** The ladder is **ENDLESS and GENERATED FROM YOUR WORLD'S
   SEED** — there is no fixed list to memorise, and another city's ladder differs from yours. So:
   **read `city.Base().Quest()`** for what this level wants and **`city.Base().Unlocks()`** for what
   is buildable; never hardcode either. Level 1 is always a raws-only bootstrap. Every level is
   completable with what you already have, and every level is harder than the one below.
   `city.Base().NextQuest()` previews the level above once you pass ~75% of the current one — nil
   before that means "not revealed yet", never "no more levels".

**Robot types** — chosen at build time via `station.BuildRobot(type, n)`, unlocked by Base level.
Robots cost **raw ore + metal** (per type), spent from a Flying Station's own store. Each class
differs in **cargo / speed / lifespan / cost** — read the actual figures from `get_world_config`'s
`robot_types` (or a live robot's handles), not from here:

| Type (`sc.` constant) | Unlock | Role |
| --- | --- | --- |
| **builder** `sc.RobotBuilder` | L1 | generalist — the starting fleet; places & supplies sites |
| **hauler** `sc.RobotHauler` | L2 | logistics — big loads, slow |
| **scout** `sc.RobotScout` | L2 | exploration — fast, far, low cargo, cheap |
| **mechanic** `sc.RobotMechanic` | L2 | building maintenance (`Repair`); carries metal |
| **heavy_hauler** `sc.RobotHeavyHauler` | L4 | advanced logistics — largest loads |
| **ranger** `sc.RobotRanger` | L4 | advanced explorer — fast and long-lived |

The starting fleet is **builders**. Higher types live **longer** and cost **more** — but nothing
lives forever, so replacement is a permanent part of the loop.

- **Robots start EMPTY.** There's no free kit — a robot carries nothing until it picks
  something up. Your capital is a **Storage building pre-placed next to the Base**, stocked
  with a starting supply of **ore + metal** (the amount is set by the config); robots `PickUp`
  from it to get building materials.
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
  input, `PickUp` → output. Input/output stores have a **fixed cap** each (in the config) — they
  accumulate real stock between hauls.) The tree (only the **item flow** is shown — the recipe
  **amounts**, batch **ticks**, and **build cost** are balance, so read them from `b.Recipe()` /
  `get_world_config`, not here):

  | Tier | Building (`sc.` constant) | Refines (item flow) | Wears? |
  | --- | --- | --- | --- |
  | T1 | Smelter `sc.BuildingSmelter` | `ore → plate` | no |
  | T1 | Wire Mill `sc.BuildingWireMill` | `metal → wire` | no |
  | T1 | Glassworks `sc.BuildingGlassworks` | `crystal → glass` | no |
  | T1 | Kiln `sc.BuildingKiln` | `carbon → coke` | no |
  | T2 | Assembler `sc.BuildingAssembler` | `plate + wire → part` | **yes** |
  | T2 | Electronics Lab `sc.BuildingElectronicsLab` | `wire + glass → circuit` | **yes** |
  | T2 | Alloy Furnace `sc.BuildingAlloyFurnace` | `plate + coke → alloy` | **yes** |
  | T3 | Module Assembler `sc.BuildingModuleAssembler` | `part + circuit → module` | **yes** |
  | T3 | Frame Shop `sc.BuildingFrameShop` | `alloy + plate → frame` | **yes** |

  A building's cost is always paid in **lower tiers** than it produces, so the tree bootstraps from
  raws with no deadlock. T2/T3 processors have a **2×2** footprint. **Every build cost exceeds a
  robot's carry capacity**, so raising any structure is a **≥2-trip haul** — no site is funded by a
  single `PickUp`; sites **accumulate deliveries across trips**. Each batch makes its output over
  its ticks — scale comes from **volume + many processors**, not a big per-batch yield, so at quest
  scale a single processor can't solo a level (and, for T2/T3, it **wears out** — see below).

  **T2/T3 processors wear (`#42`).** Every completed batch costs a wearing building **condition**
  (from full toward empty; T3 wears faster than T2). Above the halfway mark it runs full speed;
  **below it the process time stretches** (productivity scales with condition); at **empty it stops
  entirely** (`sc.EventBuildingStopped`). A **mechanic** robot (a class unlocked at L2) carrying
  **metal** flies onto the worn building and runs `r.Repair()`, which drains its held metal over
  time into condition until the metal runs out or the building is full (`sc.EventRepairComplete`).
  Read a building's condition with `b.Condition()`, and watch `sc.EventMaintenanceNeeded`
  (condition dropped below the maintenance threshold). **Mining and T1 processors never wear** —
  only T2/T3, and the **mechanic is guaranteed to unlock no later than the first building
  that can wear**, so nothing can decay before you can repair it — but the ladder is generated per
  world, so read `city.Base().Unlocks()` for *which level* that is rather than assuming a number. (Wear-per-batch and repair rates are config `maintenance` dials — read them, don't assume.)

- **Base infrastructure buildings** (all build costs are in the config — read them there):
  - **Base** (pre-placed, one) — the **quest hub** and a **charging pad**. `Drop` the quest's
    goods on it to progress; meet the quest and it **levels up**. You **cannot** `PickUp` from
    the Base (its store is the quest accumulator only). The Base **cannot be destroyed**.
  - **Storage** (2×2 hub, costs **ore + metal**) — a big buffer robots `PickUp` from and `Drop`
    into. The starting one holds your capital; build more with `World().Build(sc.BuildingStorage, …)`.
  - **Mining** (**ore-only** cost) — placed on a live resource spot; auto-mines into a small capped
    store that robots `PickUp` from. (Mining is ore-only so a metal spot is always rebuildable — a
    metal-costed mine could deadlock once metal ran dry.)
  - **Flying Station** (costs **ore + metal**) — a **charging pad** *and* the **robot factory**:
    stock it with **raw ore + metal**, then `station.BuildRobot(type, n)`.

- **Upgrade buildings** (higher-tier sinks — built structures, *not* processors; unlocked at **L4**):
  - **Deep Mine `sc.BuildingDeepMine`** (built from parts + plate) — like Mining but mines **faster
    into a larger buffer**. Place on a spot.
  - **Warehouse `sc.BuildingWarehouse`** (built from alloy + plate, 2×2) — a general store like
    Storage but **much larger**.
  - **Charging Tower `sc.BuildingChargingTower`** (built from circuit + wire) — a remote
    **charging pad** (no haulable store); land and `r.Charge()`.

- **Everything except the Base is built autonomously:** place a site with
  `city.World().Build(type, x, y)`, robots **`Drop`** the build cost to fulfil it, and the site
  **self-completes** once supplied — no connect step, no robot labor. Building a **not-yet-unlocked**
  type is rejected with a `level_required` reason (raise the Base's level first).
- **Growing the fleet costs raw ore + metal** (per robot type — see the type table). A Flying
  Station spends **that type's cost** from its own store per robot it builds, so stock a station
  by `Drop`-ing **ore + metal** into it (not products).
- **The Base quest is product-based, seed-generated and endless.** Read `Quest()` — the items
  differ per world. Level 1 is raws; past that it wants processed goods, with the amount
  climbing per level. Each level also **unlocks** the next tier of buildings + robot types — the
  objective pulls you up the whole tree. (The exact quantities and scaling live in the config — read
  `city.Base().Quest()` / `get_world_config`.)
- **Same map for everyone.** The module fixes the world seed, so *every* city of this type
  starts from the **identical canonical map** — the only variable is your code.

## client library reference

```go
import sc "github.com/oduvan/simcode-go"

func main() {
    city := sc.New()                          // connects via the client runtime
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
| `sc.EventRobotDestroyed` | `position`, `reason` | a robot ran out of energy **mid-flight** — gone, cargo lost. **Avoidable** (charge in time). |
| `sc.EventRobotExpired` | `position`, `reason` | a robot flew past its **lifespan** (max cumulative flight distance) — removed from the map, cargo lost. **Separate from `RobotDestroyed` and unavoidable** — end-of-life; build a replacement. |
| `sc.EventChargeComplete` | — | a robot on a charging pad finished charging (battery full). |
| `sc.EventMaintenanceNeeded` | `building_id` | a T2/T3 processor's **condition dropped below the maintenance threshold** (around half) — it's slowing; send a mechanic to `Repair()` it (no `robot_id`). |
| `sc.EventBuildingStopped` | `building_id` | a T2/T3 processor's **condition hit empty** — it stopped producing entirely until repaired (no `robot_id`). |
| `sc.EventRepairComplete` | `building_id` | a mechanic's `Repair()` ended — either it ran out of held metal or the building reached full condition (no `robot_id`). |
| `sc.EventQuestUpdated` | `level`, `requirements` | the Base's current quest — at start and after each level-up (`building_id`). Requirement is **product-based** past L1. |
| `sc.EventBaseLevelUp` | `level`, `quest`, `unlocks` | the Base cleared its quest and **leveled up** — carries the next (product) quest and **only what THIS level ADDED** (`building_id`). ⚠️ It is a **delta, not the full set**: cache it as your buildable list and you silently lose every earlier level, then quietly stop building mines. Read `city.Base().Unlocks()` — that one is cumulative. |
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
| `city.World().Build(type, x, y int)` | Place a self-building construction **site** at `(x, y)` for any buildable `type` — infrastructure (`sc.BuildingMining`, `sc.BuildingStorage`, `sc.BuildingFlyingStation`), a **processor** (`sc.BuildingSmelter`, `sc.BuildingWireMill`, `sc.BuildingGlassworks`, `sc.BuildingKiln`, `sc.BuildingAssembler`, `sc.BuildingElectronicsLab`, `sc.BuildingAlloyFurnace`, `sc.BuildingModuleAssembler`, `sc.BuildingFrameShop`), or an **upgrade** (`sc.BuildingDeepMine`, `sc.BuildingWarehouse`, `sc.BuildingChargingTower`). `mining`/`deep_mine` must be on a live resource spot; the Base isn't buildable. A **not-yet-unlocked** type is rejected `level_required`. **Not** bound to a robot. | `construction_started` / `blocked` (`level_required`) |
| `city.World().Destroy(x, y int)` | Decommission the building at `(x, y)` (also `b.Destroy()` on a handle). It enters `decommissioning` with a **recoverable** store = its build cost (fully refunded) **＋ its current contents**; robots `PickUp` from it and haul it off, and once empty it's removed (`building_destroyed`). The Base can't be destroyed. **Not** bound to a robot. | `decommission_started` / `building_destroyed` |
| `r.PickUp(item, amount)` | Grab `amount` of `item` from the building on the robot's cell **into its inventory** (a Mining/Storage/Warehouse store, a **processor's output**, or a **recoverable** store — e.g. `r.PickUp("plate", 6)`). Use `r.PickUpItem("ore")` for all of one item, `r.PickUpAll()` for everything that fits. Instant. | resolves, then `idle` |
| `r.Drop(item, amount)` | Release `amount` of `item` into the building/site on the robot's cell — supply a build site, feed a Storage/Warehouse, feed a **processor's input**, or deliver to the Base. Use `r.DropItem("metal")` for all of one item, `r.DropAll()` for everything held. Instant. | `resource_delivered` |
| `r.Charge()` | Charge on the **charging pad on the robot's cell** — a **Flying Station**, the **Base**, or a **Charging Tower**; holds the robot until the battery is full. | `charge_complete` / `blocked` (`no_station`) |
| `r.Repair()` | **Mechanic only.** On a worn **T2/T3 processor** on the robot's cell, run a repair process that **drains the mechanic's held metal** over time into the building's condition (the metal→condition rate is a config `maintenance` dial), until the metal runs out or condition is full. | `repair_complete` / `blocked` |
| `r.Send(targetID, payload)` | Send a message to another robot. | the peer gets an `EventMessage` |
| `r.Cancel()` | Abort the current command; the robot goes free. | `idle` |
| `r.Log("…")` | Write a line to the city log (debug your code; surfaces in the MCP tools / logs). | — |

**Position-based:** `PickUp`, `Drop`, `Charge`, and `Repair` act on whatever building/site is on
the robot's **current (rounded) cell** (`r.Cell()`). So to haul, `MoveTo` the mine, `PickUp`, then
`MoveTo` the Base and `Drop`; to recharge, `MoveTo` a Flying Station then `Charge()`; to repair,
load a mechanic with metal, `MoveTo` the worn processor, then `Repair()`. Mining, construction,
and processing are **autonomous**, so there are no robot-driven mining, build-wiring,
site-placing, or single-step-move commands — robots only fly, haul, charge, and (mechanics) repair.

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
  (progress = min(delivered, required); the items depend on the level). ⚠️ Note the shape mismatch:
  `Quest()` is a **raw nested `map[string]any`**, whereas `r.Inventory()` / `building.Storage()`
  are **`Store`** handles — so read the quest maps with map indexing + `float64`/`int` assertions on
  the qty values (`req := q["required"].(map[string]any); need, _ := req["ore"].(float64)`), **not**
  with `Store` methods like `.Get(...)`. Don't write one accessor that assumes both are `Store`s.
  The requirement is **product-based** past the first level and generated per world (read it, don't assume;
  module+frame). Deliver the required goods and the Base **levels up** to the next, harder quest.
  React via `EventQuestUpdated` / `EventBaseLevelUp`.
- **Read what's unlocked:** `city.Base().Unlocks()` returns the buildings + robot types buildable
  at the current level (each level-up widens it — `EventBaseLevelUp` carries only the ADDITION).
  Building or building-a-robot of anything not in it is rejected with a `level_required` reason.

### Grow the fleet — Flying Stations — `city.Stations()`
Robots are built at a **Flying Station** (not the Base). Build one with
`city.World().Build(sc.BuildingFlyingStation, x, y)`, stock it, then command it:

| Call | What it does |
| --- | --- |
| `station.BuildRobot(type, n)` | Queue `n` robots of **`type`** (`sc.RobotBuilder`/`RobotHauler`/`RobotScout`/`RobotMechanic`/`RobotHeavyHauler`/`RobotRanger`) at **this** station. Each consumes **that type's raw ore + metal cost** (per-type amount in the config) from the station's own store and takes time; each finished one spawns **empty** at the station and fires `EventRobotProduced` + its first `EventIdle`. Waits if the store is short. A **not-yet-unlocked** type is rejected `level_required`. |
| `station.CancelProduction()` | Clear this station's production queue. |

Get a station handle from `city.Stations()`; each exposes `.Storage()` (its production store —
`Drop` **ore + metal** here to fund robots) and `.Production()` (`active`/`progress`/`queued`).
You **cannot `PickUp` from a station** (its store is production-only). Each robot type has its own
ore/metal cost and its own **lifespan** — pick the class for the job (a cheap long-lived **scout**
to explore, a **hauler** for bulk, a **mechanic** to keep factories alive).

### Read the world (read fresh each event)
You never hold a live object — these read the current state when your handler runs.

- **Robots:** `city.Robot(id)` → `r.ID`, `r.Type()` (the class: `builder`/`hauler`/`scout`/
  `mechanic`/`heavy_hauler`/`ranger`), `r.Position()` → **float** `(x, y float64)`,
  `r.Cell()` → the **rounded** `(x, y int)` used for position-based actions, `r.Facing()`,
  `r.State()` (`idle`/`moving`/`charging`/`hauling`/`blocked`), `r.Command()`, `r.Energy()`
  (battery, `float64`, 0…cap), `r.LifeRemaining()` / `r.LifeMax()` (`float64` — cumulative
  flight distance left before **expiry**, and this type's total lifespan; retire/replace a robot
  as `LifeRemaining()` nears 0), `r.Inventory()` (a **`Store`** item map: `.Get("ore")`,
  `.Has("ore")`, `.Items`, `.Total()`, `.Free()`, `.Capacity`, `.IsFull()`),
  `r.Here()` (`.X`, `.Y`, `.Terrain`, `.Spot`, `.Building` — what's on its cell),
  and per-robot state `r.Memory()` / `r.SetMemory(map[string]any)`.
- **Buildings:** `city.Buildings()` `[]*Building`, `city.Base()`, `city.Stations()`. A
  `*Building` exposes `.Type()` (one of `base`/`mining`/`storage`/`flying_station`/`smelter`/
  `wire_mill`/`glassworks`/`kiln`/`assembler`/`electronics_lab`/`alloy_furnace`/
  `module_assembler`/`frame_shop`/`deep_mine`/`warehouse`/`charging_tower`), `.Position()`,
  `.Footprint()` `(w, h int)`, `.Status()` (`constructing`/`active`/`decommissioning` — compare
  with `sc.StatusActive`/`sc.StatusConstructing`), `.Storage()` (a **`Store`** item map:
  `.Get("ore")`, `.Has("ore")`, `.Items`, `.Total()`, `.Free()`, `.Capacity`), `.Spot()`
  (Mining/Deep Mine — auto-mines into its storage), `.Level()` + `.Quest()` + `.Unlocks()` (Base —
  the current level, its product quest, and what's buildable now), `.Production()` (Flying
  Station), `.Construction()` (while building — sites self-complete, no connect step).
  **Processors** add `.Input()` / `.Output()` (`Store`s — haul inputs into `.Input()`, pull
  products from `.Output()`) and `.Recipe()` (its `inputs`/`output`/`ticks`); **T2/T3
  processors** also expose `.Condition()` (a wear meter, full→empty — full speed above the halfway
  mark, slows below, stops at empty; send a mechanic to `Repair()` when it drops). A
  **decommissioning** building exposes
  `.Recoverable()` (a `Store` to `PickUp` and haul away). `.Quest()` / `.Construction()` /
  `.Recipe()` are raw `map[string]any` bags.
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

## Common gotchas

Real things that trip up controllers — **behaviour, not magnitudes** (read the numbers from the
config, per the balance rule above):

1. **A build cost can exceed a robot's inventory.** Funding a construction site can take
   **several trips** — sites accumulate deliveries across `Drop`s. Don't assume one `PickUp` funds
   a site: read the cost from `b.Recipe()` and your cap from `r.Inventory().Capacity`, and keep
   hauling until the site completes.
2. **`Drop` deposits only what the target still needs or can hold — the excess stays on the
   robot.** True for build sites, processor inputs, and the Base. Over-pick, or drop mixed cargo
   into a store that only wants some of it, and you keep the leftovers — and can loop re-dropping
   into a full store (looks stuck). `sc.EventResourceDelivered` reports the amount actually
   **accepted** — trust that, not what you asked to drop.
3. **`World().Build` / `World().Destroy` failures are WORLD events with no `robot_id`.** They
   arrive as an **`sc.EventBlocked`** carrying `reason`
   (`no_spot` / `cell_occupied` / `level_required` / `unknown_type` / `nothing_here` / …) plus the
   target `type` and `pos` in the payload, so you can tell **which** build failed.
   `World().Build(...)` itself **returns nothing**, so subscribe to `sc.EventBlocked` rather than
   checking a return value. (A real spot builds even **under fog** — the world is generated
   deterministically; a build fails only when there's genuinely no live spot there, the type isn't
   unlocked, the cell's taken, etc.)
4. **Guard energy for the whole ROUND TRIP, not just the way out.** A robot can reach a far target
   and then be too drained to get home, dying mid-flight. Before a long flight require
   `Energy() ≥ dist(here→dest) + dist(dest→nearest pad) + margin`. Pads are the **Base**, active
   **Flying Stations**, and **Charging Towers**. (The starter parks robots instead of flying
   them, so this is the first thing to get right once you DO start moving.)
5. **After the early levels the Base stops accepting raws — it wants PRODUCTS.** Read
   `city.Base().Quest()`: once it asks for products, raws pile up in Storage and haulers can freeze
   holding undroppable cargo unless you've built the processor chain. Cap how much of each item you
   bank, **harvest processor outputs even before a downstream consumer exists** (a full output
   store stalls the processor), and always give an idle robot something to do.
6. **The city store and the world persist across code reloads; package-level globals reset.** A
   design change won't retroactively apply if an old decision is cached via `city.SetStore(...)`
   (or baked into a building / a robot's memory). Detect stale state on load and migrate or rebuild
   it.
7. **Full Storage can dead-lock the fleet — never park a robot holding cargo it can't drop.**
   Every idle handler must issue a command; a robot left parked (charge/wait) while still holding
   undroppable cargo **never re-enters your task selection** and is stuck for good. Don't make
   Storage the only drop target: if it's full, fall through to the **next valid sink** — the
   **Base quest item → a processor input that needs it → any Storage with room**. Storage has a
   fixed cap, so cap how much of each item you bank, harvest processor outputs even without a
   downstream consumer, and add Storage/Warehouse capacity *before* you hit the ceiling. A couple
   of robots frozen on undroppable cargo can stall the **entire** city (only `EventQuestUpdated`
   keeps firing). (Full-Storage flavour of #5 above: same freeze, different trigger.)
8. **You can dig yourself into an unrecoverable raw shortage.** A Mining building costs a raw
   (ore); a spot is **finite** and eventually depletes (`sc.EventSpotDepleted`). If stored ore
   drops below one mine's cost *before* a replacement is up, you can build **neither a mine nor
   robots** (both cost ore) → a permanent, self-reinforcing deadlock. Defend proactively:
   (1) **reserve** ~one mine's worth of each raw nothing else may spend; (2) **replace a mine on
   `Spot().Remaining` getting low, not on your stockpile getting low** — so you still have the raw
   to fund it; (3) keep the fleet's **type mix bounded** (e.g. cap mechanics) so long-lived
   specialists don't crowd out haulers; (4) last-ditch, `World().Destroy` a spent mine to reclaim
   its ore via `.Recoverable()` — but only if a robot then hauls that away.
9. **Local dev tips.** `fmt.Println(...)` shows in `robocity-sim` stdout — and so does `r.Log(...)`.
   Store values must be **JSON-serializable**. `r.ID` is a **string** like `"r1"`, not an int; and
   note `r.Memory()["hop"]` comes back as a `float64` (JSON numbers decode that way), so
   type-assert accordingly.

## Constraints — read before editing

- **Sandbox:** restricted runtime — no file/network/process access, no arbitrary packages
  beyond `simcode` + a safe stdlib subset. Keep helpers in this module.
- **Handlers must be fast** (tight per-invocation CPU/time budget); do a little and return.
- **State** in package-level vars persists while the process runs but **resets on a code
  reload** — use `city.SetStore/GetStore` (city-wide) or `r.SetMemory` (per robot) for state
  that must survive a push.
- **Determinism:** no wall-clock or randomness; the world is seeded and replayable.
- **The client library is provided** by the platform at runtime — don't vendor a different version.
- **You cannot reset the world from code.** Resetting a city (wiping it back to tick 0) is a
  **destructive, owner-only action available ONLY in the web dashboard** (the Reset button) —
  there is no client library/MCP reset. Your code influences the world only through robot/world commands.

## Working in this repo with Claude Code

**Commit straight to the default branch — never a feature branch, never a PR.** The city only
hot-reloads from this repo's **default branch** on push; work parked on another branch or an
unmerged PR **never deploys**. So the loop is: edit `main.go` → run the local check → commit to
the default branch → push → **resync**. Don't create branches, don't open PRs.

**After the push completes, check that it landed.** The push alone usually does it:
GitHub notifies the platform, which pulls, validates and hot-reloads. But that delivery
is **not guaranteed**, and the catch-up that recovers a missed one only runs
**periodically** — so confirm rather than assume. Either way works:
- **With the platform's MCP tools:** call `resync` for your city. It re-pulls and reloads
  **immediately**, so there is nothing left to wait for.
- **Without them** — a session with no platform MCP connected, which is common — run
  `robocity-sim inspect`. No token, no MCP. It reports the **release** your city is
  actually running: once that is your new commit, the push has landed and you are done.
  If it still shows the old one after a minute, push again, or press **Resync** on the
  city page. A missing MCP tool is not a blocked deploy — do not stop on it.
Only changes to the **running code** (`main.go`, `lib/`) need any of this; a docs-only or
`issues/`-only commit does not. (Resync just re-pulls this repo and reloads the code; it
never resets your world.)

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
  is destroyed and its cargo lost. Add **Storage** as a buffer. Mind the tension: a station's
  store pays for robots, the Base's store pays for quests — balance growing the fleet against
  leveling up.
- **Replace the fleet before it ages out (#42).** Every robot **expires** by cumulative flight
  distance (`sc.EventRobotExpired`) — watch `r.LifeRemaining()` and keep a Flying Station
  stocked with **ore + metal** so `station.BuildRobot(type, n)` can churn out replacements. A
  steady-state fleet needs steady replacement; pick the right **class** per role (cheap far-flying
  **scout** to explore, big-cargo **hauler** for bulk, **mechanic** for repairs).
- **Keep the factories alive (#42).** T2/T3 processors **wear** (`b.Condition()` runs full→empty): they
  slow past the halfway mark (`sc.EventMaintenanceNeeded`) and **stop at empty** (`sc.EventBuildingStopped`). Build
  **mechanics**, load them with **metal**, fly them to worn buildings, and `r.Repair()`. At quest
  scale a single processor can't solo a level without upkeep, so budget metal for both robots and
  repairs.
- **Progress unlocks tiers.** You can only build what your Base level has **unlocked**
  (`city.Base().Unlocks()`; a locked type → `level_required`). Level 1 needs **raws**;
  every level after needs **products** (part → module → module+frame), so plan the chain that the
  next quest — and the tier you want to unlock — demands.
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

**If this doc (or the game's docs) describes something that DOESN'T actually work, that IS a bug —
file it.** A documented event / command / read-model field / mechanic that misbehaves, or any
behaviour that doesn't match what's written here, is a **platform bug** — not your mistake, and not
something to silently work around. File a `bug` post so it gets fixed or the doc gets corrected.

- *Example.* This doc says the **city store persists** across code reloads, container restarts, and
  resync — only a world **reset** clears it. So if you `city.SetStore("x", 1)`, push a change (or
  resync), and `GetStore` comes back **empty** with no reset, that contradicts the doc → file it:
  `create_forum_post(kind="bug", city="<slug>", title="store empty after resync", body="SetStore
  x=1 → pushed a no-op change → GetStore(\"x\") returns not-found; expected it to persist per
  CLAUDE.md")`.

**After you file, MONITOR for an answer — a post isn't done when you submit it.** Check back with
`get_forum_post(id)` (or `list_forum_posts`) for a maintainer **reply** or a `resolved` flag, and
relay the answer to your human. (Only the MCP forum tools can post/read — your robot code / the client library
cannot touch the forum.)

### When MCP isn't enough — file from the repo with an `issues/` folder
MCP is best for **quick ideas and small bugs**. For a **complicated bug** (you need to attach a
repro script, logs, screenshots — several files) **or when the MCP forum tools themselves aren't
working**, commit an issue folder to this repo instead:

```
issues/
  store-empty-after-resync/     # one folder per issue (kebab-case name)
    README.md                   # the write-up (required) — same slug + saw + expected + repro
    repro.go  logs.txt  ...     # any other files = evidence
```

On your next push the platform scans `issues/` and creates one forum post per folder. Conventions
(no frontmatter needed): the post **title** is the README's first `# heading` (else the folder
name); the **kind** is `bug` by default, or `idea`/`question` if you name the folder `idea-…` /
`question-…`; every non-README file becomes a linked **evidence** attachment. It's **one-way**
(repo → forum) and **idempotent** — the same folder won't double-post on later pushes. A push that
only touches `issues/` **does not restart your city**, so filing a bug never interrupts the robots.
