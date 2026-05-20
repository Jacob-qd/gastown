package refinery

import (
	"fmt"
	"io"
	"strings"

	"github.com/steveyegge/gastown/internal/beads"
	"github.com/steveyegge/gastown/internal/style"
)

func mergedIssueCloseReason(mrID, targetBranch, mergeCommit string) string {
	closeReason := fmt.Sprintf("Merged in %s", mrID)
	if mergeCommit != "" {
		closeReason = fmt.Sprintf("%s\ntarget_branch: %s\ncommit_sha: %s", closeReason, targetBranch, mergeCommit)
	}
	return closeReason
}

func closeMergedSourceIssue(b *beads.Beads, output io.Writer, sourceIssueID, mrID, targetBranch, mergeCommit, logPrefix string) (closed bool, notFound bool) {
	if sourceIssueID == "" {
		return false, false
	}

	var externalRef string
	if sourceIssue, err := b.Show(sourceIssueID); err == nil {
		externalRef = closableBeadRef(sourceIssue.ExternalRef)
	} else if err != beads.ErrNotFound {
		_, _ = fmt.Fprintf(output, "%s Warning: failed to inspect source issue %s for external_ref: %v\n", logPrefix, sourceIssueID, err)
	}

	closeReason := mergedIssueCloseReason(mrID, targetBranch, mergeCommit)
	closed, notFound = forceCloseMergedIssue(b, output, sourceIssueID, closeReason, logPrefix, "source issue")
	if !closed || externalRef == "" || externalRef == sourceIssueID {
		return closed, notFound
	}

	aliasReason := fmt.Sprintf("%s\nvia_alias: %s", closeReason, sourceIssueID)
	externalClosed, _ := forceCloseMergedIssue(b, output, externalRef, aliasReason, logPrefix, "source external_ref")
	if externalClosed {
		_, _ = fmt.Fprintf(output, "%s Closed source external_ref: %s\n", logPrefix, externalRef)
	}
	return closed, notFound
}

func forceCloseMergedIssue(b *beads.Beads, output io.Writer, issueID, reason, logPrefix, label string) (closed bool, notFound bool) {
	if err := b.ForceCloseWithReason(reason, issueID); err != nil {
		if issue, showErr := b.Show(issueID); showErr == nil && beads.IssueStatus(issue.Status).IsTerminal() {
			_, _ = fmt.Fprintf(output, "%s %s already closed: %s\n", logPrefix, label, issueID)
			return true, false
		}
		_, _ = fmt.Fprintf(output, "%s %s close: %v\n", style.Dim.Render("○"), label, err)
		return false, true
	}
	return true, false
}

func closableBeadRef(ref string) string {
	ref = strings.TrimSpace(beads.ExtractIssueID(ref))
	if ref == "" || beads.ExtractPrefix(ref) == "" {
		return ""
	}
	return ref
}
