# Spec: mcp-answer-updates

## Overview

Updates the MCP tool surface for Phase 4C: `recall_next` exposes `correct_index` to AI agent callers, `recall_answer` accepts `answer_index` and returns correctness data, and `recallWorkflowInstructions` is updated to guide agents to self-explain. The `recall://recent` resource is enriched with answer history.

---

## Scenarios

### 1. recall_next returns correct_index

**Given** a pending question with `correct_index = 2`  
**When** an MCP client calls `recall_next`  
**Then** the tool result JSON includes `"correct_index": 2`  
**And** the full response shape is:
```json
{
  "id": 1,
  "question": "Why is jitter added...",
  "choices": ["Prevent storms", "Reduce memory", "Improve cache"],
  "correct_index": 2
}
```

---

### 2. recall_next with empty queue — no correct_index

**Given** the question queue is empty  
**When** an MCP client calls `recall_next`  
**Then** the tool result is `{"question": null}`  
**And** no `correct_index` is included

---

### 3. recall_answer — correct answer submitted

**Given** question ID 1 with `correct_index = 0`  
**When** an MCP client calls `recall_answer` with `{"id": 1, "answer_index": 0}`  
**Then** the server evaluates `correct = true`  
**And** calls `AnswerQuestion(ctx, 1, 0, "Prevent storms", true, "")` — no AI feedback call  
**And** returns:
```json
{
  "ok": true,
  "correct": true,
  "correct_index": 0,
  "correct_text": "Prevent storms from synchronizing across clients"
}
```

---

### 4. recall_answer — incorrect answer submitted

**Given** question ID 1 with `correct_index = 0`  
**When** an MCP client calls `recall_answer` with `{"id": 1, "answer_index": 1}`  
**Then** `correct = false`  
**And** returns `{"ok":true,"correct":false,"correct_index":0,"correct_text":"Prevent storms..."}`  
**And** no AI call is made — the agent is expected to self-explain

---

### 5. recall_answer — skip

**Given** question ID 1  
**When** an MCP client calls `recall_answer` with `{"id": 1, "skip": true}`  
**Then** `SkipQuestion` is called  
**And** returns `{"ok": true}`  
**And** `answer_index`, `correct`, `feedback` remain NULL in the DB

---

### 6. recall_workflow prompt instructs self-explanation

**Given** an MCP client retrieves the `recall_workflow` prompt  
**Then** the prompt text instructs the agent:
  - After calling `recall_next` and `recall_answer`, tell the user whether their answer was correct
  - Provide a brief, direct explanation — especially for incorrect answers
  - Use its own knowledge for the explanation — do not invoke a separate AI tool
  - If the queue is empty, continue normally without interrupting the workflow

---

### 7. recall://recent returns enriched history

**Given** three questions have been answered: one correct (terminal), one incorrect (MCP), one skipped  
**When** an MCP client reads `recall://recent`  
**Then** the JSON array includes all three rows  
**And** the correct terminal row includes `"correct": true`, `"feedback": "<AI text>"`, `"answer_index": 0`, `"correct_index": 0`  
**And** the incorrect MCP row includes `"correct": false`, `"feedback": null`, `"answer_index": 1`, `"correct_index": 0`  
**And** the skipped row has `"answer_index": null`, `"correct": null`, `"feedback": null`

---

### 8. recall_answer — unknown question ID

**When** an MCP client calls `recall_answer` with an ID that does not exist  
**Then** the tool returns an error result  
**And** no DB write occurs

---

### 9. recall_answer input struct updated — no breaking change on skip field

**Given** the existing `recallAnswerIn` struct used `Answer string`  
**When** updated to `{ID int64, AnswerIndex *int, Skip bool}`  
**Then** existing MCP callers that pass `{"id": N, "answer": "text"}` receive a 400-equivalent tool error  
**And** the change is documented as a breaking tool input change in the proposal
