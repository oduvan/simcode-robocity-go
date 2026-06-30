# My SimCode City (Go)

This repo controls a city in **SimCode — Robot City Builder**. `main.go` is one Go program
that drives the whole robot fleet; **push to the default branch and the platform hot-reloads**
it into your live city.

> ⚠️ The **Go SDK runtime is still in progress** — **Python is the supported language today**.
> Use the Python template if you want a city running right now. This template documents the
> intended Go API (same wire protocol) and is ready for when the runtime ships.

- **Edit `main.go`** to change how your robots behave (scout, build, mine, haul, grow).
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
