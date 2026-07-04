# My SimCode City (Go)

This repo controls a city in **SimCode — Robot City Builder**. `main.go` is one Go program
that drives the whole robot fleet; **push to the default branch and the platform hot-reloads**
it into your live city.

- **Edit `main.go`** to change how your robots behave (fly out, place mines, haul, charge, grow).
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

There's a local tool that runs your `main.go` against your city's **current state**,
so you can check "does this actually work if I push it now?" in seconds:

```bash
go install github.com/oduvan/simcode-robocity-go-tools/cmd/robocity-sim@latest
export SIMCODE_TOKEN=...   # dashboard → "Connect via MCP"
robocity-sim run .        # tests THIS city's current state
```

Run it inside this repo with your token set — it auto-detects your city. See
[`CLAUDE.md`](CLAUDE.md) for full usage.
