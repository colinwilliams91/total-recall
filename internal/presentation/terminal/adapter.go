package terminal

import (
	"fmt"

	"github.com/colinwilliams91/total-recall/internal/recall"
)

// Adapter implements engine.Dispatcher by printing recall questions to stdout.
//
// NOTE (v1 limitation): This writes to the daemon's stdout, not to the
// committing developer's terminal. Phase 4 will replace this with true
// out-of-band delivery (VS Code extension notifications API, or a
// /recall/next polling endpoint) so the question surfaces in the right place.
type Adapter struct{}

// New returns a new terminal Adapter.
func New() *Adapter { return &Adapter{} }

// Dispatch prints the recall question and choices to stdout in a styled format.
func (a *Adapter) Dispatch(q recall.Question) error {
	fmt.Println()
	fmt.Println("🧠 Recall Check")
	fmt.Println("─────────────────────────────────────────")
	fmt.Printf("  %s\n\n", q.Question)
	for i, choice := range q.Choices {
		fmt.Printf("  %d. %s\n", i+1, choice)
	}
	fmt.Println("─────────────────────────────────────────")
	fmt.Println()
	return nil
}
