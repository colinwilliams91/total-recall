
- [ ] What to capture from MCP interactions (e.g. active concepts, knowledge state, etc.) and their tradeoffs (privacy, noise, etc.)
```
 TRADEOFF SUMMARY
 ═══════════════════════════════════════════════════

   Privacy          ← significant, mitigated by
                    extract-and-discard pattern

   Noise            ← needs relevance threshold
                   (same problem as git hooks,
                    same solution: dispatch policy)

   Schema           ← Event Monitor needs to normalize
                    across MCP client shapes

   Latency          ← non-issue (async, no ceiling)

   API cost         ← non-issue (deduplication handles it)

   Question quality ← strictly better with all three
```
The privacy one is worth sitting with. The extract-and-discard pattern resolves it architecturally, but it also means the privacy contract needs to be explicit to users: TR analyzes your
conversations but never stores the conversation itself, only the concept fingerprints that come out of it.

Does that feel like an acceptable contract? Or is there a version of this where you'd want to make conversation analysis opt-in separately from code-change analysis?