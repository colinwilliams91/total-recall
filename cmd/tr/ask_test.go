package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/colinwilliams91/total-recall/internal/hooks"
)

func TestAskViewShowsCaughtUpMessageInFinalWindow(t *testing.T) {
	m := newAskModel(10*time.Second, "", "main")
	m.started = time.Now().Add(-7 * time.Second)

	view := m.View()
	if !strings.Contains(view, caughtUpMessage) {
		t.Fatalf("expected caught-up message in view, got %q", view)
	}
}

func TestAskTimeoutSetsCaughtUpFeedback(t *testing.T) {
	m := newAskModel(2*time.Second, "", "main")
	m.started = time.Now().Add(-3 * time.Second)

	updated, _ := m.updateThinking(tickMsg{})
	got := updated.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if got.advisory != caughtUpMessage {
		t.Fatalf("expected advisory %q, got %q", caughtUpMessage, got.advisory)
	}
}

func TestRenderQuestionShowsAllChoices(t *testing.T) {
	view := renderQuestion(questionMsg{
		question: "What does the diff hunk header mean?",
		choices: []string{"first", "second", "third", "fourth"},
	})

	if !strings.Contains(view, "4. fourth") {
		t.Fatalf("expected fourth choice in view, got %q", view)
	}
	if !strings.Contains(view, "[1-4] or Enter to skip: ") {
		t.Fatalf("expected dynamic prompt in view, got %q", view)
	}
}

func TestParseChoiceSelectionSupportsFourthChoice(t *testing.T) {
	idx, ok := parseChoiceSelection("4", 4)
	if !ok {
		t.Fatalf("expected fourth choice to parse")
	}
	if idx != 3 {
		t.Fatalf("expected index 3, got %d", idx)
	}
}

func TestAskDaemonUnreachableShowsAdvisory(t *testing.T) {
	m := newAskModel(10*time.Second, "", "main")

	updated, _ := m.updateThinking(daemonUnreachableMsg{})
	got := updated.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if got.advisory != daemonUnavailableMessage {
		t.Fatalf("expected advisory %q, got %q", daemonUnavailableMessage, got.advisory)
	}
}

func TestAskProgramRunKeepsCaughtUpFeedbackOnTimeout(t *testing.T) {
	m := newAskModel(200*time.Millisecond, "", "main")
	m.httpClient.Timeout = 50 * time.Millisecond
	m.started = time.Now().Add(-250 * time.Millisecond)

	p := tea.NewProgram(m, tea.WithInput(nil), tea.WithOutput(io.Discard))
	finalModel, err := p.Run()
	if err != nil {
		t.Fatalf("program run failed: %v", err)
	}

	got, ok := finalModel.(askModel)
	if !ok {
		t.Fatalf("expected askModel, got %T", finalModel)
	}
	if got.advisory != caughtUpMessage {
		t.Fatalf("expected advisory %q, got %q", caughtUpMessage, got.advisory)
	}
}

func TestAskModelSendsAnswerOnKeyPress(t *testing.T) {
	m := newAskModel(10*time.Second, "", "main")

	updated, _ := m.updateThinking(questionMsg{
		id:       1,
		question: "What is the meaning of life?",
		choices:  []string{"42", "To crush your enemies", "To learn Go", "All of the above"},
	})
	m2 := updated.(askModel)

	if m2.state != stateQuestion {
		t.Fatalf("expected stateQuestion, got %v", m2.state)
	}

	final, cmd := m2.updateQuestion(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'1'}}))
	got := final.(askModel)

	if got.state != stateFeedback {
		t.Fatalf("expected stateFeedback after selecting choice, got %v", got.state)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (postAnswer)")
	}
}

func TestAskModelExitsOnCtrlC(t *testing.T) {
	m := newAskModel(10*time.Second, "", "main")

	final, cmd := m.updateThinking(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlC}))
	got := final.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone after Ctrl+C, got %v", got.state)
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (Quit)")
	}
}

func TestAskModelEnterKeySkipsQuestion(t *testing.T) {
	m := newAskModel(10*time.Second, "", "main")

	updated, _ := m.updateThinking(questionMsg{
		id:       1,
		question: "Skip me?",
		choices:  []string{"yes", "no"},
	})
	m2 := updated.(askModel)

	if m2.state != stateQuestion {
		t.Fatalf("expected stateQuestion, got %v", m2.state)
	}

	final, cmd := m2.updateQuestion(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
	got := final.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone after Enter, got %v", got.state)
	}
	if !got.skipped {
		t.Fatal("expected skipped to be true after Enter")
	}
	if cmd == nil {
		t.Fatal("expected non-nil cmd (postSkip)")
	}
}

func TestAskModelIgnoresOutOfRangeChoice(t *testing.T) {
	m := newAskModel(10*time.Second, "", "main")

	updated, _ := m.updateThinking(questionMsg{
		id:       1,
		question: "Pick one?",
		choices:  []string{"a", "b", "c", "d"},
	})
	m2 := updated.(askModel)

	if m2.state != stateQuestion {
		t.Fatalf("expected stateQuestion, got %v", m2.state)
	}

	final, cmd := m2.updateQuestion(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'9'}}))
	got := final.(askModel)

	if got.state != stateQuestion {
		t.Fatalf("expected stateQuestion for out-of-range choice, got %v", got.state)
	}
	if cmd != nil {
		t.Fatal("expected nil cmd for unrecognized key")
	}
}

// isQuitCmd executes a tea.Cmd and reports whether it yields tea.QuitMsg.
func isQuitCmd(cmd tea.Cmd) bool {
	if cmd == nil {
		return false
	}
	_, ok := cmd().(tea.QuitMsg)
	return ok
}

func TestUpdateFeedbackReceivesFeedbackMsg(t *testing.T) {
	m := newAskModel(10 * time.Second, "", "main")
	m.state = stateFeedback

	updated, cmd := m.updateFeedback(feedbackMsg{correct: true, correctText: "X", feedback: "Y"})
	got := updated.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if !got.feedbackResult.correct {
		t.Fatal("expected feedbackResult.correct true")
	}
	if got.feedbackResult.correctText != "X" {
		t.Fatalf("expected correctText %q, got %q", "X", got.feedbackResult.correctText)
	}
	if got.feedbackResult.feedback != "Y" {
		t.Fatalf("expected feedback %q, got %q", "Y", got.feedbackResult.feedback)
	}
	if !isQuitCmd(cmd) {
		t.Fatal("expected tea.Quit cmd")
	}
}

func TestUpdateFeedbackReceivesSkipMsg(t *testing.T) {
	m := newAskModel(10 * time.Second, "", "main")
	m.state = stateFeedback

	updated, cmd := m.updateFeedback(skipMsg{})
	got := updated.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if !got.skipped {
		t.Fatal("expected skipped true")
	}
	if !isQuitCmd(cmd) {
		t.Fatal("expected tea.Quit cmd")
	}
}

func TestUpdateFeedbackCtrlC(t *testing.T) {
	m := newAskModel(10 * time.Second, "", "main")
	m.state = stateFeedback

	updated, cmd := m.updateFeedback(tea.KeyMsg(tea.Key{Type: tea.KeyCtrlC}))
	got := updated.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if !isQuitCmd(cmd) {
		t.Fatal("expected tea.Quit cmd")
	}
}

func TestStateFeedbackView(t *testing.T) {
	m := newAskModel(10 * time.Second, "", "main")
	m.state = stateFeedback

	view := m.View()
	if !strings.Contains(view, "Evaluating...") {
		t.Fatalf("expected view to contain %q, got %q", "Evaluating...", view)
	}
	if strings.Contains(view, "Thinking.") {
		t.Fatalf("expected view to NOT contain %q, got %q", "Thinking.", view)
	}
}

func TestPostAnswerParsesResponse(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	origURL := daemonBaseURL
	daemonBaseURL = baseURL
	t.Cleanup(func() { daemonBaseURL = origURL })

	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "/path/to/repo", "main", "post-answer q", []string{"correct-a", "wrong-b"}, 0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := mustGET(t, baseURL, "/recall/next?repo="+url.QueryEscape("/path/to/repo")+"&branch=main")
	var q struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
		t.Fatalf("decode next: %v", err)
	}
	resp.Body.Close()

	cmd := postAnswer(q.ID, 0, "/path/to/repo", "main", &http.Client{Timeout: 3 * time.Second})
	msg := cmd()

	fm, ok := msg.(feedbackMsg)
	if !ok {
		t.Fatalf("expected feedbackMsg, got %T", msg)
	}
	if !fm.correct {
		t.Fatal("expected correct true")
	}
	if fm.correctText != "correct-a" {
		t.Fatalf("expected correctText %q, got %q", "correct-a", fm.correctText)
	}
}

func TestPostSkipReturnsSkipMsg(t *testing.T) {
	_, store, baseURL := startTestDaemon(t)
	origURL := daemonBaseURL
	daemonBaseURL = baseURL
	t.Cleanup(func() { daemonBaseURL = origURL })

	ctx := context.Background()
	if err := store.SaveQuestion(ctx, "/path/to/repo", "main", "post-skip q", []string{"a", "b"}, 0); err != nil {
		t.Fatalf("seed: %v", err)
	}

	resp := mustGET(t, baseURL, "/recall/next?repo="+url.QueryEscape("/path/to/repo")+"&branch=main")
	var q struct {
		ID int64 `json:"id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&q); err != nil {
		t.Fatalf("decode next: %v", err)
	}
	resp.Body.Close()

	cmd := postSkip(q.ID, "/path/to/repo", "main", &http.Client{Timeout: 3 * time.Second})
	msg := cmd()

	if _, ok := msg.(skipMsg); !ok {
		t.Fatalf("expected skipMsg, got %T", msg)
	}
}

func TestPostAltScreenRendering(t *testing.T) {
	// Correct answer with feedback.
	correctOut := renderPostAltScreen(askModel{
		feedbackResult: feedbackMsg{correct: true, correctText: "X", feedback: "Great work!"},
	})
	if !strings.Contains(correctOut, "✓ Correct.") {
		t.Fatalf("correct case: expected %q in output, got %q", "✓ Correct.", correctOut)
	}
	if !strings.Contains(correctOut, "Great work!") {
		t.Fatalf("correct case: expected feedback in output, got %q", correctOut)
	}

	// Incorrect answer with feedback.
	incorrectOut := renderPostAltScreen(askModel{
		feedbackResult: feedbackMsg{correct: false, correctText: "The right answer", feedback: "Because..."},
	})
	if !strings.Contains(incorrectOut, "✗ The answer was: The right answer") {
		t.Fatalf("incorrect case: expected correctText in output, got %q", incorrectOut)
	}
	if !strings.Contains(incorrectOut, "Because...") {
		t.Fatalf("incorrect case: expected feedback in output, got %q", incorrectOut)
	}

	// Skip.
	skipOut := renderPostAltScreen(askModel{skipped: true})
	if !strings.Contains(skipOut, "→ Question saved for later.") {
		t.Fatalf("skip case: expected skip message, got %q", skipOut)
	}

	// Advisory.
	advisoryOut := renderPostAltScreen(askModel{advisory: "caught up message"})
	if advisoryOut != "caught up message\n" {
		t.Fatalf("advisory case: expected %q, got %q", "caught up message\n", advisoryOut)
	}

	// q/Esc exit — nothing printed.
	emptyOut := renderPostAltScreen(askModel{})
	if emptyOut != "" {
		t.Fatalf("q/Esc case: expected empty output, got %q", emptyOut)
	}
}

func TestAskModelQKeyExitsSilently(t *testing.T) {
	m := newAskModel(10 * time.Second, "", "main")

	updated, _ := m.updateThinking(questionMsg{
		id:       1,
		question: "Silent exit?",
		choices:  []string{"a", "b"},
	})
	m2 := updated.(askModel)
	if m2.state != stateQuestion {
		t.Fatalf("expected stateQuestion, got %v", m2.state)
	}

	final, cmd := m2.updateQuestion(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'q'}}))
	got := final.(askModel)

	if got.state != stateDone {
		t.Fatalf("expected stateDone after q, got %v", got.state)
	}
	if !isQuitCmd(cmd) {
		t.Fatal("expected tea.Quit cmd")
	}
	if got.skipped {
		t.Fatal("expected skipped false for q exit")
	}
	if got.advisory != "" {
		t.Fatalf("expected empty advisory for q exit, got %q", got.advisory)
	}
	if got.feedbackResult.correctText != "" {
		t.Fatal("expected empty feedbackResult for q exit")
	}
	if out := renderPostAltScreen(got); out != "" {
		t.Fatalf("expected no post-alt-screen output for q exit, got %q", out)
	}
}

// ── 10. ask.go URL encoding tests ────────────────────────────────────────────

// captureRequestServer starts an httptest.Server that records the request URL
// and returns the given status code with the optional JSON body.
func captureRequestServer(t *testing.T, status int, body string) (*httptest.Server, *string) {
	t.Helper()
	var captured string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		captured = r.URL.RequestURI()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		if body != "" {
			w.Write([]byte(body))
		}
	}))
	t.Cleanup(srv.Close)
	return srv, &captured
}

// withDaemonBaseURL temporarily sets the package-level daemonBaseURL for a test.
func withDaemonBaseURL(t *testing.T, newURL string) {
	t.Helper()
	orig := daemonBaseURL
	daemonBaseURL = newURL
	t.Cleanup(func() { daemonBaseURL = orig })
}

// Task 10.10: pollCmd appends ?repo=<url-encoded>&branch=<url-encoded> when
// both are non-empty. (Pre-Y1 the URL was repo-only; Y1 makes branch mandatory.)
func TestPollCmdAppendsRepoAndBranchQueryParam(t *testing.T) {
	srv, captured := captureRequestServer(t, http.StatusNoContent, "")
	withDaemonBaseURL(t, srv.URL)

	m := newAskModel(10*time.Second, "/path/to/repo", "feature-X")
	cmd := m.pollCmd()
	cmd()

	expectedRepo := "repo=" + url.QueryEscape("/path/to/repo")
	expectedBranch := "branch=" + url.QueryEscape("feature-X")
	if !strings.Contains(*captured, expectedRepo) {
		t.Fatalf("expected %q in URL, got %q", expectedRepo, *captured)
	}
	if !strings.Contains(*captured, expectedBranch) {
		t.Fatalf("expected %q in URL, got %q", expectedBranch, *captured)
	}
}

// TestPollCmdOmitsRepoQueryParamWhenEmpty is removed in Y1: the global-pool
// fallback behavior is gone, and RunE short-circuits before newAskModel when
// repo or branch resolution fails. The pollCmd always includes both params.

// Task 10.12: postAnswer appends &repo=<url-encoded>&branch=<url-encoded>.
func TestPostAnswerAppendsRepoAndBranchQueryParam(t *testing.T) {
	body := `{"ok":true,"correct":true,"correct_text":"a","feedback":""}`
	srv, captured := captureRequestServer(t, http.StatusOK, body)
	withDaemonBaseURL(t, srv.URL)

	cmd := postAnswer(42, 0, "/path/to/repo", "feature-X", &http.Client{Timeout: 3 * time.Second})
	cmd()

	expectedRepo := "repo=" + url.QueryEscape("/path/to/repo")
	expectedBranch := "branch=" + url.QueryEscape("feature-X")
	if !strings.Contains(*captured, expectedRepo) {
		t.Fatalf("expected %q in URL, got %q", expectedRepo, *captured)
	}
	if !strings.Contains(*captured, expectedBranch) {
		t.Fatalf("expected %q in URL, got %q", expectedBranch, *captured)
	}
}

// Task 10.13: postSkip appends ?repo=<url-encoded>&branch=<url-encoded>.
func TestPostSkipAppendsRepoAndBranchQueryParam(t *testing.T) {
	srv, captured := captureRequestServer(t, http.StatusOK, `{"ok":true}`)
	withDaemonBaseURL(t, srv.URL)

	cmd := postSkip(42, "/path/to/repo", "feature-X", &http.Client{Timeout: 3 * time.Second})
	cmd()

	expectedRepo := "repo=" + url.QueryEscape("/path/to/repo")
	expectedBranch := "branch=" + url.QueryEscape("feature-X")
	if !strings.Contains(*captured, expectedRepo) {
		t.Fatalf("expected %q in URL, got %q", expectedRepo, *captured)
	}
	if !strings.Contains(*captured, expectedBranch) {
		t.Fatalf("expected %q in URL, got %q", expectedBranch, *captured)
	}
}

// Task 10.14: resolveAskRepo returns the underlying error from FindRepoRoot.
// (Pre-Y1 it logged an advisory and returned ""; Y1 propagates the error so
// the caller in askCmd().RunE can print the advisory.)
func TestResolveAskRepoReturnsErrorOnGitFailure(t *testing.T) {
	origFindRepoRoot := hooks.FindRepoRoot
	hooks.FindRepoRoot = func() (string, error) {
		return "", fmt.Errorf("not a git repository")
	}
	t.Cleanup(func() { hooks.FindRepoRoot = origFindRepoRoot })

	repo, err := resolveAskRepo()
	if err == nil {
		t.Fatalf("expected error from resolveAskRepo, got nil (repo=%q)", repo)
	}
	if repo != "" {
		t.Fatalf("expected empty repo on error, got %q", repo)
	}
}

// TestResolveAskBranchReturnsBranchOnSuccess asserts the happy path of
// resolveAskBranch by stubbing `git rev-parse --abbrev-ref HEAD` via PATH.
func TestResolveAskBranchReturnsBranchOnSuccess(t *testing.T) {
	// Create a temporary directory and a fake `git` script that prints a
	// branch name. PATH is overridden so exec.Command finds the fake.
	tmpDir := t.TempDir()
	gitScript := tmpDir + string(os.PathSeparator) + "git"
	if runtime.GOOS == "windows" {
		gitScript += ".bat"
		script := "@echo off\r\necho feature-X\r\n"
		if err := os.WriteFile(gitScript, []byte(script), 0o644); err != nil {
			t.Fatalf("write fake git: %v", err)
		}
	} else {
		script := "#!/bin/sh\necho feature-X\n"
		if err := os.WriteFile(gitScript, []byte(script), 0o755); err != nil {
			t.Fatalf("write fake git: %v", err)
		}
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+origPath)

	branch, err := resolveAskBranch()
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if branch != "feature-X" {
		t.Fatalf("expected branch %q, got %q", "feature-X", branch)
	}
}

// TestResolveAskBranchReturnsErrorOnDetachedHEAD asserts that resolveAskBranch
// returns an error when `git rev-parse --abbrev-ref HEAD` returns the literal
// "HEAD" (detached HEAD state).
func TestResolveAskBranchReturnsErrorOnDetachedHEAD(t *testing.T) {
	tmpDir := t.TempDir()
	gitScript := tmpDir + string(os.PathSeparator) + "git"
	if runtime.GOOS == "windows" {
		gitScript += ".bat"
		script := "@echo off\r\necho HEAD\r\n"
		if err := os.WriteFile(gitScript, []byte(script), 0o644); err != nil {
			t.Fatalf("write fake git: %v", err)
		}
	} else {
		script := "#!/bin/sh\necho HEAD\n"
		if err := os.WriteFile(gitScript, []byte(script), 0o755); err != nil {
			t.Fatalf("write fake git: %v", err)
		}
	}
	origPath := os.Getenv("PATH")
	t.Setenv("PATH", tmpDir+string(os.PathListSeparator)+origPath)

	branch, err := resolveAskBranch()
	if err == nil {
		t.Fatalf("expected error on detached HEAD, got branch=%q", branch)
	}
	if branch != "" {
		t.Fatalf("expected empty branch on error, got %q", branch)
	}
	if !strings.Contains(err.Error(), "detached HEAD") {
		t.Fatalf("expected detached HEAD error, got %v", err)
	}
}