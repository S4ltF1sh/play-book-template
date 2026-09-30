# Welcome to the Playbook Template

This is the **demo chapter** — it exists so you can see every content type the template supports in action, and so content-generating agents have a reference. When your real playbook is ready, just replace the `content/` directory and rebuild.

## What's in a playbook?

| Section type | How the next section unlocks |
|--------------|------------------------------|
| **Reading** (like this page) | click "Mark as done" |
| **Inline quiz** | answer *every* question correctly (retries allowed) |
| **Exercise** | run code until every auto-graded criterion passes |

Every chapter ends with a **Summary & Glossary** and a final **Quiz**.

## The playground on the right

- Each exercise ships 1 or 2 code panes, depending on the use case. The two panes run **independently** — enough to simulate client/server.
- Code runs **for real on your machine** through a pty: type input in the bottom box, pass arguments in the `args:` box.
- The **Scratch** tab is always available for quick experiments.
- Toolchains are data: `content/toolchains.json` says how each pane builds and runs. The template ships presets (`c`, `cpp`, `python`, `node`, `kotlin`, `java`, `rust`, `rust-cargo`) and a playbook can add its own (make, Gradle, a Python venv, …) — each pane declares its toolchain in `exercises.json`.

> Try it now: head to the quiz and then the exercise in the next sections — you'll pass through all three unlock mechanisms.
