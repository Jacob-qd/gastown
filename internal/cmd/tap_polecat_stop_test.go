package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDetectPolecatStopPendingWork_UncommittedImplementationWork(t *testing.T) {
	repo := newPolecatStopRepo(t)
	testRunGit(t, repo, "checkout", "-b", "polecat/test")
	writeRepoFile(t, repo, "feature.txt", "work in progress\n")

	got := detectPolecatStopPendingWork(repo)
	if !got.Pending {
		t.Fatal("expected uncommitted implementation work to trigger gt done safety net")
	}
	if got.Branch != "polecat/test" {
		t.Fatalf("branch = %q, want polecat/test", got.Branch)
	}
	if !strings.Contains(got.Summary, "uncommitted") {
		t.Fatalf("summary = %q, want uncommitted work", got.Summary)
	}
}

func TestDetectPolecatStopPendingWork_RuntimeOnlyChangesIgnored(t *testing.T) {
	repo := newPolecatStopRepo(t)
	testRunGit(t, repo, "checkout", "-b", "polecat/test")
	mustMkdirAll(t, filepath.Join(repo, ".runtime"))
	writeRepoFile(t, repo, filepath.Join(".runtime", "state.json"), "{}\n")

	got := detectPolecatStopPendingWork(repo)
	if got.Pending {
		t.Fatalf("runtime-only changes should not trigger gt done safety net: %+v", got)
	}
}

func TestDetectPolecatStopPendingWork_UnpushedCommits(t *testing.T) {
	repo := newPolecatStopRepo(t)
	testRunGit(t, repo, "checkout", "-b", "polecat/test")
	writeRepoFile(t, repo, "feature.txt", "committed work\n")
	testRunGit(t, repo, "add", ".")
	testRunGit(t, repo, "commit", "-m", "feature")

	got := detectPolecatStopPendingWork(repo)
	if !got.Pending {
		t.Fatal("expected unpushed commits to trigger gt done safety net")
	}
	if !strings.Contains(got.Summary, "unpushed commit") {
		t.Fatalf("summary = %q, want unpushed commits", got.Summary)
	}
}

func newPolecatStopRepo(t *testing.T) string {
	t.Helper()
	tmp := t.TempDir()
	origin := filepath.Join(tmp, "origin.git")
	repo := filepath.Join(tmp, "repo")

	testRunGit(t, tmp, "init", "--bare", origin)
	testRunGit(t, tmp, "init", "--initial-branch", "main", repo)
	testRunGit(t, repo, "config", "user.email", "test@test.com")
	testRunGit(t, repo, "config", "user.name", "Test")
	writeRepoFile(t, repo, "README.md", "# test\n")
	testRunGit(t, repo, "add", ".")
	testRunGit(t, repo, "commit", "-m", "initial")
	testRunGit(t, repo, "remote", "add", "origin", origin)
	testRunGit(t, repo, "push", "-u", "origin", "main")

	return repo
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}
