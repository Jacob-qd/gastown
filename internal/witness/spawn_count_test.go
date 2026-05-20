package witness

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/steveyegge/gastown/internal/config"
)

func TestRecordBeadRespawn_Increments(t *testing.T) {
	tmpDir := t.TempDir()
	// Create the witness subdirectory so the state file path is valid.
	if err := os.MkdirAll(filepath.Join(tmpDir, "witness"), 0755); err != nil {
		t.Fatal(err)
	}

	count := RecordBeadRespawn(tmpDir, "bead-1")
	if count != 1 {
		t.Errorf("first RecordBeadRespawn = %d, want 1", count)
	}

	count = RecordBeadRespawn(tmpDir, "bead-1")
	if count != 2 {
		t.Errorf("second RecordBeadRespawn = %d, want 2", count)
	}
}

func TestShouldBlockRespawn_Threshold(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "witness"), 0755); err != nil {
		t.Fatal(err)
	}

	// Below threshold.
	for i := 0; i < config.DefaultWitnessMaxBeadRespawns-1; i++ {
		RecordBeadRespawn(tmpDir, "bead-2")
	}
	if ShouldBlockRespawn(tmpDir, "bead-2") {
		t.Error("ShouldBlockRespawn = true before reaching threshold")
	}

	// At threshold.
	RecordBeadRespawn(tmpDir, "bead-2")
	if !ShouldBlockRespawn(tmpDir, "bead-2") {
		t.Error("ShouldBlockRespawn = false at threshold")
	}
}

func TestResetBeadRespawnCount(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "witness"), 0755); err != nil {
		t.Fatal(err)
	}

	RecordBeadRespawn(tmpDir, "bead-3")
	RecordBeadRespawn(tmpDir, "bead-3")

	if err := ResetBeadRespawnCount(tmpDir, "bead-3"); err != nil {
		t.Fatalf("ResetBeadRespawnCount error: %v", err)
	}

	if ShouldBlockRespawn(tmpDir, "bead-3") {
		t.Error("ShouldBlockRespawn = true after reset")
	}

	// Re-increment should start from 1.
	count := RecordBeadRespawn(tmpDir, "bead-3")
	if count != 1 {
		t.Errorf("RecordBeadRespawn after reset = %d, want 1", count)
	}
}

func TestRecordBeadRespawn_ConcurrentSafe(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "witness"), 0755); err != nil {
		t.Fatal(err)
	}

	const goroutines = 20
	var wg sync.WaitGroup
	wg.Add(goroutines)

	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			RecordBeadRespawn(tmpDir, "bead-race")
		}()
	}
	wg.Wait()

	// After all goroutines, the count must equal the number of increments.
	state := loadBeadRespawnState(tmpDir)
	rec, ok := state.Beads["bead-race"]
	if !ok {
		t.Fatal("bead-race record not found")
	}
	if rec.Count != goroutines {
		t.Errorf("concurrent count = %d, want %d", rec.Count, goroutines)
	}
}

func TestShouldBlockRespawn_UnknownBead(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "witness"), 0755); err != nil {
		t.Fatal(err)
	}

	if ShouldBlockRespawn(tmpDir, "nonexistent") {
		t.Error("ShouldBlockRespawn = true for unknown bead")
	}
}

func TestRecordBeadRespawnWithReason_Diagnostics(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "witness"), 0755); err != nil {
		t.Fatal(err)
	}

	RecordBeadRespawnWithReason(tmpDir, "bead-context", "gt sling dispatch attempt (Dolt preflight passed)")
	RecordBeadRespawnWithReason(tmpDir, "bead-context", "witness recovered abandoned bead from rig/polecat (previous status: hooked)")

	diagnostics := DescribeBeadRespawnHistory(tmpDir, "bead-context")
	for _, want := range []string{
		"Recent respawn attempts for bead-context (total: 2)",
		"#1 at ",
		"gt sling dispatch attempt (Dolt preflight passed)",
		"#2 at ",
		"witness recovered abandoned bead from rig/polecat (previous status: hooked)",
	} {
		if !strings.Contains(diagnostics, want) {
			t.Fatalf("DescribeBeadRespawnHistory missing %q in:\n%s", want, diagnostics)
		}
	}
}

func TestRecordBeadRespawnWithReason_BoundsRecentAttempts(t *testing.T) {
	tmpDir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(tmpDir, "witness"), 0755); err != nil {
		t.Fatal(err)
	}

	for i := 0; i < maxRecentRespawnAttempts+2; i++ {
		RecordBeadRespawnWithReason(tmpDir, "bead-bounded", "attempt")
	}

	state := loadBeadRespawnState(tmpDir)
	rec := state.Beads["bead-bounded"]
	if rec.Count != maxRecentRespawnAttempts+2 {
		t.Fatalf("Count = %d, want %d", rec.Count, maxRecentRespawnAttempts+2)
	}
	if len(rec.RecentAttempts) != maxRecentRespawnAttempts {
		t.Fatalf("len(RecentAttempts) = %d, want %d", len(rec.RecentAttempts), maxRecentRespawnAttempts)
	}
	if rec.RecentAttempts[0].Count != 3 {
		t.Fatalf("oldest retained attempt count = %d, want 3", rec.RecentAttempts[0].Count)
	}
}
