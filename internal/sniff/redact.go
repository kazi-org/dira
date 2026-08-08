package sniff

import "regexp"

// What sniff refuses to capture, and why it drops rather than masks.
//
// Session text is untrusted input. A user pastes a key to get something
// unblocked, a tool prints an environment, a colleague's name appears in a
// requirement — and `dira sniff` runs unattended from a Stop hook, writing into
// a directory that is committed to a repository that may be public. cst-0003
// calls a leak here a security bug rather than a UX bug, and it is right: a
// credential in git history cannot be un-published, and rotating it is somebody
// else's afternoon.
//
// Two layers, and neither is "redact":
//
//  1. Tool input and tool output never reach the matcher (transcript.go). That
//     is where secrets actually live in a transcript — an `env` dump, a curl
//     with a bearer token, a config file printed to debug it.
//
//  2. Any candidate whose sentence carries something shaped like a credential is
//     dropped whole, here.
//
// Masking was rejected. `source.excerpt` exists so a human can audit what the
// hook inferred, and an excerpt reading "the key is [redacted], so we're going
// with X" is evidence with a hole in it: the reviewer cannot tell whether the
// removed span changed the meaning, and they cannot recover it. A dropped
// candidate loses at most one decision, which the human can still log by hand
// and which the semantic tier may still catch. A masked one puts an
// unauditable record in the artifact that is meant to outlive the tool.
//
// What this deliberately does NOT do:
//
//   - It does not scrub file paths, hostnames or personal names. It cannot tell
//     a customer's name from a library's, and a matcher that guessed would
//     silently drop real decisions while still missing real leaks. The boundary
//     that does bind is cst-0003's, enforced at the export edge, plus
//     scripts/privacy-lint.py over what is committed.
//   - It never sets `private: true`. Whether an entry is private is a property
//     of the ledger's tier and of a human's judgement, and a regex asserting it
//     would be the same category error as a regex asserting `accepted`.
//   - It never writes to a parent ledger. Inheritance is one-way (cst-0003 rule
//     1); the staging writer holds exactly the store it was given, which
//     openLedger resolves from the working directory downward.

// credentialPatterns are the shapes worth refusing on. Each is a token whose
// prefix or structure is specific enough that a false drop is rare, plus one
// generic assignment form for the rest.
var credentialPatterns = []*regexp.Regexp{
	// Provider-issued tokens, by their published prefixes.
	regexp.MustCompile(`\bsk-[A-Za-z0-9_-]{16,}`),
	regexp.MustCompile(`\b(?:ghp|gho|ghu|ghs|ghr)_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`\bgithub_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`\bxox[baprs]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\bAIza[0-9A-Za-z_-]{30,}`),

	// Private keys, in any of the armoured forms.
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----`),

	// The generic form: a secret-shaped name bound to a secret-shaped
	// value. The length floor is what keeps it off "password: yes".
	regexp.MustCompile(`(?i)\b(?:api[_-]?key|secret|token|password|passwd|credential)s?\b\s*[:=]\s*["']?[A-Za-z0-9/+_-]{12,}`),
	regexp.MustCompile(`(?i)\bauthorization\s*:\s*(?:bearer|basic)\s+\S{8,}`),
}

// carriesCredential reports whether a sentence must not be captured.
func carriesCredential(s string) bool {
	for _, re := range credentialPatterns {
		if re.MatchString(s) {
			return true
		}
	}
	return false
}
