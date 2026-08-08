// Package skilltest locates the skill document this repository ships.
//
// It exists as its own package, and is on internal/ledger/boundary_test.go's
// allowlist, for the same reason internal/index/indextest is: dec-0005 says
// only a storage backend may name a path, and internal/skill is pure policy —
// it takes a document's TEXT and returns what it found. Reading the file is a
// filesystem concern, and two packages in two different directories need to
// read the same file, so the walk lives here rather than being duplicated in
// each of their test files or hard-coded as a relative constant that is right
// from one directory and a landmine from the other.
//
// Nothing in the shipped binary imports this.
package skilltest

import (
	"fmt"
	"os"
	"path/filepath"
)

// Locate returns the path of the skill this repository ships, found by walking
// up from the working directory to the directory holding go.mod.
//
// A walk rather than a relative constant because two packages check this file
// from two different directories, and a constant that is right from one of them
// is a landmine for the other the first time a check moves.
func Locate() (string, error) {
	start, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("locating the skill: %w", err)
	}
	dir := start
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			path := filepath.Join(dir, "skills", "dira", "SKILL.md")
			if _, err := os.Stat(path); err != nil {
				return "", fmt.Errorf("module root %s carries no skill at %s", dir, path)
			}
			return path, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("no go.mod in %s or any directory above it", start)
		}
		dir = parent
	}
}

// ReadSkill returns the shipped skill document's text.
//
// It returns TEXT, not parsed invocations, so this package does not import
// internal/skill — an in-package test importing a helper that imports its own
// package is a cycle Go refuses. The caller parses.
func ReadSkill() (string, error) {
	path, err := Locate()
	if err != nil {
		return "", err
	}
	data, err := os.ReadFile(path) // #nosec G304 -- a located repository path, not user input
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", path, err)
	}
	return string(data), nil
}
