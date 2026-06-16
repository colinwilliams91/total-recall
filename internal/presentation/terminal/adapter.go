package terminal

import (
	"log"

	"github.com/colinwilliams91/total-recall/internal/recall"
)

// Adapter implements engine.Dispatcher by logging recall delivery events.
//
// The actual interactive question is delivered through tr ask via /recall/next.
// The daemon terminal only gets a compact operational log so pipeline activity
// remains visible without duplicating the recall card.
type Adapter struct{}

// New returns a new terminal Adapter.
func New() *Adapter { return &Adapter{} }

// Dispatch logs that a recall question was queued for terminal delivery.
func (a *Adapter) Dispatch(q recall.Question) error {
	log.Printf("[recall] question queued for terminal delivery choices=%d", len(q.Choices))
	return nil
}
