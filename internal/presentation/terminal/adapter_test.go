package terminal

import (
	"bytes"
	"log"
	"strings"
	"testing"

	"github.com/colinwilliams91/total-recall/internal/recall"
)

func TestDispatchLogsQueueEventWithoutQuestionBody(t *testing.T) {
	var buf bytes.Buffer
	prevWriter := log.Writer()
	prevFlags := log.Flags()
	log.SetOutput(&buf)
	log.SetFlags(0)
	defer log.SetOutput(prevWriter)
	defer log.SetFlags(prevFlags)

	adapter := New()
	err := adapter.Dispatch(recall.Question{
		Question: "What does @@ -a,b +c,d @@ mean?",
		Choices:  []string{"a", "b", "c", "d"},
	})
	if err != nil {
		t.Fatalf("dispatch failed: %v", err)
	}

	got := buf.String()
	if !strings.Contains(got, "[recall] question queued for terminal delivery choices=4") {
		t.Fatalf("expected recall queue log, got %q", got)
	}
	if strings.Contains(got, "What does @@ -a,b +c,d @@ mean?") {
		t.Fatalf("expected question body to stay out of daemon logs, got %q", got)
	}
	if strings.Contains(got, "1. a") {
		t.Fatalf("expected choices to stay out of daemon logs, got %q", got)
	}
}