#!/bin/sh
# Sleeps well past any deadline this suite sets, so a timeout implementation
# that waits for the process and reports ReasonTimeout late (rather than at
# the deadline) is caught by the elapsed-time assertion in failopen_test.go,
# not by this script ever finishing.
#
# /bin/sleep, not a bare `sleep`: this suite runs it with PATH set to
# exactly this fixture's own directory (isolatedPATH in failopen_test.go),
# by design, to prove PATH resolution. A bare `sleep` is unresolvable under
# that PATH, which silently skipped the sleep entirely rather than testing
# deadline enforcement.
#
# `exec`, not a plain call: a plain `/bin/sleep 5` runs as sh's CHILD, so
# when the context deadline kills sh's pid, sleep is orphaned and keeps
# running — and since it inherited the stdout pipe Snapshot() reads from,
# cmd.Wait() blocks until sleep exits on its own 5s later, not at the
# deadline. `exec` replaces sh's own process image with sleep (same pid,
# no child), so killing that pid kills the thing actually sleeping,
# immediately — matching a real `kazi` invocation, which is one process
# with no forked children to begin with.
exec /bin/sleep 5
