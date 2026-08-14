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
- No manifest, no extra setup — the `simcode` client library is provided by the platform at runtime.

Open **[`CLAUDE.md`](CLAUDE.md)** for the game rules, the full client library (events + commands + read
model), and the sandbox constraints — written so [Claude Code](https://claude.com/claude-code)
can help you write better robot code.

```
main.go        # your controller (the only thing that runs)
setup.sh       # one-command setup for local testing (run this first in a new environment)
go.mod         # module + simcode client library dependency
issues/        # optional — commit a bug/idea folder here and it posts to the forum
CLAUDE.md      # the client library + game reference
```

> **Hit a bug?** Small stuff → ask your assistant to file it via the MCP forum tools. Something
> **complicated** (needs a repro + logs), or MCP not working? Commit an `issues/<name>/` folder
> (a `README.md` write-up + any evidence files); the next push turns it into a forum post. Always
> **commit to the default branch — no feature branches, no PRs** (the city only deploys from the
> default branch).

## Test it locally before you push

You can run your `main.go` against the **real game engine** on your machine — the exact
engine the server runs, downloaded on demand — so you can check "does this actually work
if I push it now?" in seconds:

```bash
./setup.sh                  # one command: installs the test tooling (needs CGO/gcc) + warms the engine cache
robocity-sim run main.go    # run vs the real engine, on YOUR city's world
robocity-sim check main.go  # would a deploy accept this code?
```

`./setup.sh` is the **only** setup step, and the first thing to run in a fresh environment.
It is idempotent — re-run it any time; when everything is already installed it finishes
immediately, and if something is missing (Go, a C compiler, network) it says so. The engine
is downloaded + cached on first use (no build step, no token), which `setup.sh` does for
you, so later runs are instant. Read the SUMMARY — `handler errors` must be **0**, and so
should `robots destroyed` (`robots expired` is normal end of life, not a fault). A run uses
**your city's** world and stops rather than substituting another one; before your city
exists, pass `--canonical`. See [`CLAUDE.md`](CLAUDE.md) for full usage and options
(`--ticks`, `--seed`, `--canonical`, `--json`).

After you push a change that affects the running code, **resync the city** (the platform's
MCP `resync` tool) so it deploys immediately instead of waiting for a push notification.
