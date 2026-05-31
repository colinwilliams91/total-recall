package config

const (
	sourceUser    = "[user]"
	sourceRepo    = "[repo]"
	sourceDefault = "[default]"
)

// Merge deep-merges user defaults with per-repo overrides.
//
// Rules:
//   - Per-repo values win over user defaults on any conflicting key.
//   - recall is deep-merged at the field level: a repo override of recall.max_questions
//     does not discard the user's recall.difficulty.
//   - privacy.* and ai.* in .tr.yaml are silently discarded (warnings are emitted
//     earlier by LoadRepoConfig via warnRepoConfigSecrets).
//   - A nil repo means no .tr.yaml was found; hook/mode/presentation sources are "default".
func Merge(user *UserConfig, repo *RepoConfig) *Config {
	cfg := &Config{
		Privacy: user.Privacy,
		AI:      user.AI,
		Recall:  user.Recall,
		Sources: ConfigSources{
			PrivacyConversationAnalysis: sourceUser,
			AIProvider:                  sourceUser,
			AIModel:                     sourceUser,
			AIAPIKey:                    sourceUser,
			AIBaseURL:                   sourceUser,
			RecallDifficulty:            sourceUser,
			RecallMaxQuestions:          sourceUser,
			HooksPreCommit:              sourceDefault,
			HooksCommitMsg:              sourceDefault,
			HooksPrePush:                sourceDefault,
			ModeBlocking:                sourceDefault,
			PresentationTerminal:        sourceDefault,
			PresentationMCP:             sourceDefault,
		},
	}

	if repo == nil {
		return cfg
	}

	// Privacy and AI keys in .tr.yaml are warned about (and security-graded if a
	// raw api-key is present) in LoadRepoConfig via warnRepoConfigSecrets.
	// Silently discard them here — they are never merged into the resolved config.

	// Per-repo overrides for project-specific blocks.
	cfg.Hooks = repo.Hooks
	cfg.Sources.HooksPreCommit = sourceRepo
	cfg.Sources.HooksCommitMsg = sourceRepo
	cfg.Sources.HooksPrePush = sourceRepo

	cfg.Mode = repo.Mode
	cfg.Sources.ModeBlocking = sourceRepo

	cfg.Presentation = repo.Presentation
	cfg.Sources.PresentationTerminal = sourceRepo
	cfg.Sources.PresentationMCP = sourceRepo

	// Deep merge recall: only override the keys the repo explicitly sets.
	if repo.Recall != nil {
		if repo.Recall.Difficulty != "" {
			cfg.Recall.Difficulty = repo.Recall.Difficulty
			cfg.Sources.RecallDifficulty = sourceRepo
		}
		if repo.Recall.MaxQuestions != 0 {
			cfg.Recall.MaxQuestions = repo.Recall.MaxQuestions
			cfg.Sources.RecallMaxQuestions = sourceRepo
		}
	}

	return cfg
}
