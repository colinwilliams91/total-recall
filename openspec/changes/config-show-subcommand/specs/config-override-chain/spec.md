## ADDED Requirements

### Requirement: `tr config show` subcommand renders the resolved config; `--show` is a deprecated hidden alias
The `tr config` command SHALL expose a `show` subcommand that renders the fully resolved, deep-merged config (with per-key source annotations and the `prompt assets:` section) — identical output to the current `--show` behavior. The `--show` flag SHALL continue to function as a deprecated alias: it remains functional, SHALL NOT appear in help/usage output, and when used SHALL print a one-line deprecation notice to stderr — `note: 'tr config --show' is deprecated — use 'tr config show'` — suppressed by the `--quiet` global flag. The bare `tr config` invocation SHALL continue to print the command help, now listing the `show` subcommand.

#### Scenario: Subcommand form prints the resolved config
- **WHEN** `tr config show` is invoked
- **THEN** the output equals the historical `--show` rendering — every resolved key annotated with its source (`[user]`/`[repo]`/`[default]`) plus the `prompt assets:` section

#### Scenario: Deprecated flag form still works
- **WHEN** `tr config --show` is invoked
- **THEN** the same resolved-config output prints, plus the one-line deprecation notice on stderr before it; exit 0

#### Scenario: Deprecation notice respects --quiet
- **WHEN** `tr config --show --quiet` is invoked
- **THEN** the resolved-config output prints and no deprecation notice appears

#### Scenario: Bare form prints help
- **WHEN** `tr config` is invoked with no arguments or flags
- **THEN** the command help prints, listing the `show` subcommand, and the previous behavior is unchanged

#### Scenario: Show subcommand does not accept stray args
- **WHEN** `tr config show extra-arg` is invoked
- **THEN** it fails with an argument error (Cobra `Args: cobra.NoArgs`) rather than silently ignoring the argument
