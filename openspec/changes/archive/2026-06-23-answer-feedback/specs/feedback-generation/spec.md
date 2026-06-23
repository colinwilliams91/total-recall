# Spec: feedback-generation

## Overview

Defines the server-side evaluation and AI feedback loop triggered when a terminal user answers a recall question. Covers the `POST /recall/answer` endpoint contract, the `recall.Engine.GenerateFeedback` method, the `FeedbackRequest` prompt builder, and graceful degradation when the AI call fails.

---

## Scenarios

### 1. Terminal answer — correct, feedback returned

**Given** the daemon is running with an AI provider configured  
**And** question ID 1 exists: `correct_index = 0`, `choices = ["Prevent storms", "Reduce memory", "Improve cache"]`  
**When** `POST /recall/answer?feedback=true` is called with body `{"id": 1, "answer_index": 0}`  
**Then** the server evaluates `correct = true` (0 == 0)  
**And** calls `recallEngine.GenerateFeedback` with the question, choices, `correctIndex=0`, `answerIndex=0`  
**And** stores `answer = "Prevent storms"`, `answer_index = 0`, `correct = 1`, `feedback = <AI text>`  
**And** responds `200 OK` with `{"ok":true,"correct":true,"correct_text":"Prevent storms","feedback":"<AI text>"}`

---

### 2. Terminal answer — incorrect, correct answer named in feedback

**Given** question ID 1 with `correct_index = 0`  
**When** `POST /recall/answer?feedback=true` with body `{"id": 1, "answer_index": 1}`  
**Then** `correct = false` (1 ≠ 0)  
**And** the AI prompt user turn labels choice 0 as `← correct` and choice 1 as `← chosen (incorrect)`  
**And** the AI response names the correct answer explicitly and explains why it is right  
**And** the response includes `"correct":false`, `"correct_text":"Prevent storms"`, `"feedback":"<explanation>"`

---

### 3. Feedback AI call failure — answer still recorded

**Given** the AI provider returns an error (timeout, bad key, rate limit)  
**When** `POST /recall/answer?feedback=true` is called  
**Then** `GenerateFeedback` logs the error and returns `""`  
**And** `AnswerQuestion` is still called with `feedback = ""`  
**And** the response is `{"ok":true,"correct":<bool>,"correct_text":"<text>","feedback":null}`  
**And** the HTTP status is `200 OK` — the answer is recorded regardless of feedback failure

---

### 4. No feedback when `?feedback=true` is absent

**Given** `POST /recall/answer` is called without the `?feedback=true` query parameter  
**When** the handler processes the request  
**Then** `GenerateFeedback` is NOT called  
**And** `AnswerQuestion` is called with `feedback = ""`  
**And** the response is `{"ok":true,"correct":<bool>,"correct_text":"<text>","feedback":null}`

---

### 5. Skip — no evaluation, no feedback

**Given** a question with ID 1  
**When** `POST /recall/answer` is called with body `{"id": 1, "skip": true}`  
**Then** `SkipQuestion` is called (no evaluation, no AI call)  
**And** the response is `{"ok":true}`  
**And** `answer_index`, `correct`, `feedback` remain NULL in the DB

---

### 6. Invalid answer_index — 400 response

**Given** question ID 1 has 3 choices (indices 0-2)  
**When** `POST /recall/answer?feedback=true` is called with `{"id": 1, "answer_index": 5}`  
**Then** the server responds `400 Bad Request` with `{"error":"answer_index out of range"}`  
**And** no DB write occurs

---

### 7. Unknown question ID — 404 response

**When** `POST /recall/answer?feedback=true` is called with `{"id": 99999, "answer_index": 0}`  
**And** no question with ID 99999 exists  
**Then** the server responds `404 Not Found`

---

### 8. FeedbackRequest prompt structure — correct case

**Given** a correct answer  
**When** `FeedbackRequest` builds the user turn  
**Then** the user turn lists all choices with `← correct, chosen` annotating the correct choice  
**And** ends with `"The developer answered correctly."`  
**And** the system prompt instructs: direct, plain prose, no markdown, max 3 sentences, briefly confirm and explain why correct

---

### 9. FeedbackRequest prompt structure — incorrect case

**Given** an incorrect answer  
**When** `FeedbackRequest` builds the user turn  
**Then** the user turn annotates the correct choice with `← correct` and the chosen choice with `← chosen (incorrect)`  
**And** ends with `"The developer chose option N and was incorrect."`  
**And** the system prompt instructs: name the correct answer explicitly, explain why it is right, briefly note why the chosen answer doesn't fit, do not apologize

---

### 10. Feedback token budget enforced

**Given** any feedback request  
**When** `FeedbackRequest` is built  
**Then** `MaxTokens` is set to `feedbackMaxTokens` (150)  
**And** `JSON` is false (plain prose, not JSON)
