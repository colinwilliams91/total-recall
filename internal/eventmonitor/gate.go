package eventmonitor

import "github.com/colinwilliams91/total-recall/internal/config"

// ConversationGate reports whether Signal 1 (user questions) and Signal 2 (agent explanations)
// from MCP conversation analysis are enabled for the resolved config.
//
// Both signals are gated on privacy.conversation_analysis.
// Signal 3 (code context from filesystem/git watchers) is always-on and bypasses this gate.
func ConversationGate(cfg *config.Config) bool {
	return cfg.Privacy.ConversationAnalysis
}

// CodeContextGate reports whether Signal 3 (filesystem/git code context) is active.
// Code context is always enabled — it does not capture conversational content.
func CodeContextGate(_ *config.Config) bool {
	return true
}
