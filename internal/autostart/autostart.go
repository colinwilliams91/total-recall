// Package autostart configures the platform's login hook that starts the
// daemon without user action: a launchd agent on macOS, a systemd user unit
// on Linux, and a Startup-folder script on Windows. Autostart only launches
// `torec serve` at login; the daemon's own lifecycle (pidfile, `torec stop`)
// stays authoritative afterwards, so no restart-on-exit supervision is
// configured — a stopped daemon stays stopped until the next login.
package autostart

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

// launchdLabel names the agent inside the plist so launchd treats re-installs
// as an update of the same job rather than a new one.
const launchdLabel = "com.colinwilliams91.total-recall"

// systemdUnitName is the canonical unit filename; re-running init overwrites
// the same unit instead of minting a duplicate.
const systemdUnitName = "torec.service"

const launchdPlistTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>%s</string>
    <key>ProgramArguments</key>
    <array>
        <string>%s</string>
        <string>serve</string>
    </array>
    <key>RunAtLoad</key>
    <true/>
</dict>
</plist>
`

const systemdUnitTemplate = `[Unit]
Description=Total Recall daemon (torec serve)

[Service]
Type=simple
ExecStart="%s" serve
Restart=no

[Install]
WantedBy=default.target
`

const startupCmdTemplate = "@echo off\r\n" +
	":: Managed by Total Recall — re-run 'torec init' to update.\r\n" +
	"\"%s\" serve\r\n"

// SelfPath resolves the absolute, symlink-followed path of the running
// binary. Autostart entries bake this path in so the login hook execs the
// same installed binary the user just configured; the symlink resolution
// matters because package managers (and go install) commonly expose torec
// through a symlink.
func SelfPath() (string, error) {
	self, err := os.Executable()
	if err != nil {
		return "", err
	}
	if resolved, err := filepath.EvalSymlinks(self); err == nil {
		return resolved, nil
	}
	return self, nil
}

// FilePath returns the platform's autostart destination: the launchd agent
// plist, the systemd user unit, or the Windows Startup-folder script.
func FilePath() (string, error) {
	switch runtime.GOOS {
	case "darwin":
		return launchdPath()
	case "windows":
		return windowsPath()
	default:
		return linuxPath()
	}
}

// Content renders the platform's autostart file with the binary path baked in.
func Content(selfPath string) string {
	switch runtime.GOOS {
	case "darwin":
		return launchdContent(selfPath)
	case "windows":
		return windowsContent(selfPath)
	default:
		return linuxContent(selfPath)
	}
}

// launchdContent renders the macOS agent plist. RunAtLoad only — no
// KeepAlive, which would relaunch the daemon whenever it exits and fight
// `torec stop`: autostart starts the daemon, but the daemon's own lifecycle
// commands stay authoritative afterwards.
func launchdContent(selfPath string) string {
	return fmt.Sprintf(launchdPlistTemplate, launchdLabel, selfPath)
}

// linuxContent renders the systemd user unit. Restart=no keeps `torec stop`
// effective between logins.
func linuxContent(selfPath string) string {
	return fmt.Sprintf(systemdUnitTemplate, selfPath)
}

// windowsContent renders the Startup-folder batch script, which Windows
// executes at logon.
func windowsContent(selfPath string) string {
	return fmt.Sprintf(startupCmdTemplate, selfPath)
}

// Install writes the platform autostart file, overwriting in place so
// re-running init refreshes the entry instead of duplicating it. An unchanged
// file is left untouched (no rewrite) to keep the operation side-effect free.
// Returns the path written (or already present).
func Install(selfPath string) (string, error) {
	path, err := FilePath()
	if err != nil {
		return "", err
	}
	content := Content(selfPath)
	if existing, readErr := os.ReadFile(path); readErr == nil && string(existing) == content {
		return path, nil
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// Enable best-effort registers the written file with the platform service
// manager: on Linux the unit must be enabled (symlinked into default.target)
// via systemctl --user before login picks it up; on macOS the plist is
// loaded by launchd at next login, and the Windows Startup folder is scanned
// at logon, so neither needs a registration step. Errors surface to the
// caller as an advisory — a failed enable must not abort init, since the
// file itself is already in place and can be enabled manually.
func Enable() error {
	if runtime.GOOS != "linux" {
		return nil
	}
	if _, err := exec.LookPath("systemctl"); err != nil {
		return fmt.Errorf("systemctl not found — run 'systemctl --user enable %s' manually", systemdUnitName)
	}
	if out, err := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl --user daemon-reload: %v (%s)", err, strings.TrimSpace(string(out)))
	}
	if out, err := exec.Command("systemctl", "--user", "enable", systemdUnitName).CombinedOutput(); err != nil {
		return fmt.Errorf("systemctl --user enable %s: %v (%s)", systemdUnitName, err, strings.TrimSpace(string(out)))
	}
	return nil
}

// launchdPath places the agent in the user LaunchAgents directory.
func launchdPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, "Library", "LaunchAgents", launchdLabel+".plist"), nil
}

// linuxPath places the unit in the systemd user directory (~/.config, not
// XDG-resolved, to match where `systemctl --user` expects user units by
// default even when XDG_CONFIG_HOME is customized mid-session).
func linuxPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "systemd", "user", systemdUnitName), nil
}

// windowsPath places the script in the per-user Startup folder, which
// Windows runs at logon — the simplest robust autostart that needs no
// admin rights and no Task Scheduler registration.
func windowsPath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "Microsoft", "Windows", "Start Menu", "Programs", "Startup", "torec-autostart.cmd"), nil
}
