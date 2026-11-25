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
	"github.com/colinwilliams91/total-recall/internal/cache"
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
		choices:  []string{"first", "second", "third", "fourth"},
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
	m := newAskModel(10*time.Second, "", "main")
	m.state = stateFeedback

	// The feedback resolves into the one offer probe per session — the model
	// stays in stateFeedback until the probe resolves.
	updated, cmd := m.updateFeedback(feedbackMsg{correct: true, correctText: "X", feedback: "Y"})
	got := updated.(askModel)

	if got.state != stateFeedback {
		t.Fatalf("expected stateFeedback while probing, got %v", got.state)
	}
	if got.feedbackResult.correctText != "X" || got.feedbackResult.feedback != "Y" {
		t.Fatalf("expected feedbackResult retained, got %+v", got.feedbackResult)
	}
	if cmd == nil {
		t.Fatal("expected the offer probe cmd")
	}

	// No offer → the session ends exactly as it does without resurfacing.
	final, quit := got.updateFeedback(noOfferMsg{})
	done := final.(askModel)
	if done.state != stateDone {
		t.Fatalf("expected stateDone after no-offer, got %v", done.state)
	}
	if !isQuitCmd(quit) {
		t.Fatal("expected tea.Quit cmd")
	}

	// A served offer continues into the offer flow instead of quitting.
	served, _ := got.updateFeedback(bonusOfferMsg{slug: "bonus", prose: "prose"})
	offered := served.(askModel)
	if offered.state != stateOffer || offered.offer == nil || offered.offer.slug != "bonus" {
		t.Fatalf("expected stateOffer with the served offer, got state=%v offer=%+v", offered.state, offered.offer)
	}
	if !offered.offerShown {
		t.Fatal("expected offerShown to mark the inline feedback render")
	}
}

func TestUpdateFeedbackReceivesSkipMsg(t *testing.T) {
	m := newAskModel(10*time.Second, "", "main")
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
	m := newAskModel(10*time.Second, "", "main")
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
	m := newAskModel(10*time.Second, "", "main")
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
	if err := store.SaveQuestion(ctx, "/path/to/repo", "main", "post-answer q",
		[]cache.Choice{
			{Text: "correct-a", IsCorrect: true, Position: 0},
			{Text: "wrong-b", IsCorrect: false, Position: 1},
		}, "", nil); err != nil {
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
	if err := store.SaveQuestion(ctx, "/path/to/repo", "main", "post-skip q",
		[]cache.Choice{
			{Text: "a", IsCorrect: true, Position: 0},
			{Text: "b", IsCorrect: false, Position: 1},
		}, "", nil); err != nil {
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
	m := newAskModel(10*time.Second, "", "main")

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

// ── bonus offer flow (spaced-repetition-resurfacing, tasks 4.1-4.3) ──────────

// offerModel returns a model in the offer state with feedback retained — the
// shape the session reaches when a bonus offer was served.
func offerModel() askModel {
	m := newAskModel(10*time.Second, "/repo/test", "main")
	m.state = stateOffer
	m.offerProbed = true
	m.offerShown = true
	m.feedbackResult = feedbackMsg{correct: true, correctText: "X", feedback: "Because circuits break."}
	offer := bonusOfferMsg{slug: "circuit_breaker", prose: "circuit breaker pattern"}
	m.offer = &offer
	return m
}

// TestAskOfferAcceptFetchesBonusQuestion verifies the y path: y → thinking
// animation while synthesis runs → the bonus question arrives as a normal
// questionMsg → the existing question interaction takes over.
func TestAskOfferAcceptFetchesBonusQuestion(t *testing.T) {
	m := offerModel()

	updated, cmd := m.updateOffer(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'y'}}))
	got := updated.(askModel)
	if !got.awaitingBonus {
		t.Fatal("expected awaitingBonus while synthesis runs")
	}
	if got.state != stateOffer {
		t.Fatalf("expected to stay in stateOffer, got %v", got.state)
	}
	if cmd == nil {
		t.Fatal("expected the accept cmd")
	}

	final, _ := got.updateOffer(questionMsg{id: 7, question: "bonus?", choices: []string{"a", "b"}})
	bonus := final.(askModel)
	if bonus.state != stateQuestion {
		t.Fatalf("expected stateQuestion, got %v", bonus.state)
	}
	if !bonus.questionIsBonus {
		t.Fatal("expected questionIsBonus for the bonus question")
	}
	if bonus.question.id != 7 {
		t.Fatalf("expected the bonus question payload, got %+v", bonus.question)
	}
}

// TestAskOfferDeclinePostsAndEnds verifies the n path: the decline POST
// settles before the session ends, and the declined concept never shows a
// question.
func TestAskOfferDeclinePostsAndEnds(t *testing.T) {
	var capturedBody string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		capturedBody = string(b)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"ok":true,"exited":false}`))
	}))
	t.Cleanup(srv.Close)
	withDaemonBaseURL(t, srv.URL)
	m := offerModel()

	updated, cmd := m.updateOffer(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'n'}}))
	got := updated.(askModel)
	if cmd == nil {
		t.Fatal("expected the decline cmd")
	}
	msg := cmd()
	if _, ok := msg.(declinePostedMsg); !ok {
		t.Fatalf("expected declinePostedMsg, got %T", msg)
	}
	if !strings.Contains(capturedBody, "circuit_breaker") {
		t.Fatalf("expected the declined slug in the POST body, got %q", capturedBody)
	}

	final, quit := got.updateOffer(declinePostedMsg{})
	done := final.(askModel)
	if done.state != stateDone {
		t.Fatalf("expected stateDone after the decline settles, got %v", done.state)
	}
	if !isQuitCmd(quit) {
		t.Fatal("expected tea.Quit cmd")
	}
}

// TestAskOfferKilledEndsSilently verifies the kill path at the offer: q, Esc
// and Ctrl+C all end the session without posting anything — the daemon's
// serve-time defer already stands.
func TestAskOfferKilledEndsSilently(t *testing.T) {
	srv, captured := captureRequestServer(t, http.StatusOK, `{"ok":true}`)
	withDaemonBaseURL(t, srv.URL)

	for _, key := range []struct {
		name string
		msg  tea.KeyMsg
	}{
		{"q", tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'q'}})},
		{"esc", tea.KeyMsg(tea.Key{Type: tea.KeyEsc})},
		{"ctrl+c", tea.KeyMsg(tea.Key{Type: tea.KeyCtrlC})},
	} {
		t.Run(key.name, func(t *testing.T) {
			m := offerModel()
			final, cmd := m.updateOffer(key.msg)
			got := final.(askModel)
			if got.state != stateDone {
				t.Fatalf("expected stateDone on %s, got %v", key.name, got.state)
			}
			if !isQuitCmd(cmd) {
				t.Fatal("expected tea.Quit cmd")
			}
			// Nothing was posted: no decline, no accept.
			if strings.Contains(*captured, "/recall/bonus/decline") || strings.Contains(*captured, "/recall/bonus/accept") {
				t.Fatalf("expected no bonus POSTs on %s, got %q", key.name, *captured)
			}
			// The feedback rendered inline, so nothing repeats post-exit.
			if out := renderPostAltScreen(got); out != "" {
				t.Fatalf("expected empty post-exit output on %s, got %q", key.name, out)
			}
		})
	}
}

// TestAskAwaitingBonusKilledEndsSilently verifies the kill path after the
// accept, while synthesis is still running: q, Esc and Ctrl+C end the session
// without posting anything — no decline and no selection, the daemon's
// serve-time defer already stands.
func TestAskAwaitingBonusKilledEndsSilently(t *testing.T) {
	srv, captured := captureRequestServer(t, http.StatusOK, `{"ok":true}`)
	withDaemonBaseURL(t, srv.URL)

	for _, key := range []struct {
		name string
		msg  tea.KeyMsg
	}{
		{"q", tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'q'}})},
		{"esc", tea.KeyMsg(tea.Key{Type: tea.KeyEsc})},
		{"ctrl+c", tea.KeyMsg(tea.Key{Type: tea.KeyCtrlC})},
	} {
		t.Run(key.name, func(t *testing.T) {
			// Accept to enter the awaitingBonus state; the accept cmd itself
			// is never run, mirroring a session killed mid-synthesis.
			m := offerModel()
			accepted, _ := m.updateOffer(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'y'}}))
			waiting := accepted.(askModel)
			if !waiting.awaitingBonus {
				t.Fatal("expected awaitingBonus after the accept key")
			}

			final, cmd := waiting.updateOffer(key.msg)
			got := final.(askModel)
			if got.state != stateDone {
				t.Fatalf("expected stateDone on %s, got %v", key.name, got.state)
			}
			if !isQuitCmd(cmd) {
				t.Fatal("expected tea.Quit cmd")
			}
			// Nothing was posted: no decline, no selection.
			if strings.Contains(*captured, "/recall/bonus/decline") || strings.Contains(*captured, "/recall/select") {
				t.Fatalf("expected no POSTs on %s, got %q", key.name, *captured)
			}
			// The feedback rendered inline in the offer view, so nothing
			// repeats post-exit.
			if out := renderPostAltScreen(got); out != "" {
				t.Fatalf("expected empty post-exit output on %s, got %q", key.name, out)
			}
		})
	}
}

// TestAskOfferAcceptFailureEndsWithoutDecline verifies the failed-fetch path:
// the session ends, nothing is posted, the feedback is not repeated.
func TestAskOfferAcceptFailureEndsWithoutDecline(t *testing.T) {
	m := offerModel()

	final, quit := m.updateOffer(acceptFailedMsg{})
	got := final.(askModel)
	if got.state != stateDone {
		t.Fatalf("expected stateDone on accept failure, got %v", got.state)
	}
	if !isQuitCmd(quit) {
		t.Fatal("expected tea.Quit cmd")
	}
	if out := renderPostAltScreen(got); out != "" {
		t.Fatalf("expected empty post-exit output (feedback shown inline), got %q", out)
	}
}

// TestAskBonusQuestionResolvedEndsSession verifies the bonus question rides
// the existing interaction and that resolving it ends the session without a
// second offer probe.
func TestAskBonusQuestionResolvedEndsSession(t *testing.T) {
	t.Run("answered", func(t *testing.T) {
		m := offerModel()
		updated, _ := m.updateOffer(questionMsg{id: 7, question: "bonus?", choices: []string{"a", "b"}})
		bonusQ := updated.(askModel)

		updated, cmd := bonusQ.updateQuestion(tea.KeyMsg(tea.Key{Type: tea.KeyRunes, Runes: []rune{'1'}}))
		answered := updated.(askModel)
		if answered.state != stateFeedback || cmd == nil {
			t.Fatalf("expected stateFeedback with postAnswer, got state=%v cmd=%v", answered.state, cmd)
		}
		if !answered.bonusAnswered {
			t.Fatal("expected bonusAnswered so the post-exit output prints the bonus feedback")
		}

		// The bonus feedback resolves straight to done — no second probe.
		final, quit := answered.updateFeedback(feedbackMsg{correct: false, correctText: "a", feedback: "not quite"})
		done := final.(askModel)
		if done.state != stateDone {
			t.Fatalf("expected stateDone, got %v", done.state)
		}
		if !isQuitCmd(quit) {
			t.Fatal("expected tea.Quit cmd")
		}
		if out := renderPostAltScreen(done); !strings.Contains(out, "✗ The answer was: a") {
			t.Fatalf("expected the bonus verdict post-exit, got %q", out)
		}
	})

	t.Run("skipped", func(t *testing.T) {
		m := offerModel()
		updated, _ := m.updateOffer(questionMsg{id: 7, question: "bonus?", choices: []string{"a", "b"}})
		bonusQ := updated.(askModel)

		final, cmd := bonusQ.updateQuestion(tea.KeyMsg(tea.Key{Type: tea.KeyEnter}))
		done := final.(askModel)
		if done.state != stateDone || !done.skipped {
			t.Fatalf("expected stateDone+skipped, got state=%v skipped=%v", done.state, done.skipped)
		}
		if cmd == nil {
			t.Fatal("expected postSkip cmd")
		}
		if !done.bonusAnswered {
			t.Fatal("expected bonusAnswered on the bonus skip")
		}
		if out := renderPostAltScreen(done); !strings.Contains(out, "→ Question saved for later.") {
			t.Fatalf("expected the skip message post-exit, got %q", out)
		}
	})
}

// TestAskOfferViewRendersAgreedCopy verifies the offer view renders the commit
// feedback inline, the retention framing, and the agreed offer prompt.
func TestAskOfferViewRendersAgreedCopy(t *testing.T) {
	m := offerModel()
	view := m.View()
	if !strings.Contains(view, "✓ Correct.") {
		t.Fatalf("expected inline feedback verdict, got %q", view)
	}
	if !strings.Contains(view, "Knowledge retention bonus") {
		t.Fatalf("expected the retention framing, got %q", view)
	}
	if !strings.Contains(view, bonusOfferPrompt) {
		t.Fatalf("expected the agreed offer copy, got %q", view)
	}
	if !strings.Contains(view, "circuit breaker pattern") {
		t.Fatalf("expected the offer prose, got %q", view)
	}
}

// TestAskOfferAwaitingShowsThinking verifies the synthesis wait renders the
// thinking animation instead of the prompt.
func TestAskOfferAwaitingShowsThinking(t *testing.T) {
	m := offerModel()
	m.awaitingBonus = true
	m.frame = 1

	view := m.View()
	if strings.Contains(view, bonusOfferPrompt) {
		t.Fatalf("expected the prompt hidden while synthesizing, got %q", view)
	}
	if !strings.Contains(view, "Thinking..") {
		t.Fatalf("expected the thinking animation, got %q", view)
	}
}

// TestProbeOfferCmdParsesOffer verifies the probe contract: 200 offer payload
// → bonusOfferMsg; 204 / error / malformed → noOfferMsg.
func TestProbeOfferCmdParsesOffer(t *testing.T) {
	t.Run("200 offer", func(t *testing.T) {
		srv, _ := captureRequestServer(t, http.StatusOK, `{"slug":"circuit_breaker","prose":"breaker prose"}`)
		withDaemonBaseURL(t, srv.URL)
		msg := probeOfferCmd(&http.Client{Timeout: 3 * time.Second})()
		offer, ok := msg.(bonusOfferMsg)
		if !ok {
			t.Fatalf("expected bonusOfferMsg, got %T", msg)
		}
		if offer.slug != "circuit_breaker" || offer.prose != "breaker prose" {
			t.Fatalf("unexpected offer: %+v", offer)
		}
	})
	t.Run("204 no offer", func(t *testing.T) {
		srv, _ := captureRequestServer(t, http.StatusNoContent, "")
		withDaemonBaseURL(t, srv.URL)
		if _, ok := probeOfferCmd(&http.Client{Timeout: 3 * time.Second})().(noOfferMsg); !ok {
			t.Fatal("expected noOfferMsg on 204")
		}
	})
	t.Run("malformed body", func(t *testing.T) {
		srv, _ := captureRequestServer(t, http.StatusOK, `not json`)
		withDaemonBaseURL(t, srv.URL)
		if _, ok := probeOfferCmd(&http.Client{Timeout: 3 * time.Second})().(noOfferMsg); !ok {
			t.Fatal("expected noOfferMsg on malformed body")
		}
	})
	t.Run("unreachable daemon", func(t *testing.T) {
		withDaemonBaseURL(t, "http://localhost:1")
		if _, ok := probeOfferCmd(&http.Client{Timeout: 3 * time.Second})().(noOfferMsg); !ok {
			t.Fatal("expected noOfferMsg when the daemon is unreachable")
		}
	})
}

// TestAcceptOfferCmdParsesBonusQuestion verifies the accept contract: 200
// question payload → questionMsg; anything else → acceptFailedMsg.
func TestAcceptOfferCmdParsesBonusQuestion(t *testing.T) {
	t.Run("200 question", func(t *testing.T) {
		var capturedBody string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			b, _ := io.ReadAll(r.Body)
			capturedBody = string(b)
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"id":9,"question_type":"multiple_choice","question":"Why?","choices":["a","b"]}`))
		}))
		t.Cleanup(srv.Close)
		withDaemonBaseURL(t, srv.URL)

		msg := acceptOfferCmd("circuit_breaker", "/repo/test", "main", &http.Client{Timeout: 3 * time.Second})()
		q, ok := msg.(questionMsg)
		if !ok {
			t.Fatalf("expected questionMsg, got %T", msg)
		}
		if q.id != 9 || q.question != "Why?" || len(q.choices) != 2 {
			t.Fatalf("unexpected bonus question: %+v", q)
		}
		for _, want := range []string{"circuit_breaker", "/repo/test", "main"} {
			if !strings.Contains(capturedBody, want) {
				t.Fatalf("expected the accept POST body to carry %q, got %q", want, capturedBody)
			}
		}
	})
	t.Run("204 no question", func(t *testing.T) {
		srv, _ := captureRequestServer(t, http.StatusNoContent, "")
		withDaemonBaseURL(t, srv.URL)
		if _, ok := acceptOfferCmd("x", "/repo/test", "main", &http.Client{Timeout: 3 * time.Second})().(acceptFailedMsg); !ok {
			t.Fatal("expected acceptFailedMsg on 204")
		}
	})
}

// TestAskUnrecordedAnswerEndsWithoutProbe verifies the answer-not-recorded
// path (empty correctText) ends the session immediately — no probe fires when
// the daemon never recorded the answer.
func TestAskUnrecordedAnswerEndsWithoutProbe(t *testing.T) {
	m := newAskModel(10*time.Second, "", "main")
	m.state = stateFeedback

	final, cmd := m.updateFeedback(feedbackMsg{})
	got := final.(askModel)
	if got.state != stateDone {
		t.Fatalf("expected stateDone, got %v", got.state)
	}
	if !isQuitCmd(cmd) {
		t.Fatal("expected tea.Quit cmd")
	}
	if got.advisory != "[total-recall] Could not reach daemon — answer not recorded." {
		t.Fatalf("expected the advisory, got %q", got.advisory)
	}
	if got.offerProbed {
		t.Fatal("expected no offer probe after an unrecorded answer")
	}
}

// TestRenderPostAltScreenNoOfferByteIdentical pins the no-offer path: with no
// offer shown, the post-exit output is exactly today's rendering.
func TestRenderPostAltScreenNoOfferByteIdentical(t *testing.T) {
	correctOut := renderPostAltScreen(askModel{
		feedbackResult: feedbackMsg{correct: true, correctText: "X", feedback: "Great work!"},
	})
	want := "✓ Correct.\n\n  Great work!\n"
	if correctOut != want {
		t.Fatalf("no-offer output changed: got %q want %q", correctOut, want)
	}

	skipOut := renderPostAltScreen(askModel{skipped: true})
	if skipOut != "→ Question saved for later.\n" {
		t.Fatalf("no-offer skip output changed: got %q", skipOut)
	}

	// With the offer shown but no bonus answer, the inline feedback is not
	// repeated post-exit.
	suppressed := renderPostAltScreen(askModel{
		offerShown:     true,
		feedbackResult: feedbackMsg{correct: true, correctText: "X", feedback: "Great work!"},
	})
	if suppressed != "" {
		t.Fatalf("expected suppression after the inline render, got %q", suppressed)
	}

	// With the bonus answered, the bonus feedback prints as any other.
	bonus := renderPostAltScreen(askModel{
		offerShown:     true,
		bonusAnswered:  true,
		feedbackResult: feedbackMsg{correct: false, correctText: "a", feedback: "not quite"},
	})
	if !strings.Contains(bonus, "✗ The answer was: a") {
		t.Fatalf("expected the bonus verdict post-exit, got %q", bonus)
	}
}
