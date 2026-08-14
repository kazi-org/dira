#!/bin/sh
# Valid JSON, exit 0, but a kind neither Snapshot() ("portfolio") nor
# Status() ("run" | "proposal") ever expects.
echo '{"schema_version":2,"kind":"status"}'
exit 0
