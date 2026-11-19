package autostart

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLaunchdPlistTemplateRunsAtLoadWithoutKeepAlive(t *testing.T) {
	content := launchdContent("/usr/local/bin/torec")
	if !strings.Contains(content, launchdLabel) {
		t.Fatalf("expected launchd label in plist, got:\n%s", content)
	}
	if !strings.Contains(content, "<string>/usr/local/bin/torec</string>") {
		t.Fatalf("expected baked binary path in plist, got:\n%s", content)
	}
	if !strings.Contains(content, "<string>serve</string>") {
		t.Fatalf("expected serve argument in plist, got:\n%s", content)
	}
	if !strings.Contains(content, "<key>RunAtLoad</key>") {
		t.Fatalf("expected RunAtLoad in plist, got:\n%s", content)
	}
	// KeepAlive would relaunch the daemon whenever it exits, fighting
	// `torec stop` — autostart must not supervise the daemon's lifecycle.
	if strings.Contains(content, "KeepAlive") {
		t.Fatalf("unexpected KeepAlive in plist (must not fight 'torec stop'):\n%s", content)
	}
}

func TestSystemdUnitTemplateWantedByDefaultTarget(t *testing.T) {
	content := linuxContent("/usr/local/bin/torec")
	if !strings.Contains(content, `ExecStart="/usr/local/bin/torec" serve`) {
		t.Fatalf("expected quoted ExecStart with baked path, got:\n%s", content)
	}
	if !strings.Contains(content, "WantedBy=default.target") {
		t.Fatalf("expected WantedBy=default.target, got:\n%s", content)
	}
	if !strings.Contains(content, "Restart=no") {
		t.Fatalf("expected Restart=no (stop must stay effective), got:\n%s", content)
	}
}

func TestStartupCmdTemplateQuotesBinaryPath(t *testing.T) {
	content := windowsContent(`C:\Users\colin\go\bin\torec.exe`)
	if !strings.Contains(content, `"C:\Users\colin\go\bin\torec.exe" serve`) {
		t.Fatalf("expected quoted baked path with serve, got:\n%s", content)
	}
	if !strings.Contains(content, "Managed by Total Recall") {
		t.Fatalf("expected managed-by marker in cmd, got:\n%s", content)
	}
}

func TestLaunchdPathUnderUserLaunchAgents(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	path, err := launchdPath()
	if err != nil {
		t.Fatalf("launchdPath: %v", err)
	}
	want := filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist")
	if path != want {
		t.Fatalf("expected %q, got %q", want, path)
	}
}

func TestLinuxPathUnderSystemdUserDir(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	path, err := linuxPath()
	if err != nil {
		t.Fatalf("linuxPath: %v", err)
	}
	want := filepath.Join(home, ".config", "systemd", "user", systemdUnitName)
	if path != want {
		t.Fatalf("expected %q, got %q", want, path)
	}
}

func TestWindowsPathUnderStartupFolder(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)
	// os.UserConfigDir reads %AppData% on Windows and XDG_CONFIG_HOME (falling
	// back to ~/.config) elsewhere; point both at the same isolated root so
	// the expectation holds on any platform.
	configRoot := filepath.Join(home, "AppData", "Roaming")
	t.Setenv("XDG_CONFIG_HOME", configRoot)
	t.Setenv("AppData", configRoot)

	path, err := windowsPath()
	if err != nil {
		t.Fatalf("windowsPath: %v", err)
	}
	want := filepath.Join(filepath.FromSlash(home+"/AppData/Roaming"),
		"Microsoft", "Windows", "Start Menu", "Programs", "Startup", "torec-autostart.cmd")
	if path != want {
		t.Fatalf("expected %q, got %q", want, path)
	}
}

// TestInstallIsIdempotent re-running the install must not duplicate the
// entry: the same canonical file is overwritten in place, and an unchanged
// render is left untouched (no rewrite churn).
func TestInstallIsIdempotent(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	selfPath := filepath.Join(home, "bin", "torec")

	first, err := Install(selfPath)
	if err != nil {
		t.Fatalf("first Install: %v", err)
	}
	data1, err := os.ReadFile(first)
	if err != nil {
		t.Fatalf("read after first install: %v", err)
	}
	stat1, err := os.Stat(first)
	if err != nil {
		t.Fatalf("stat after first install: %v", err)
	}

	second, err := Install(selfPath)
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if second != first {
		t.Fatalf("expected same path across installs, got %q then %q", first, second)
	}
	data2, err := os.ReadFile(second)
	if err != nil {
		t.Fatalf("read after second install: %v", err)
	}
	if string(data1) != string(data2) {
		t.Fatalf("content changed across installs:\nfirst:\n%s\nsecond:\n%s", data1, data2)
	}
	if stat2, err := os.Stat(second); err != nil {
		t.Fatalf("stat after second install: %v", err)
	} else if !stat2.ModTime().Equal(stat1.ModTime()) {
		// An unchanged render must not be rewritten: Install skips the write
		// when the file already matches, keeping re-runs side-effect free.
		t.Fatalf("file was rewritten on idempotent re-install (mtime changed)")
	}

	// Exactly one autostart entry — no numbered/suffixed duplicates.
	dir := filepath.Dir(first)
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("read dir: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("expected exactly 1 file in %s, got %d entries", dir, len(entries))
	}
}

// TestInstallUpdatesStalePath exercises the overwrite branch: a previously
// installed file pointing at a moved binary is refreshed on re-install.
func TestInstallUpdatesStalePath(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)
	t.Setenv("USERPROFILE", home)

	oldPath, err := Install(filepath.Join(home, "old", "torec"))
	if err != nil {
		t.Fatalf("first Install: %v", err)
	}
	newPath, err := Install(filepath.Join(home, "new", "torec"))
	if err != nil {
		t.Fatalf("second Install: %v", err)
	}
	if oldPath != newPath {
		t.Fatalf("expected stable path, got %q then %q", oldPath, newPath)
	}
	content, err := os.ReadFile(newPath)
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	if !strings.Contains(string(content), filepath.Join(home, "new", "torec")) {
		t.Fatalf("expected refreshed binary path, got:\n%s", content)
	}
}
