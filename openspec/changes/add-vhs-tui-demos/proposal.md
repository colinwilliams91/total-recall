## Why

The README's landing page has no visual demo. Visitors must read the Setup section to understand what `tr` actually feels like, and there is no way to see the TUI in action without installing Go, configuring an AI provider, and starting a daemon. A short, silent GIF at the top of the README is the standard way CLI tools of this kind communicate their value in the first five seconds.

## What Changes

- Add VHS (charmbracelet/vhs) tape files that script terminal sessions demoing the `tr` TUI experience.
- Produce three alternative tape files so the best demo can be compared and picked; each renders to `.gif` (and also `.txt` ASCII output for accessibility/diffing).
- All demo assets (tapes, harness, dummy setup, renders) live in a dedicated demo repo outside this codebase; rendered GIFs are published as that repo's GitHub Releases assets.
- This repo's footprint is minimal: one linked image at the very top of `README.md` plus a pointer note in `CONTRIBUTING.md`.
- Demos are simple and concise: minimal exposition, letting the TUI speak for itself; light comments in the tape files only where needed.

## Capabilities

### New Capabilities
- `tui-demo-tapes`: VHS tape files and their rendered GIF/ASCII outputs that demo the `tr` TUI on the README landing page, including how the demo sessions are scripted (what the tapes show, output formats, placement in the README).

### Modified Capabilities

## Impact

- New external repo: `total-recall-demos` (tapes, harness, scratch/dummy project, re-render notes, rendered `.gif`/`.txt` outputs; GIFs published as GitHub Releases assets).
- `.tape` files: three demo variants recording the real `tr` binary (real `tr init` huh forms; seeded headless daemon for quiz flows; no live AI provider calls).
- This repo: one linked image line near the top of `README.md` and one pointer line in `CONTRIBUTING.md`. No demo scaffolding or binaries enter this repository.
- No Go code, API, or dependency changes to the `tr` binary itself. VHS is an authoring-time tool in the demo repo only — consumers never need it.
