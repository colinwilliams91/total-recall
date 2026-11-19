package hooks

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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

// TestRepoHookDocCopiesMirrorGenerated guards the repo-root hooks/ script
// pairs. They are documentation copies of the content `torec repo` installs
// (internal/hooks/scripts.go consts) and must stay byte-identical; the #14
// fix drifted them (the generated commit-msg hook became a no-op while the
// doc copies still dispatched and printed the duplicate advisory).
func TestRepoHookDocCopiesMirrorGenerated(t *testing.T) {
	pairs := []struct {
		repoCopy  string
		generated string
	}{
		{"pre-commit.sh", buildShScript("pre-commit", "")},
		{"pre-commit.bat", preCommitBat},
		{"commit-msg.sh", buildShScript("commit-msg", "")},
		{"commit-msg.bat", commitMsgBat},
		{"pre-push.sh", buildShScript("pre-push", "")},
		{"pre-push.bat", prePushBat},
	}
	for _, p := range pairs {
		data, err := os.ReadFile(filepath.Join("..", "..", "hooks", p.repoCopy))
		if err != nil {
			t.Fatalf("reading hooks/%s: %v", p.repoCopy, err)
		}
		if string(data) != p.generated {
			t.Errorf("hooks/%s drifted from generated content — regenerate the doc copy to mirror it", p.repoCopy)
		}
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
