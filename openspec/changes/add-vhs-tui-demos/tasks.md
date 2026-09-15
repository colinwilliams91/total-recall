## 1. Demo repo setup (external to the main repo)

- [ ] 1.1 Create the dedicated demo repo (e.g. `total-recall-demos`, may start private) that doubles as the scratch project the recorded session operates on. Verify the repo exists with its own Git history.
- [ ] 1.2 Add the maintainer-facing `README.md` (how to re-render with `vhs <name>.tape`, Docker fallback one-liner, note that demo refresh after UX changes is maintainer-owned and golden test failures in the main repo are the drift signal). Verify it renders.
- [ ] 1.3 Build the demo harness in the demo repo: `TR_HOME`-isolated environment tooling for the tapes (temp data dir, dummy provider key, headless `tr serve` started and seeded with known questions via `Hide`-wrapped tape sections). Verify a manual dry run shows the seeded quiz question answering end-to-end.

## 2. Demo tapes

- [ ] 2.1 Write `demo-a-full-flow.tape` (install → real `tr init` on camera with dummy key → `tr serve` → commit in the scratch project → quiz → done) with the shared visual profile (dark theme, FontSize ~20, WindowBar, ~1000px wide) and `Output demo-a-full-flow.gif` + `Output demo-a-full-flow.txt`. Verify `vhs demo-a-full-flow.tape` renders both files with no AI network calls.
- [ ] 2.2 Write `demo-b-quiz-focus.tape` (commit → question appears → answer → feedback) using the seeded headless daemon, same profile, dual outputs. Verify it renders both files; fall back to the golden-frame shim for the quiz frames only if daemon seeding proves brittle, and record which approach was used.
- [ ] 2.3 Write `demo-c-init-focus.tape` (real `tr init` huh forms with `Env TR_HOME` temp dir → provider config with dummy key → `tr repo` → `tr status`), same profile, dual outputs. Verify it renders both files.
- [ ] 2.4 Re-render all three; check each GIF is ≤ ~25s duration and reasonably sized, and text outputs diff cleanly on re-render. Verify with a second render pass.

## 3. Releases & landing page

- [ ] 3.1 Tag the demo repo (e.g. `demos-v1`) and attach the rendered GIFs as GitHub Releases assets. Verify each asset URL loads.
- [ ] 3.2 Link the leading candidate GIF's release asset URL at the very top of the main repo's `README.md` (under logo/badges) as a single image line with no added exposition. Verify it renders as the first visual on the GitHub page preview.
- [ ] 3.3 Add a one-line pointer in the main repo's `CONTRIBUTING.md`: where demos live (the demo repo), that re-rendering is maintainer-owned, and that golden test failures signal a stale demo. Verify the note appears in the contributing doc.

## 4. Comparison & wrap-up

- [ ] 4.1 Present all three rendered GIFs side by side for comparison; maintainer picks the winner. Verify decision recorded (README link updated to the winner if different from 3.2).
- [ ] 4.2 Verify the main repo gained no demo scaffolding or binaries (`git status` shows only the README link and CONTRIBUTING.md pointer) and `go build ./... && go test ./...` passes untouched.
