//go:build !race

package fixture_test

// raceEnabled reports whether the race detector is instrumenting this build.
// Timing assertions are meaningless under it — instrumentation costs several
// times the work being measured — so the budget test skips rather than reporting
// a number that is not dira's.
const raceEnabled = false
