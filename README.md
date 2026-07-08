# My SimCode City (Go)

This repo controls a city in **SimCode — Robot City Builder**. `main.go` is one Go program
that drives the whole robot fleet; **push to the default branch and the platform hot-reloads**
it into your live city.

**The goal:** robots start empty. Pick up materials from the starting **Storage**, build
**mines** on resource spots, and haul their ore/metal to the **Base** to complete its **quest**
— each quest cleared **levels the Base up** (your score). Build a **Flying Station** to recharge
robots and to manufacture more of them. The starter controller does all of this; improve it.

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

Local testing runs your controller against the **real game engine** (the exact engine the
server runs), not a re-implementation. For Go, the SDK's real-engine local mode exists but a
clean one-command runner (`robocity-sim`) is **still pending** — see the honest status and the
current options (compile-check, the identical **Python** local runner, push + observe) in
[`CLAUDE.md`](CLAUDE.md).

> Don't `go install …/simcode-robocity-go-tools` — that older re-implementation is retired.
