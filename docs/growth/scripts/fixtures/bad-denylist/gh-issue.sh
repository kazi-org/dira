#!/bin/sh
# Bad fixture: proves check-drafts.mjs's deny-list scan catches a
# `gh issue create` invocation. Never run this file.
gh issue create --title "filed by an agent" --body "should never happen"
