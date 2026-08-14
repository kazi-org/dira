#!/bin/sh
# Sleeps well past any deadline this suite sets, then would answer cleanly —
# so a timeout implementation that waits for the process and reports
# ReasonTimeout late (rather than at the deadline) is caught by the elapsed-
# time assertion in failopen_test.go, not by this script ever finishing.
sleep 5
echo '{"schema_version":2,"kind":"portfolio"}'
exit 0
