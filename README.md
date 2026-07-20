# My SimCode City (Go)

This repo controls a city in **SimCode — Robot City Builder**. `main.go` is one Go program
that drives the whole robot fleet; **push to the default branch and the platform hot-reloads**
it into your live city.

**The goal:** robots start empty. Pick up materials from the starting **Storage**, build
**mines** on resource spots (4 raws: ore, metal, crystal, carbon), and feed a tree of
autonomous **processors** (smelter → assembler → module assembler, …) that refine raws into
higher-tier goods. Haul the **products** the **Base**'s current **quest** asks for to complete it —
each quest cleared **levels the Base up** (your score) and **unlocks the next tier** of buildings +
robot types; leveling is product-based (L1→L2 raws, then part → module → module+frame). Build
**Flying Stations** to recharge robots and manufacture more — robots come in **level-gated types**
(`BuildRobot(type, n)`) and cost **raw ore + metal**. It's a *living economy*: every robot
**expires** after flying a fixed distance (keep building replacements), and **T2/T3 processors wear
down** and need a **mechanic** to `Repair()` them. The starter only keeps robots alive and explores
— building and maintaining the winning loop is your job.

> **Balance lives in the config, not in these docs.** The exact numbers (cargo, speed, lifespan,
> costs, recipe amounts, store caps, quest quantities, wear/repair rates, energy, start capital)
> are **tuned per city and change over time**, so don't copy magnitudes out of the docs. Read them
> live — from handles like `b.Recipe()`, `city.Base().Unlocks()`, `r.LifeRemaining()`,
> `b.Storage().Capacity` — or from the MCP tool **`get_world_config`** for the full picture.
> `CLAUDE.md` describes the mechanics + API; the config is the source of truth for numbers.

- **Edit `main.go`** to change how your robots behave (pick up, place mines, haul to the Base,
  charge, build robots at a Flying Station).
- **Push** → your city updates in real time.
- No manifest, no extra setup — the `simcode` SDK is provided by the platform at runtime.

Open **[`CLAUDE.md`](CLAUDE.md)** for the game rules, the full SDK (events + commands + read
model), and the sandbox constraints — written so [Claude Code](https://claude.com/claude-code)
can help you write better robot code.

```
main.go        # your controller (the only thing that runs)
go.mod         # module + simcode SDK dependency
CLAUDE.md      # the SDK + game reference
```

## Test it locally before you push

You can run your `main.go` against the **real game engine** on your machine — the exact
engine the server runs, downloaded on demand — so you can check "does this actually work
if I push it now?" in seconds:

```bash
go install github.com/oduvan/simcode-robocity-go-tools/cmd/robocity-sim@latest  # needs CGO/gcc
robocity-sim run main.go                                                        # run vs the real engine
```

The first run downloads + caches the engine (no build step, no token); later runs are
instant. Read the SUMMARY — `handler errors` must be **0**. See [`CLAUDE.md`](CLAUDE.md)
for full usage and options (`--ticks`, `--seed`, `--json`).
