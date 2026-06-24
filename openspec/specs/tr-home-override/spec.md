## Requirements

### Requirement: TR_HOME environment variable redirects the Total Recall data directory
When the `TR_HOME` environment variable is set to a non-empty filesystem path, all Total Recall subsystems SHALL resolve their data directory to `$TR_HOME` instead of `~/.tr`. This includes: the SQLite memory store (`memory.db`), the user config file (`config.yaml`), and any other state written under the data directory. When `TR_HOME` is unset or empty, behavior SHALL be unchanged (`~/.tr` via `os.UserHomeDir()`).

The `TR_HOME` directory SHALL be created with mode 0700 if it does not exist, mirroring the existing `~/.tr` creation behavior.

#### Scenario: Daemon started with TR_HOME set
- **WHEN** `TR_HOME=/tmp/tr-isolation` is set and `total-recall serve` is started
- **THEN** the daemon opens `/tmp/tr-isolation/memory.db` for the store and reads `/tmp/tr-isolation/config.yaml` for config; `~/.tr/` is not touched

#### Scenario: TR_HOME unset — default behavior
- **WHEN** `TR_HOME` is not set and `total-recall serve` is started
- **THEN** the daemon uses `~/.tr/memory.db` and `~/.tr/config.yaml` as before

#### Scenario: Test isolation via TR_HOME
- **WHEN** a Go test sets `t.Setenv("TR_HOME", t.TempDir())` and calls `cache.Open()`
- **THEN** the store opens `<tempdir>/memory.db` and the test never reads or writes the real `~/.tr/`

---

### Requirement: TR_HOME is honored by both cache and config path resolution
`cache.trDir()` and `config.UserConfigPath()` / `config.UserConfigDir()` SHALL each check `TR_HOME` first and return `$TR_HOME` when set, falling back to `~/.tr` when unset. A single `TR_HOME` value SHALL redirect both config and memory.db to the same directory, preserving the co-located layout.

#### Scenario: Config and cache agree on TR_HOME
- **WHEN** `TR_HOME=/tmp/tr-x` is set
- **THEN** `config.UserConfigPath()` returns `/tmp/tr-x/config.yaml` and `cache.trDir()` returns `/tmp/tr-x`, so config and memory.db live in the same directory
