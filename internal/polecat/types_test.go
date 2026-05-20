package polecat

import (
	"testing"
	"time"
)

func TestState_IsWorking(t *testing.T) {
	tests := []struct {
		state  State
		expect bool
	}{
		{StateWorking, true},
		{StateDone, false},
		{StateStuck, false},
		{StateStalled, false},
		{StateZombie, false},
		{State("unknown"), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsWorking(); got != tt.expect {
				t.Errorf("State(%q).IsWorking() = %v, want %v", tt.state, got, tt.expect)
			}
		})
	}
}

func TestPolecat_Summary(t *testing.T) {
	now := time.Now()
	p := &Polecat{
		Name:      "alpha",
		Rig:       "gastown",
		State:     StateWorking,
		ClonePath: "/some/path",
		Branch:    "polecat/alpha",
		Issue:     "gt-123",
		CreatedAt: now,
		UpdatedAt: now,
	}

	s := p.Summary()
	if s.Name != "alpha" {
		t.Errorf("Summary.Name = %q, want %q", s.Name, "alpha")
	}
	if s.State != StateWorking {
		t.Errorf("Summary.State = %q, want %q", s.State, StateWorking)
	}
	if s.Issue != "gt-123" {
		t.Errorf("Summary.Issue = %q, want %q", s.Issue, "gt-123")
	}
}

func TestPolecat_Summary_NoIssue(t *testing.T) {
	p := &Polecat{
		Name:  "beta",
		State: StateDone,
	}

	s := p.Summary()
	if s.Issue != "" {
		t.Errorf("Summary.Issue = %q, want empty", s.Issue)
	}
}

func TestState_IsStalled(t *testing.T) {
	tests := []struct {
		state  State
		expect bool
	}{
		{StateStalled, true},
		{StateWorking, false},
		{StateIdle, false},
		{StateDone, false},
		{StateStuck, false},
		{StateZombie, false},
		{State("unknown"), false},
	}
	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			if got := tt.state.IsStalled(); got != tt.expect {
				t.Errorf("State(%q).IsStalled() = %v, want %v", tt.state, got, tt.expect)
			}
		})
	}
}

func TestCleanupStatus_IsSafe(t *testing.T) {
	tests := []struct {
		status CleanupStatus
		expect bool
	}{
		{CleanupClean, true},
		{CleanupUncommitted, false},
		{CleanupStash, false},
		{CleanupUnpushed, false},
		{CleanupUnknown, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.IsSafe(); got != tt.expect {
				t.Errorf("CleanupStatus(%q).IsSafe() = %v, want %v", tt.status, got, tt.expect)
			}
		})
	}
}

func TestCleanupStatus_RequiresRecovery(t *testing.T) {
	tests := []struct {
		status CleanupStatus
		expect bool
	}{
		{CleanupClean, false},
		{CleanupUncommitted, true},
		{CleanupStash, true},
		{CleanupUnpushed, true},
		{CleanupUnknown, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.RequiresRecovery(); got != tt.expect {
				t.Errorf("CleanupStatus(%q).RequiresRecovery() = %v, want %v", tt.status, got, tt.expect)
			}
		})
	}
}

func TestCleanupStatus_CanForceRemove(t *testing.T) {
	tests := []struct {
		status CleanupStatus
		expect bool
	}{
		{CleanupClean, true},
		{CleanupUncommitted, true},
		{CleanupStash, false},
		{CleanupUnpushed, true},
		{CleanupUnknown, false},
	}
	for _, tt := range tests {
		t.Run(string(tt.status), func(t *testing.T) {
			if got := tt.status.CanForceRemove(); got != tt.expect {
				t.Errorf("CleanupStatus(%q).CanForceRemove() = %v, want %v", tt.status, got, tt.expect)
			}
		})
	}
}

func TestReusableIdlePolecatRequiresCleanCleanup(t *testing.T) {
	idle := &Polecat{Name: "toast", State: StateIdle}
	working := &Polecat{Name: "toast", State: StateWorking}

	tests := []struct {
		name          string
		polecat       *Polecat
		cleanupStatus CleanupStatus
		want          bool
	}{
		{name: "clean idle reusable", polecat: idle, cleanupStatus: CleanupClean, want: true},
		{name: "unpushed idle skipped", polecat: idle, cleanupStatus: CleanupUnpushed, want: false},
		{name: "uncommitted idle skipped", polecat: idle, cleanupStatus: CleanupUncommitted, want: false},
		{name: "stash idle skipped", polecat: idle, cleanupStatus: CleanupStash, want: false},
		{name: "unknown idle skipped", polecat: idle, cleanupStatus: CleanupUnknown, want: false},
		{name: "clean working skipped", polecat: working, cleanupStatus: CleanupClean, want: false},
		{name: "nil skipped", polecat: nil, cleanupStatus: CleanupClean, want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := reusableIdlePolecat(tt.polecat, tt.cleanupStatus); got != tt.want {
				t.Fatalf("reusableIdlePolecat() = %v, want %v", got, tt.want)
			}
		})
	}
}
