# Demo chapter summary

- A playbook = **chapters → sections**; three section types: reading, inline quiz, exercise — each with its own unlock mechanism.
- Exercises are auto-graded by **checks**: regexes against each pane's output; all passing ⇒ section done.
- The playground runs **real** code on your machine through a pty, one independent process per pane; toolchains are defined in `content/toolchains.json` (presets `c`, `cpp`, `python`, `node`, `kotlin`, `java`, `rust`, `rust-cargo`, or your own).
- A sample solution always sits behind the "Sample solution" disclosure, with copy-into-editor buttons and a completion fallback.
- Chapters end with a summary + glossary + final quiz (this very page and the next one).
