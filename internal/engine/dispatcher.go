package engine

import "github.com/colinwilliams91/total-recall/internal/recall"

// Dispatcher delivers a synthesized recall question to the developer.
// v1 ships with a terminal adapter only. Out-of-band delivery (VS Code
// notifications, polling endpoint) is planned for Phase 4.
type Dispatcher interface {
	Dispatch(q recall.Question) error
}
