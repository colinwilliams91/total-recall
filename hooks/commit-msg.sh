#!/usr/bin/env bash
# total-recall managed
# Managed by Total Recall — do not edit this block manually.
# Re-run 'torec repo' to update.

# No-op: pre-commit is the only dispatch point on a git commit. Commit-msg
# previously dispatched too, so a single commit with the daemon down printed
# the advisory twice. Kept as a clean exit so a chained pre-existing
# unmanaged commit-msg hook can still be installed over.
exit 0
