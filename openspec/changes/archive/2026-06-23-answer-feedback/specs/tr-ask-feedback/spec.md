# Spec: tr-ask-feedback

## Overview

Extends the `tr ask` Bubbletea state machine with a `stateFeedback` state that blocks on the server's evaluation and AI feedback response before rendering the result to the developer's terminal.

---

## Scenarios

### 1. Correct answer — feedback rendered after alt-screen

**Given** `tr ask` is in `stateQuestion` with a question displayed  
**When** the developer presses `[1]`  
**Then** `tr ask` transitions to `stateFeedback`  
**And** renders `"Evaluating..."` in the alt-screen  
**And** fires `postAnswer(id, answerIndex=0)` as a `tea.Cmd` (async, does not block the event loop)  
**When** `feedbackMsg{correct: true, correctText: "...", feedback: "..."}` is received  
**Then** `tr ask` transitions to `stateDone` and calls `tea.Quit`  
**And** after the alt-screen closes, prints to stdout:
```
✓ Correct.

  <feedback text>
```

---

### 2. Incorrect answer — correct answer named explicitly

**Given** `tr ask` is in `stateQuestion`  
**When** the developer presses `[2]` (incorrect choice)  
**Then** `stateFeedback` renders `"Evaluating..."`  
**When** `feedbackMsg{correct: false, correctText: "Prevent storms...", feedback: "Jitter randomizes..."}` is received  
**Then** after alt-screen closes, prints:
```
✗ The answer was: Prevent storms from synchronizing across clients

  Jitter randomizes each client's retry delay so they don't all
  attempt reconnection at the same moment.
```

---

### 3. Skip — gentle acknowledgement, no blocking

**Given** `tr ask` is in `stateQuestion`  
**When** the developer presses `Enter`  
**Then** `postSkip(id)` is called as a `tea.Cmd`  
**And** `tr ask` transitions directly to `stateDone` (no `stateFeedback` — skip never blocks)  
**When** `skipMsg{}` is received (confirming the POST succeeded)  
**Then** after alt-screen closes, prints:
```
→ Question saved for later.
```

---

### 4. q / Esc — silent exit, no POST

**Given** `tr ask` is in `stateQuestion`  
**When** the developer presses `q` or `Esc`  
**Then** `tr ask` transitions to `stateDone` immediately  
**And** no `POST /recall/answer` is made  
**And** the question remains unclaimed in the queue (available for future `tr ask` or MCP poll)  
**And** nothing is printed after the alt-screen closes

---

### 5. Feedback AI call failure — verdict still shown

**Given** the AI provider fails during evaluation  
**When** `postAnswer` receives a response with `{"ok":true,"correct":false,"correct_text":"...","feedback":null}`  
**Then** `feedbackMsg` has `feedback = ""`  
**Then** after alt-screen closes, prints:
```
✗ The answer was: Prevent storms from synchronizing across clients
```
(no feedback paragraph — graceful degradation with verdict only)

---

### 6. Daemon unreachable during postAnswer

**Given** the daemon stops between question display and answer submission  
**When** `postAnswer` receives a connection error  
**Then** `tr ask` transitions to `stateDone`  
**And** prints nothing (silent exit — same as TTY guard behavior)  
**And** exits 0

---

### 7. postAnswer sends answer_index, not answer text

**Given** the developer presses `[2]` for choice at index 1  
**When** `postAnswer` is called  
**Then** the HTTP body is `{"id": N, "answer_index": 1}` (0-based)  
**And** the URL includes `?feedback=true`  
**And** the `answer` text is NOT sent by `tr ask` (the server looks it up from DB)

---

### 8. stateFeedback renders static message (no animation)

**Given** `tr ask` is in `stateFeedback`  
**When** `View()` is called  
**Then** it renders `"Evaluating..."` as a static string  
**And** does NOT advance an animation frame (no tick needed — feedback is typically < 3s)

---

### 9. Ctrl+C in stateFeedback — clean exit

**Given** `tr ask` is in `stateFeedback` waiting on the AI response  
**When** the developer presses Ctrl+C  
**Then** `tr ask` transitions to `stateDone` and exits  
**And** the in-flight `postAnswer` HTTP response is discarded  
**And** the answer may or may not have been recorded (best-effort — the POST was already sent)
