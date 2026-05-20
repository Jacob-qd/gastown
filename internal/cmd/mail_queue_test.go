package cmd

import (
	"testing"
	"time"

	"github.com/steveyegge/gastown/internal/beads"
)

func TestQueueCapacityHelpers(t *testing.T) {
	claimedAt := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	messages := []queueMessage{
		{ID: "hq-a", ClaimedBy: "gastown/polecats/a", ClaimedAt: &claimedAt},
		{ID: "hq-b"},
		{ID: "hq-c", ClaimedBy: "gastown/polecats/c"},
	}

	if got := countProcessingQueueMessages(messages); got != 2 {
		t.Fatalf("countProcessingQueueMessages() = %d, want 2", got)
	}

	fields := &beads.QueueFields{MaxConcurrency: 2}
	if !isQueueAtCapacity(fields, messages) {
		t.Fatal("expected queue to be at capacity")
	}

	fields.MaxConcurrency = 3
	if isQueueAtCapacity(fields, messages) {
		t.Fatal("expected queue below capacity")
	}

	unclaimed := filterUnclaimedQueueMessages(messages)
	if len(unclaimed) != 1 || unclaimed[0].ID != "hq-b" {
		t.Fatalf("filterUnclaimedQueueMessages() = %#v, want only hq-b", unclaimed)
	}
}

func TestClaimWithinQueueCapacity(t *testing.T) {
	first := time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)
	second := first.Add(time.Second)
	messages := []queueMessage{
		{ID: "hq-late", ClaimedBy: "gastown/polecats/late", ClaimedAt: &second},
		{ID: "hq-early", ClaimedBy: "gastown/polecats/early", ClaimedAt: &first},
		{ID: "hq-open"},
	}

	if !claimWithinQueueCapacity(messages, "hq-early", 1) {
		t.Fatal("oldest claim should be within capacity")
	}
	if claimWithinQueueCapacity(messages, "hq-late", 1) {
		t.Fatal("later claim should exceed capacity")
	}
	if !claimWithinQueueCapacity(messages, "hq-late", 0) {
		t.Fatal("zero max concurrency should be unlimited")
	}
}

func TestFormatMaxConcurrency(t *testing.T) {
	if got := formatMaxConcurrency(0); got != "unlimited" {
		t.Fatalf("formatMaxConcurrency(0) = %q, want unlimited", got)
	}
	if got := formatMaxConcurrency(4); got != "4" {
		t.Fatalf("formatMaxConcurrency(4) = %q, want 4", got)
	}
}
