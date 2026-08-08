#!/bin/sh
# Install dira's pre-commit hook. .git/hooks is not tracked by git, so a fresh
# clone gets no hook at all — the gates that make this repo's guarantees real
# would silently not run. Run this after cloning.
set -eu
cd "$(dirname "$0")/.."
cp hooks/pre-commit .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
echo "installed .git/hooks/pre-commit — coverage, privacy lint, golangci-lint, go test"
