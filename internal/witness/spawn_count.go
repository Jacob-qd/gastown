package witness

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/steveyegge/gastown/internal/config"
	"github.com/steveyegge/gastown/internal/lock"
	"github.com/steveyegge/gastown/internal/workspace"
)

const maxRecentRespawnAttempts = 5

// respawnMu serializes in-process access to the respawn state file.
// Cross-process serialization is handled by lock.FlockAcquire on a
// sibling .flock file (see RecordBeadRespawn, ShouldBlockRespawn, etc.).
var respawnMu sync.Mutex

// beadRespawnRecord tracks how many times a single bead has been reset for re-dispatch.
type beadRespawnRecord struct {
	BeadID         string               `json:"bead_id"`
	Count          int                  `json:"count"`
	LastRespawn    time.Time            `json:"last_respawn"`
	RecentAttempts []beadRespawnAttempt `json:"recent_attempts,omitempty"`
}

type beadRespawnAttempt struct {
	Count       int       `json:"count"`
	RespawnedAt time.Time `json:"respawned_at"`
	Reason      string    `json:"reason,omitempty"`
}

// beadRespawnState holds respawn counts for all tracked beads.
type beadRespawnState struct {
	Beads       map[string]*beadRespawnRecord `json:"beads"`
	LastUpdated time.Time                     `json:"last_updated"`
}

func beadRespawnStateFile(townRoot string) string {
	return filepath.Join(townRoot, "witness", "bead-respawn-counts.json")
}

func respawnTownRoot(workDir string) string {
	townRoot, err := workspace.Find(workDir)
	if err != nil || townRoot == "" {
		return workDir
	}
	return townRoot
}

func loadBeadRespawnState(townRoot string) *beadRespawnState {
	data, err := os.ReadFile(beadRespawnStateFile(townRoot)) //nolint:gosec // G304: path from trusted townRoot
	if err != nil {
		return &beadRespawnState{Beads: make(map[string]*beadRespawnRecord)}
	}
	var state beadRespawnState
	if err := json.Unmarshal(data, &state); err != nil {
		return &beadRespawnState{Beads: make(map[string]*beadRespawnRecord)}
	}
	if state.Beads == nil {
		state.Beads = make(map[string]*beadRespawnRecord)
	}
	return &state
}

func saveBeadRespawnState(townRoot string, state *beadRespawnState) error {
	stateFile := beadRespawnStateFile(townRoot)
	if err := os.MkdirAll(filepath.Dir(stateFile), 0755); err != nil {
		return fmt.Errorf("creating witness dir: %w", err)
	}
	state.LastUpdated = time.Now().UTC()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling respawn state: %w", err)
	}
	return os.WriteFile(stateFile, data, 0600)
}

// ShouldBlockRespawn returns true if the bead has already been respawned
// MaxBeadRespawns times (from operational config). When true, the caller
// should escalate to mayor instead of sending RECOVERED_BEAD to deacon
// for re-dispatch. This is the primary circuit breaker for spawn storms
// (clown show #22).
func ShouldBlockRespawn(workDir, beadID string) bool {
	respawnMu.Lock()
	defer respawnMu.Unlock()

	townRoot := respawnTownRoot(workDir)
	maxRespawns := config.LoadOperationalConfig(townRoot).GetWitnessConfig().MaxBeadRespawnsV()

	// Cross-process flock to serialize with other witness instances.
	unlock, flockErr := lock.FlockAcquire(beadRespawnStateFile(townRoot) + ".flock")
	if flockErr == nil {
		defer unlock()
	}

	state := loadBeadRespawnState(townRoot)
	rec, ok := state.Beads[beadID]
	if !ok {
		return false
	}
	return rec.Count >= maxRespawns
}

// RecordBeadRespawn increments the respawn count for beadID and returns the new count.
// workDir is the rig path; townRoot is resolved internally via workspace.Find.
// On state file errors the count is still incremented in memory and returned, so the
// caller can log/warn without blocking the respawn itself.
//
// Serialized via respawnMu (in-process) and flock (cross-process) to prevent
// concurrent patrol cycles from racing on the load-modify-save cycle.
func RecordBeadRespawn(workDir, beadID string) int {
	return RecordBeadRespawnWithReason(workDir, beadID, "respawn recorded")
}

// RecordBeadRespawnWithReason increments the respawn count and records recent
// diagnostic context so respawn-limit errors explain what happened recently.
func RecordBeadRespawnWithReason(workDir, beadID, reason string) int {
	respawnMu.Lock()
	defer respawnMu.Unlock()

	townRoot := respawnTownRoot(workDir)

	// Cross-process flock to serialize with other witness instances.
	unlock, flockErr := lock.FlockAcquire(beadRespawnStateFile(townRoot) + ".flock")
	if flockErr == nil {
		defer unlock()
	}

	state := loadBeadRespawnState(townRoot)
	rec, ok := state.Beads[beadID]
	if !ok {
		rec = &beadRespawnRecord{BeadID: beadID}
		state.Beads[beadID] = rec
	}
	rec.Count++
	rec.LastRespawn = time.Now().UTC()
	rec.RecentAttempts = append(rec.RecentAttempts, beadRespawnAttempt{
		Count:       rec.Count,
		RespawnedAt: rec.LastRespawn,
		Reason:      strings.TrimSpace(reason),
	})
	if len(rec.RecentAttempts) > maxRecentRespawnAttempts {
		rec.RecentAttempts = rec.RecentAttempts[len(rec.RecentAttempts)-maxRecentRespawnAttempts:]
	}
	_ = saveBeadRespawnState(townRoot, state) // Non-fatal: tracking failure must not block respawn
	return rec.Count
}

// DescribeBeadRespawnHistory returns a compact, operator-facing summary of the
// recent attempts that led to a respawn-limit block.
func DescribeBeadRespawnHistory(workDir, beadID string) string {
	respawnMu.Lock()
	defer respawnMu.Unlock()

	townRoot := respawnTownRoot(workDir)
	unlock, flockErr := lock.FlockAcquire(beadRespawnStateFile(townRoot) + ".flock")
	if flockErr == nil {
		defer unlock()
	}

	state := loadBeadRespawnState(townRoot)
	rec, ok := state.Beads[beadID]
	if !ok {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "Recent respawn attempts for %s (total: %d):", beadID, rec.Count)
	if len(rec.RecentAttempts) > 0 {
		for _, attempt := range rec.RecentAttempts {
			reason := attempt.Reason
			if reason == "" {
				reason = "reason not recorded"
			}
			fmt.Fprintf(&b, "\n- #%d at %s: %s", attempt.Count, attempt.RespawnedAt.Format(time.RFC3339), reason)
		}
		return b.String()
	}

	if !rec.LastRespawn.IsZero() {
		fmt.Fprintf(&b, "\n- #%d at %s: legacy respawn record (reason not recorded)", rec.Count, rec.LastRespawn.Format(time.RFC3339))
	}
	return b.String()
}

// ResetBeadRespawnCount resets the respawn counter for beadID to zero.
// Used by `gt sling respawn-reset` to allow re-dispatch after investigation.
func ResetBeadRespawnCount(workDir, beadID string) error {
	respawnMu.Lock()
	defer respawnMu.Unlock()

	townRoot := respawnTownRoot(workDir)

	// Cross-process flock to serialize with other witness instances.
	unlock, flockErr := lock.FlockAcquire(beadRespawnStateFile(townRoot) + ".flock")
	if flockErr == nil {
		defer unlock()
	}

	state := loadBeadRespawnState(townRoot)
	delete(state.Beads, beadID)
	return saveBeadRespawnState(townRoot, state)
}
