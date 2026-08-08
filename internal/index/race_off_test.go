//go:build !race

package index_test

// raceEnabled reports whether this binary was built with the race detector. The
// timing test skips under it: instrumentation costs several times the work being
// measured, so the number it would report is about the detector, not about dira.
const raceEnabled = false
