#!/bin/sh
# A bad-argument / unknown-command shape: exits 2, per lane doc point 10.
# Behaves the same regardless of subcommand — this fake matters to both
# Snapshot() and Status().
echo 'error: unknown option --bogus-flag' >&2
exit 2
