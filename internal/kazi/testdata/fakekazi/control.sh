#!/bin/sh
# The one fake that behaves correctly — proves the harness itself is not
# broken in a way that always fails, and (via its distinctive
# testdata_kazi_fixture_sentinel value) lets a test prove PATH resolved to
# THIS script rather than to an ambient system kazi.
case "$1" in
  portfolio)
    echo '{"schema_version":2,"kind":"portfolio","planned":[],"by_repo":{},"fleet_remote":[],"totals":{"base":7,"empty":false,"rows":[{"bucket":"done","count":7,"pct":100}]},"todo":[],"blocked":[],"rate":{"total":0,"green":0,"empty?":true,"delta":0},"sentinel":"testdata_kazi_fixture_sentinel"}'
    ;;
  status)
    echo '{"schema_version":2,"kind":"run","ref":"'"$2"'","status":"in_progress","converged":false,"release_ref":null,"observed_at":"2026-08-14T00:00:00Z"}'
    ;;
  *)
    echo 'error: unrecognised subcommand' >&2
    exit 2
    ;;
esac
exit 0
