// grader.js — promptfoo llm-rubric grading provider for the synthesis eval.
//
// promptfoo's built-in `openai:` provider does not cleanly target OpenRouter
// for slash-named models (deepseek/deepseek-v4-flash), so rubric grading 401s
// at api.openai.com. This provider calls OpenRouter directly with the grading
// prompt promptfoo constructs and returns the raw model text, which promptfoo
// parses into { pass, reason }.
//
// Native ESM module — see provider.js for why (promptfoo's worker re-scopes
// CJS require bindings; ESM avoids the interop entirely).
//
// Env:
//   - OPENROUTER_API_KEY (required)
//   - TR_EVAL_GRADER_MODEL (default: deepseek/deepseek-v4-flash)
//   - TR_EVAL_BASE_URL    (default: https://openrouter.ai/api/v1)
const MODEL = process.env.TR_EVAL_GRADER_MODEL || 'deepseek/deepseek-v4-flash';
const BASE_URL = process.env.TR_EVAL_BASE_URL || 'https://openrouter.ai/api/v1';

export default class TotalRecallGrader {
  id() {
    return 'total-recall-grader';
  }

  async callApi(prompt) {
    const apiKey = process.env.OPENROUTER_API_KEY;
    if (!apiKey) {
      return { output: '', error: 'OPENROUTER_API_KEY is not set (required by evals/synthesis/grader.js)' };
    }

    const body = {
      model: MODEL,
      max_tokens: 512,
      messages: [{ role: 'user', content: prompt }],
    };

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
      return { output: '', error: `OpenRouter grading request failed: ${err.message}` };
    }

    const data = await resp.json();
    if (!resp.ok || data.error) {
      return { output: '', error: `OpenRouter grading error (${resp.status}): ${JSON.stringify(data).slice(0, 500)}` };
    }
    const content = data.choices && data.choices[0] && data.choices[0].message && data.choices[0].message.content;
    return { output: content || '' };
  }
}
