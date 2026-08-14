package cli

// export_test.go exposes a handful of unexported helpers to the external
// cli_test package, for tests that need to check a rendering detail without
// standing up a whole tree. Standard Go pattern: this file compiles only
// under `go test`, never into the real binary.

// RenderRowSuffixForTest exposes renderRowSuffix.
func RenderRowSuffixForTest(n *Node) string { return renderRowSuffix(n) }
