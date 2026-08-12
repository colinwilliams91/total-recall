// provider.js — promptfoo custom provider for the Total Recall synthesis prompt.
//
// This is the "wrap the real code" approach: instead of a static prompt copy
// that drifts from the Go templates, it shells out to cmd/promptgen (which
// composes the prompt from the real embedded policy doc + format contract +
// user-turn logic), then calls the configured model with those exact turns.
//
// Written as a native ESM module (import/export default) — promptfoo imports
// file providers via ESM `import()`, and a CJS `module.exports` binding for
// `require('os')` gets re-scoped inside promptfoo's worker, causing
// "os is not defined". Native ESM avoids the interop entirely.
//
// Env:
//   - OPENROUTER_API_KEY (required)
//   - TR_EVAL_MODEL       (default: deepseek/deepseek-v4-flash, matches user config)
//   - TR_EVAL_BASE_URL    (default: https://openrouter.ai/api/v1)
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import path from 'node:path';
import fs from 'node:fs';
import os from 'node:os';

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const REPO_ROOT = path.resolve(__dirname, '..', '..');
const PROMPTGEN_DIR = path.join(os.tmpdir(), 'tr-promptgen');

const MODEL = process.env.TR_EVAL_MODEL || 'deepseek/deepseek-v4-flash';
const BASE_URL = process.env.TR_EVAL_BASE_URL || 'https://openrouter.ai/api/v1';

function promptgenBinPath() {
  return path.join(PROMPTGEN_DIR, process.platform === 'win32' ? 'promptgen.exe' : 'promptgen');
}

function ensurePromptgen() {
  const bin = promptgenBinPath();
  if (fs.existsSync(bin)) return bin;
  fs.mkdirSync(PROMPTGEN_DIR, { recursive: true });
  execFileSync('go', ['build', '-o', bin, './cmd/promptgen'], {
    cwd: REPO_ROOT,
    stdio: 'inherit',
  });
  return bin;
}

export default class TotalRecallSynthesisProvider {
  constructor(config) {
    // config is the provider's resolved config object; it includes the
    // `config:` block from promptfooconfig.yaml (e.g. { policy: '...' }).
    this.config = (config && config.config) || {};
  }

  id() {
    return 'total-recall-synthesis';
  }

  async callApi(prompt, context) {
    const apiKey = process.env.OPENROUTER_API_KEY;
    if (!apiKey) {
      return { output: '', error: 'OPENROUTER_API_KEY is not set (required by evals/synthesis/provider.js)' };
    }

    const bin = ensurePromptgen();
    const vars = (context && context.vars) || {};
    const policyFile = this.config.policy || '';

    const args = [
      '--concepts', JSON.stringify(vars.concepts || []),
      '--commit-msg', vars.commit_msg || '',
      '--diff', vars.diff || '',
      '--difficulty', vars.difficulty || 'intermediate',
    ];
    if (policyFile) args.push('--policy', path.resolve(REPO_ROOT, policyFile));

    let composed;
    try {
      const stdout = execFileSync(bin, args, { cwd: REPO_ROOT, encoding: 'utf8', maxBuffer: 8 * 1024 * 1024 });
      composed = JSON.parse(stdout);
    } catch (err) {
      return { output: '', error: `promptgen failed: ${err.stderr || err.message}` };
    }

    const body = {
      model: MODEL,
      max_tokens: composed.max_tokens,
      messages: [
        { role: 'system', content: composed.system },
        { role: 'user', content: composed.user },
      ],
    };
    if (composed.json) body.response_format = { type: 'json_object' };

    let resp;
    try {
      resp = await fetch(`${BASE_URL}/chat/completions`, {
        method: 'POST',
        headers: {
          'Content-Type': 'application/json',
          Authorization: `Bearer ${apiKey}`,
        },
        body: JSON.stringify(body),
      });
    } catch (err) {
      return { output: '', error: `OpenRouter request failed: ${err.message}` };
    }

    const data = await resp.json();
    if (!resp.ok || data.error) {
      return { output: '', error: `OpenRouter error (${resp.status}): ${JSON.stringify(data).slice(0, 500)}` };
    }
    const content = data.choices && data.choices[0] && data.choices[0].message && data.choices[0].message.content;
    return { output: content || '', metadata: { system_tokens: composed.system.length, user_tokens: composed.user.length } };
  }
}
