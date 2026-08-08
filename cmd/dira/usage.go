package main

import (
	"fmt"
	"io"
	"strings"
)

// writeUsage renders the top-level help. Every registered command appears
// here; the registry is the single source, so a command that is reachable but
// undocumented is not possible.
//
// The help text is assembled in memory and written once. Writing usage to a
// closed pipe (`dira --help | head`) is a real failure and an unactionable one
// — there is nowhere left to report it — so the single write's error is
// discarded explicitly rather than at thirty separate call sites.
func (a *app) writeUsage(w io.Writer) {
	var b strings.Builder

	b.WriteString("dira - a git-native ledger of intents, decisions, rejected\n")
	b.WriteString("alternatives, open questions, and constraints.\n\n")
	fmt.Fprintf(&b, "usage:\n\n\t%s <command> [arguments]\n\n", a.name)

	b.WriteString("commands:\n\n")
	width := 0
	for _, c := range a.commands {
		if len(c.name) > width {
			width = len(c.name)
		}
	}
	for _, c := range a.commands {
		fmt.Fprintf(&b, "\t%-*s  %s\n", width, c.name, c.summary)
	}

	b.WriteString("\nflags:\n\n")
	b.WriteString("\t-h, -help   show this help\n")
	b.WriteString("\t-version    print the version\n")

	b.WriteString("\nexit codes:\n\n")
	b.WriteString("\t0  success\n")
	b.WriteString("\t1  runtime error\n")
	b.WriteString("\t2  usage error\n")

	_, _ = io.WriteString(w, b.String())
}
