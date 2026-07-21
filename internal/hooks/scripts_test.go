package hooks

import "strings"

import "testing"

// TestCommitMsgBodyIsNoOp guards against the duplicate-advisory regression
// fixed in #14: pre-commit and commit-msg both fired on `git commit`, and
// each printed a "Daemon not running" advisory when the daemon was down.
// commit-msg is intentionally a no-op; pre-commit is the only dispatch point.
func TestCommitMsgBodyIsNoOp(t *testing.T) {
	if strings.Contains(commitMsgBody, "curl") {
		t.Fatalf("commitMsgBody must not call curl (would re-introduce duplicate advisory):\n%s", commitMsgBody)
	}
	if strings.Contains(commitMsgBody, "Daemon not running") {
		t.Fatalf("commitMsgBody must not print the Daemon-not-running advisory:\n%s", commitMsgBody)
	}
	if !strings.Contains(commitMsgBody, "exit 0") {
		t.Fatalf("commitMsgBody should exit cleanly, got:\n%s", commitMsgBody)
	}
}

func TestCommitMsgBatIsNoOp(t *testing.T) {
	if strings.Contains(commitMsgBat, "Invoke-WebRequest") {
		t.Fatalf("commitMsgBat must not call Invoke-WebRequest (would re-introduce duplicate advisory):\n%s", commitMsgBat)
	}
	if strings.Contains(commitMsgBat, "Daemon not running") {
		t.Fatalf("commitMsgBat must not print the Daemon-not-running advisory:\n%s", commitMsgBat)
	}
	if !strings.Contains(commitMsgBat, "exit /b 0") {
		t.Fatalf("commitMsgBat should exit cleanly, got:\n%s", commitMsgBat)
	}
}

// TestPreCommitAdvisoryPreserved is a guard on the *other* half of #14:
// the pre-commit hook is the only place the advisory should fire on commit.
func TestPreCommitAdvisoryPreserved(t *testing.T) {
	if !strings.Contains(preCommitBody, "curl") {
		t.Fatal("preCommitBody should still POST to the daemon")
	}
	if !strings.Contains(preCommitBody, "Daemon not running") {
		t.Fatal("preCommitBody should still print the advisory on daemon failure")
	}
}
