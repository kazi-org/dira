package main

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"reflect"
	"runtime"
	"strings"
	"testing"
)

// TestEveryRunFunctionIsRegistered catches the defect this repo has now shipped
// three times: a command built, tested, merged — and absent from newApp's
// registry, so the binary answers "unknown command" for a verb whose tests all
// pass.
//
// It happened to `dira sniff`, which was unreachable for weeks while
// hooks/settings.example.json installed it, so the whole regex capture path was
// dead. It happened to `dira install-skill`. Both were found by a later lane
// whose own acceptance could not pass without them, which is luck rather than a
// gate.
//
// The lane protocol is the cause and it is a good protocol: a lane may not edit
// main.go, so it reports a registry line and the integrator adds it. This test
// is what makes forgetting that line loud instead of silent.
//
// It reads the SOURCE rather than a hand-listed slice. A list of verbs to check
// would be one more thing to forget to update — the same defect wearing a hat.
func TestEveryRunFunctionIsRegistered(t *testing.T) {
	t.Parallel()

	fset := token.NewFileSet()
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("reading the command directory: %v", err)
	}

	// runX functions found in non-test source.
	found := map[string]string{} // runName -> file
	for _, e := range entries {
		name := e.Name()
		if !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		f, err := parser.ParseFile(fset, name, nil, 0)
		if err != nil {
			t.Fatalf("parsing %s: %v", name, err)
		}
		for _, d := range f.Decls {
			fn, ok := d.(*ast.FuncDecl)
			if !ok || fn.Recv != nil || !strings.HasPrefix(fn.Name.Name, "run") || fn.Name.Name == "run" {
				continue
			}
			// A command's run function takes (*app, []string).
			if fn.Type.Params == nil || len(fn.Type.Params.List) != 2 {
				continue
			}
			found[fn.Name.Name] = name
		}
	}

	// Without this the test passes just as happily on an empty parse.
	if len(found) < 5 {
		t.Fatalf("found %d run functions in this package; the check is not measuring anything", len(found))
	}

	// Resolve what is registered from the FUNCTION VALUES, not from the command
	// names. Deriving "runUI" from the verb "ui" needs a rule about acronyms,
	// and a check that has to guess at names will eventually guess wrong — this
	// one did, on its first run, and reported `dira ui` as unregistered when it
	// is registered. runtime.FuncForPC asks the binary what was actually wired.
	registered := map[string]bool{}
	for _, c := range newApp(nil, nil).commands {
		if c.run == nil {
			continue
		}
		full := runtime.FuncForPC(reflect.ValueOf(c.run).Pointer()).Name()
		registered[full[strings.LastIndex(full, ".")+1:]] = true
	}

	for run, file := range found {
		if !registered[run] {
			t.Errorf("%s is defined in %s but no command in newApp's registry uses it.\n"+
				"A verb the binary cannot reach is worse than one that does not exist: its tests pass, its\n"+
				"lane reports done, and `dira <verb>` answers \"unknown command\". Add the registry line in\n"+
				"main.go, or delete the function.", run, file)
		}
	}
}
