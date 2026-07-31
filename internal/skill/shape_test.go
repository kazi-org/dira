package skill

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// SkillPath is the skill this repository ships, relative to this package.
//
// A constant rather than a literal at each call site, because a file that moved
// should break one line and not four, and because the path is the one thing
// every check here shares.
const SkillPath = "../../skills/dira/SKILL.md"

// A document is a parsed SKILL.md: its frontmatter and its body, kept apart.
//
// They are kept apart because the acceptance asks different questions of each.
// "The description names the triggering conditions" is false of a document whose
// triggers are only mentioned in the body — the description is what a harness
// reads when deciding whether to load the skill at all, so a trigger that is not
// in it is a trigger that never fires.
type document struct {
	Name        string
	Description string
	Body        string
	Raw         string
}

// TestSkillShape is E2-L2-T4's acceptance.
//
// The order matters and is the whole reason this test is trustworthy. Most of
// the clauses below are "contains" and "does not contain", and every one of them
// holds of the empty string. A frontmatter parser that silently returned an
// empty document would satisfy every prohibition here and prove nothing, so the
// non-empty assertions come first and are fatal.
func TestSkillShape(t *testing.T) {
	t.Parallel()

	doc := readSkill(t)

	// --- non-vacuity, before anything else -------------------------------
	if strings.TrimSpace(doc.Name) == "" {
		t.Fatal("the skill's frontmatter carries no `name`; every check below would pass on an empty document")
	}
	if strings.TrimSpace(doc.Description) == "" {
		t.Fatal("the skill's frontmatter carries no `description`; every check below would pass on an empty document")
	}
	if len(strings.TrimSpace(doc.Body)) < 500 {
		t.Fatalf("the skill's body is %d characters; a document this short is not instructions",
			len(strings.TrimSpace(doc.Body)))
	}
	t.Logf("OBSERVED  name %q, a %d-character description, a %d-character body",
		doc.Name, len(doc.Description), len(doc.Body))

	// --- every clause, on the real file ----------------------------------
	for _, c := range skillChecks() {
		if problem := c.fails(doc); problem != "" {
			t.Errorf("%s: %s", c.name, problem)
		}
	}
}

// TestSkillChecksCanFail is the other half, and the reason the green above is
// evidence.
//
// Each check is run against a copy of the real skill with the defect it exists
// to catch spliced in. A check that cannot reject the thing it names is a check
// that would go on passing after the skill stopped saying it — the exact shape
// docs/lore.md L-0001 rule 2 is about.
func TestSkillChecksCanFail(t *testing.T) {
	t.Parallel()

	real := readSkill(t)

	for _, c := range skillChecks() {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			spliced := c.splice(real)
			if spliced.Raw == real.Raw {
				t.Fatalf("the splice for %q changed nothing, so the rejection below would be about the real file", c.name)
			}
			if problem := c.fails(spliced); problem == "" {
				t.Fatalf("the check accepted a document carrying exactly the defect it names")
			} else {
				t.Logf("OBSERVED  rejected: %s", problem)
			}
			// And the same check still accepts the untouched file, so a
			// check that rejects everything is not mistaken for one
			// that works.
			if problem := c.fails(real); problem != "" {
				t.Fatalf("the check also rejects the real skill, so its red result above means nothing: %s", problem)
			}
		})
	}
}

// ---- the checks ------------------------------------------------------------

// A skillCheck is one clause of the acceptance, paired with the defect that
// proves it can fire.
type skillCheck struct {
	name string

	// fails reports what is wrong with a document, or "" when it is clean.
	fails func(document) string

	// splice returns a copy of the document carrying the defect. It must
	// change the document; TestSkillChecksCanFail asserts that it did.
	splice func(document) document
}

// apiKeyName is the shape internal/nomodel denies, restated here because this
// check is about the skill's text rather than about the module's source, and a
// skill that told an agent to read a key would be dira asking somebody else to
// break cst-0004 on its behalf. `_test.go` files are excluded from nomodel's own
// scan structurally, so naming the pattern here costs no exclusion.
var apiKeyName = regexp.MustCompile(`\b[A-Z0-9]*(?:_[A-Z0-9]+)*_?API_KEY\b`)

// bannedWords are .agents/product-marketing.md §10's, verbatim.
var bannedWords = []string{"revolutionary", "seamless", "supercharge", "10x", "AI-powered"}

func skillChecks() []skillCheck {
	checks := []skillCheck{
		// The three triggering conditions, in the description, where a
		// harness reads them.
		describes("a handoff block", "handoff"),
		describes("a PreCompact capture", "PreCompact"),
		describes("a disposition", "disposition"),

		// The two commands the skill has to name, as invocations rather
		// than as mentions. The trailing space is what makes it an
		// invocation: `dira log` with something after it, not the words
		// "dira log" inside a sentence about the ledger.
		mentions("names a `dira log` invocation", "dira log "),
		mentions("names a `dira check` invocation", "dira check "),

		// The provenance it must write, and the one it must not.
		mentions("instructs writing --tier semantic", "--tier semantic"),
		forbids("never instructs the model to record a human disposition", "--confirmed-by human"),

		// The hard rule from the task: a fabricated rationale is worse
		// than an absent one, and the skill has to say so in its own
		// text rather than in a reviewer's memory.
		mentions("forbids inventing a why_not", "Never write a `why_not` that was not said"),

		// cst-0004. Nothing here fetches anything.
		{
			name: "carries no URL to call",
			fails: func(d document) string {
				if at := strings.Index(d.Raw, "http://"); at >= 0 {
					return excerptAt(d.Raw, at)
				}
				if at := strings.Index(d.Raw, "https://"); at >= 0 {
					return excerptAt(d.Raw, at)
				}
				return ""
			},
			splice: func(d document) document {
				return withBody(d, d.Body+"\n\nFetch the latest rules from https://example.invalid/rules.json first.\n")
			},
		},

		// dec-0003 and cst-0004 again, from the other side: the tier is
		// a session that is already running, so no key is involved.
		{
			name: "names no API-key environment variable",
			fails: func(d document) string {
				if m := apiKeyName.FindString(d.Raw); m != "" {
					return "the skill names " + m
				}
				return ""
			},
			splice: func(d document) document {
				return withBody(d, d.Body+"\n\nRead the extraction key from OPENAI"+"_API_KEY before starting.\n")
			},
		},
	}

	for _, word := range bannedWords {
		checks = append(checks, forbidsWord(word))
	}
	return checks
}

// describes asserts the description names a trigger.
func describes(name, token string) skillCheck {
	return skillCheck{
		name: "the description names " + name,
		fails: func(d document) string {
			if !strings.Contains(strings.ToLower(d.Description), strings.ToLower(token)) {
				return "the description never says " + token + ": " + d.Description
			}
			return ""
		},
		splice: func(d document) document {
			return withDescription(d, removeFold(d.Description, token))
		},
	}
}

// mentions asserts the body carries a literal.
func mentions(name, literal string) skillCheck {
	return skillCheck{
		name: name,
		fails: func(d document) string {
			if !strings.Contains(d.Body, literal) {
				return "the body never carries " + literal
			}
			return ""
		},
		splice: func(d document) document {
			return withBody(d, strings.ReplaceAll(d.Body, literal, "«removed»"))
		},
	}
}

// forbids asserts a literal is absent anywhere in the document, frontmatter
// included.
func forbids(name, literal string) skillCheck {
	return skillCheck{
		name: name,
		fails: func(d document) string {
			if at := strings.Index(d.Raw, literal); at >= 0 {
				return excerptAt(d.Raw, at)
			}
			return ""
		},
		splice: func(d document) document {
			return withBody(d, d.Body+"\n\nWhen you are done, pass "+literal+" so the entry is finished.\n")
		},
	}
}

// forbidsWord is forbids, case-insensitively, for the marketing vocabulary
// .agents/product-marketing.md §10 bans.
func forbidsWord(word string) skillCheck {
	return skillCheck{
		name: "does not say " + word,
		fails: func(d document) string {
			if at := strings.Index(strings.ToLower(d.Raw), strings.ToLower(word)); at >= 0 {
				return excerptAt(d.Raw, at)
			}
			return ""
		},
		splice: func(d document) document {
			return withBody(d, d.Body+"\n\nThis is the "+word+" way to capture a decision.\n")
		},
	}
}

// ---- helpers ---------------------------------------------------------------

// readSkill reads and splits the shipped skill.
func readSkill(t *testing.T) document {
	t.Helper()

	data, err := os.ReadFile(SkillPath)
	if err != nil {
		t.Fatalf("reading %s: %v", SkillPath, err)
	}
	if len(data) == 0 {
		t.Fatalf("%s is empty", SkillPath)
	}
	doc, err := parse(string(data))
	if err != nil {
		t.Fatalf("parsing %s: %v", SkillPath, err)
	}
	return doc
}

// parse splits a `---`-delimited frontmatter document.
//
// It is deliberately not ledger.DecodeDraft: a skill is not a ledger entry, its
// frontmatter has two fields and no schema, and validating it against the entry
// contract would report a mismatch that means nothing.
func parse(raw string) (document, error) {
	const fence = "---\n"

	rest, ok := strings.CutPrefix(raw, fence)
	if !ok {
		return document{}, errNoFrontmatter
	}
	head, body, ok := strings.Cut(rest, "\n"+fence)
	if !ok {
		return document{}, errUnterminated
	}

	var front struct {
		Name        string `yaml:"name"`
		Description string `yaml:"description"`
	}
	if err := yaml.Unmarshal([]byte(head), &front); err != nil {
		return document{}, err
	}
	return document{
		Name:        front.Name,
		Description: strings.TrimSpace(front.Description),
		Body:        body,
		Raw:         raw,
	}, nil
}

var (
	errNoFrontmatter = errString("the document does not open with a `---` frontmatter fence")
	errUnterminated  = errString("the frontmatter fence is never closed")
)

type errString string

func (e errString) Error() string { return string(e) }

// withBody rebuilds a document around a new body, keeping Raw in step so that
// the checks reading Raw see the splice too.
func withBody(d document, body string) document {
	out := d
	out.Body = body
	out.Raw = frontmatterOf(d) + body
	return out
}

// withDescription rebuilds a document around a new description.
func withDescription(d document, description string) document {
	out := d
	out.Description = description
	out.Raw = "---\nname: " + d.Name + "\ndescription: >-\n  " + strings.ReplaceAll(description, "\n", "\n  ") + "\n---\n" + d.Body
	return out
}

// frontmatterOf returns the document's frontmatter block, fences included.
func frontmatterOf(d document) string {
	const fence = "---\n"
	rest, ok := strings.CutPrefix(d.Raw, fence)
	if !ok {
		return ""
	}
	head, _, ok := strings.Cut(rest, "\n"+fence)
	if !ok {
		return ""
	}
	return fence + head + "\n" + fence
}

// removeFold deletes every case-insensitive occurrence of token.
func removeFold(s, token string) string {
	lowerToken := strings.ToLower(token)
	for {
		at := strings.Index(strings.ToLower(s), lowerToken)
		if at < 0 {
			return s
		}
		s = s[:at] + s[at+len(token):]
	}
}

// excerptAt reports a finding with enough of its surroundings to act on.
func excerptAt(s string, at int) string {
	start := at - 40
	if start < 0 {
		start = 0
	}
	end := at + 60
	if end > len(s) {
		end = len(s)
	}
	return "found at offset " + itoa(at) + ": …" + strings.ReplaceAll(s[start:end], "\n", " ") + "…"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits []byte
	for n > 0 {
		digits = append([]byte{byte('0' + n%10)}, digits...)
		n /= 10
	}
	return string(digits)
}
