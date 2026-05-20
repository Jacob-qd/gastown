package cmd

import "testing"

func TestNormalizePRWorkflowOperation(t *testing.T) {
	tests := []struct {
		name      string
		operation string
		want      string
	}{
		{"empty", "", ""},
		{"pr shorthand", "pr", prWorkflowOperationPRCreate},
		{"gh pr", "gh-pr-create", prWorkflowOperationPRCreate},
		{"branch shorthand", "branch", prWorkflowOperationBranchCreate},
		{"feature branch", "feature-branch", prWorkflowOperationBranchCreate},
		{"unknown", "push", ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := normalizePRWorkflowOperation(tt.operation); got != tt.want {
				t.Fatalf("normalizePRWorkflowOperation(%q) = %q, want %q", tt.operation, got, tt.want)
			}
		})
	}
}

func TestShouldAllowPRWorkflowOperation(t *testing.T) {
	t.Run("polecat branch creation allowed", func(t *testing.T) {
		t.Setenv("GT_MERGE_STRATEGY", "")
		t.Setenv("GT_POLECAT", "dust")
		if !shouldAllowPRWorkflowOperation(prWorkflowOperationBranchCreate) {
			t.Fatal("expected polecat branch creation to be allowed")
		}
		if shouldAllowPRWorkflowOperation(prWorkflowOperationPRCreate) {
			t.Fatal("expected polecat PR creation to be blocked without PR merge strategy")
		}
	})

	t.Run("pr merge strategy allows PR operations", func(t *testing.T) {
		t.Setenv("GT_MERGE_STRATEGY", "pr")
		if !shouldAllowPRWorkflowOperation(prWorkflowOperationPRCreate) {
			t.Fatal("expected PR creation to be allowed with PR merge strategy")
		}
		if !shouldAllowPRWorkflowOperation(prWorkflowOperationBranchCreate) {
			t.Fatal("expected branch creation to be allowed with PR merge strategy")
		}
	})
}
