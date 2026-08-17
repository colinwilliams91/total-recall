# Promptfoo evals for Total Recall synthesis

Automated A/B testing for the recall question synthesis prompt. This is the
"wrap the real code" approach: the eval exercises the **actual production
prompt construction** (real embedded policy doc + format contract + user-turn
logic) via `cmd/promptgen`, so there's no YAML copy of the prompt that can
drift.

## What it evaluates

- **Synthesis quality** — the composed system+user turns, fed to the configured
  model, must produce a valid MCQ question JSON with 4+ plausible choices.
- **Counterfactual, not trivia** — llm-rubric grading checks the question asks
  WHY / what breaks if / what tradeoff, not syntax recall.
- **Topic relevance** — llm-rubric checks the question stays on the provided
  concept (and is not dragged onto the policy doc's example topics).
- **Language fidelity** — llm-rubric checks output language matches the commit
  message.
- **A/B policy variants** — the config defines two providers:
  - `embedded-policy` — uses the embedded `question-generation-policy.md`
  - `variant-policy` — uses `evals/synthesis/policies/variant.md`
  Edit `variant.md` to compare a candidate policy doc against the shipped one.

## Layout

```
evals/synthesis/
  promptfooconfig.yaml      # providers, tests, shared assertions
  provider.js               # composes real prompt via promptgen + calls model
  grader.js                 # llm-rubric grader via OpenRouter
  policies/variant.md       # A/B variant policy (seeded = copy of embedded)
  tests/*.yaml              # one fixture per sample diff
cmd/promptgen/main.go       # hermetic prompt composer (no AI calls)
```

## How the chain works

```
promptfoo ──▶ provider.js ──▶ go build ./cmd/promptgen (once, cached in $TMP)
                │                 │
                │                 └─▶ reads real embedded policy + format contract
                │                     composes system+user turns (real code)
                ▼
           OpenRouter chat/completions  (model: TR_EVAL_MODEL, default deepseek/deepseek-v4-flash)
                │
                ▼
           assertions: is-json, javascript (structure), llm-rubric (quality,
           relevance, language) graded by grader.js → OpenRouter
```

`promptgen` and `grader.js` are native ESM/dynamic — promptfoo's worker imports
file providers via ESM `import()`, and a CJS `module.exports` + `require('os')`
binding gets re-scoped, causing "os is not defined". Keep both files as ESM.

## Running

```powershell
$env:OPENROUTER_API_KEY = "<your key>"

# Validate config
npx promptfoo@latest validate config -c evals/synthesis/promptfooconfig.yaml

# Run the full suite (7 fixtures x 2 providers + grading). Costs tokens.
npx promptfoo@latest eval -c evals/synthesis/promptfooconfig.yaml --no-cache --no-share -o evals/synthesis/latest-results.json

# Run one provider only (cheaper)
npx promptfoo@latest eval -c evals/synthesis/promptfooconfig.yaml --no-cache --no-share --filter-providers "embedded-policy"

# Run one fixture
npx promptfoo@latest eval -c evals/synthesis/promptfooconfig.yaml --no-cache --no-share --filter-pattern "event sourcing"

# View results in the web UI
npx promptfoo@latest view
```

## Adding a fixture

1. Drop a sample `.go` file (or diff) in the repo.
2. Copy `evals/synthesis/tests/audit.yaml` as a template.
3. Set `description`, `vars.concepts` (JSON array), `vars.commit_msg`,
   `vars.context_desc`, `vars.commit_msg_lang`, and `vars.diff` (the file).
4. Re-run the eval.

`vars.diff` is truncated to 500 chars by `promptgen` — the same budget the
daemon uses.

## Environment

| Var | Purpose | Default |
|-----|---------|---------|
| `OPENROUTER_API_KEY` | auth for synthesis + grading | *(required)* |
| `TR_EVAL_MODEL` | model used for synthesis | `deepseek/deepseek-v4-flash` |
| `TR_EVAL_GRADER_MODEL` | model used for rubric grading | `deepseek/deepseek-v4-flash` |
| `TR_EVAL_BASE_URL` | OpenAI-compatible base URL | `https://openrouter.ai/api/v1` |

## Known behaviors

- **Transient empty outputs**: OpenRouter occasionally returns empty content
  under concurrency. `defaultTest.options.retries: 2` smooths most of these;
  a residual few will show as `is-json` failures. Re-run to confirm.
- **Rubric context**: the relevance/language rubrics reference `{{context_desc}}`
  and `{{commit_msg_lang}}` vars (set per-fixture) so the grader can see the
  input — promptfoo does not render `{{concepts}}` JSON arrays into rubric
  templates, so those two plain-string vars are the mechanism.
- **Cost**: a full run is ~14 synthesis calls + ~42 rubric calls (7 fixtures ×
  2 providers × 3 rubrics). Budget ~$0.20-0.50 on DeepSeek-class models.
