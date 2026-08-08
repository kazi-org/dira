package sniff

import "regexp"

// The matcher, in three lists.
//
// Its whole design comes from one measurement. Over a real 328KB Claude Code
// session — the one that built this repository — the phrases dec-0003 names as
// tier 1's bread and butter appear as follows:
//
//	"let's go with"     0 times
//	"we're not doing"   0 times
//	"going with"        0 times
//	"I'll <verb>"      41 times
//	"rather than"     160 times
//
// A matcher keyed on the first three catches nothing. A matcher keyed on either
// of the last two stages a hundred rows a session. The shape that actually
// separates a decision from an announcement is that a decision names a road not
// taken — which is also, and not by coincidence, what entry.schema.json requires
// of a decision and what dira exists to preserve. So:
//
//	strong      a phrase that is a decision on its own, because the
//	            rejection is inside the phrase ("ruled out", "we're not
//	            doing", "settled on", "chose X over Y")
//
//	commitment  somebody binding themselves to a course ("I'll", "we're",
//	            "let's") — necessary, never sufficient
//
//	contrast    the road not taken ("rather than", "instead of", "not a
//	            hosted renderer") — necessary, never sufficient
//
// A sentence fires when a strong pattern matches, or when a commitment and a
// contrast both match. Any guard suppresses it outright.
//
// Everything here is deliberately English-only and deliberately unclever. It is
// the cheap tier; dec-0003's second tier is the one that understands sentences,
// and it runs in a session that already has the transcript in context.

type pattern struct {
	name string
	re   *regexp.Regexp
}

func pat(name, expr string) pattern {
	return pattern{name: name, re: regexp.MustCompile(`(?i)` + expr)}
}

// strongPatterns fire on their own.
var strongPatterns = []pattern{
	// A colon and nothing else. "Decision:" is a label somebody typed to
	// mark one; "DECIDED — and on whose authority" is a heading in a report
	// template, and an em dash was enough to let two of those through on a
	// real transcript.
	pat("marker", `^\s*\**\s*decisions?\s*\**\s*:`),
	pat("marker", `^\s*\**\s*decided\s*\**\s*:`),
	pat("go-with", `\blet'?s go with\b`),
	pat("go-with", `^going with\b`),
	pat("go-with", `\b(?:we|i)(?:'re|'ll| are| will| am)?\s+going with\b`),
	pat("choose", `\bwe(?:'re| are)\s+choosing\b`),
	pat("choose", `\b(?:chose|choosing|chosen|picked|picking|went with|going with|settled on)\b[^.!?]{1,60}?\bover\b`),
	pat("settled", `\b(?:we|i)(?:'ve| have)?\s+(?:decided|settled)\s+(?:on|to|that)\b`),
	pat("ruled-out", `\bruled out\b`),
	pat("refusal", `\b(?:we|i)(?:'re|'m| are| am)?\s+not\s+(?:doing|going to|shipping|using|building|adding|writing)\b`),
	pat("refusal", `\b(?:we|i)\s+(?:won'?t|will not)\b`),
}

// commitmentPatterns bind a speaker to a course. They are necessary and never
// sufficient: 41 of the 41 "I'll <verb>" sentences in the measured session were
// announcements of the next tool call, not decisions.
//
// "let's" is here and "let me" is not, and that one-word difference is
// load-bearing. "Let's keep the manifest in this repo" proposes a course to
// another party; "let me check the manifest" narrates a step an agent is about
// to take. The second shape occurs constantly in agent transcripts and is never
// worth a ledger entry, so it is a guard rather than a commitment.
// The apostrophes are required, not optional, and both the typewriter and the
// typographic form are listed. Writing `we'?re` instead cost real precision on a
// real transcript: it matches the word "were", and three sentences of ordinary
// past-tense narration were staged as decisions before the run that found it.
var commitmentPatterns = []pattern{
	pat("commit-first-person", `\b(?:i'll|i\x{2019}ll|i will|i'm|i\x{2019}m|i am|we'll|we\x{2019}ll|we will|we're|we\x{2019}re|we are)\b`),
	pat("commit-lets", `\blet'?s\b`),
}

// contrastPatterns name the road not taken.
var contrastPatterns = []pattern{
	pat("contrast-rather", `\brather than\b`),
	pat("contrast-instead", `\binstead\b`),
	pat("contrast-not", `,\s*not\s+(?:a|an|the|to)\b`),
	pat("contrast-over", `\bover\b[^.!?]{0,40}$`),
}

// guardPatterns suppress a sentence outright, and they are where the precision
// actually comes from. Each family is here because the measured session produced
// it, not because it was imagined.
var guardPatterns = []pattern{
	// A question decides nothing, and the cheapest signal in the language.
	pat("question", `\?`),

	// Modals and conditionals: the lane's stated risk, verbatim. A ledger
	// of "we could go with X" is a ledger of decisions nobody made.
	pat("modal", `\b(?:could|might|maybe|perhaps|possibly|would|should|suppose|supposing|hypothetically)\b`),
	pat("conditional", `\b(?:if|unless|whether|in case)\b`),

	// Deferrals. These contain more decision vocabulary than real
	// decisions do, and every one of them is somebody declining to decide.
	pat("deferral", `\b(?:your call|needs a call|will decide later|to be decided|tbd|handing|still open|remains open|not yet decided)\b`),
	pat("option", `\b(?:one option|another option|options? (?:are|is)|either way)\b`),

	// A recommendation is advice until somebody takes it.
	pat("recommendation", `\brecommend`),

	// Somebody else's decision, reported back. The sniffer cannot tell a
	// restatement from a fresh call, and the measured session restated far
	// more often than it decided.
	pat("second-person", `\byou(?:'ve|'re| have| are)?\s+(?:picked|pick|chose|choose|chosen|sequenced|decided|settled|said|asked|want|wanted)\b`),

	// There is deliberately no "let me" guard. It would suppress nothing:
	// the commitment family requires "let's", so "let me verify X rather
	// than Y" never fires in the first place. A guard that cannot change an
	// outcome is protection in name only, and this package has a test that
	// says so by name.

	// A sentence that names a ledger entry is a sentence about the record,
	// not a new entry for it. This is the single highest-value guard: the
	// measured session cited dec-0001's "Go over Elixir" four separate
	// times, in the exact shape of a fresh choice.
	pat("citation", `\b(?:int|dec|qst|cst|note)-[0-9]{4,}\b`),
	pat("citation", `\badr-?\s?[0-9]{3,}\b`),

	// Rules and imperatives quoted into a session. A ledger of these is a
	// style guide.
	pat("instruction", `\b(?:never|always)\b`),

	// Machine text pasted into prose. Structural tool output never reaches
	// the matcher at all (transcript.go); this catches the rest.
	pat("code", `(?:&&|\|\||!=|:=|=>|::|\bfunc\b|\berr\b|https?://)`),
	pat("tool-output", `^(?:ok|fail|pass|---|===|\$|\+\+\+)(?:\s|$)`),
}

// match reports whether a sentence proposes an entry, and which family fired.
//
// Guards are checked first and win outright. There is no scoring and no
// threshold: a tier that could be talked into a candidate by accumulating weak
// signals is a tier whose false-positive rate cannot be reasoned about, and this
// one has to be argued with by a human reading testdata/corpus.yaml.
func match(sentence string) (string, bool) {
	return matchWith(sentence, guardPatterns)
}

// matchWith is match with the guard list supplied, for the test that measures
// what each guard is worth. It is a parameter rather than a package variable a
// test reassigns, so nothing outside a single call can change what the matcher
// suppresses — no flag, no environment variable and no ledger config can talk
// this tier into a candidate.
func matchWith(sentence string, guards []pattern) (string, bool) {
	if len([]rune(sentence)) < 20 {
		// Below this a sentence carries no reviewable evidence, and the
		// title would be shorter than the schema's three-character floor
		// once markup is stripped.
		return "", false
	}
	for _, g := range guards {
		if g.re.MatchString(sentence) {
			return "", false
		}
	}
	for _, p := range strongPatterns {
		if p.re.MatchString(sentence) {
			return p.name, true
		}
	}

	commitment, ok := first(commitmentPatterns, sentence)
	if !ok {
		return "", false
	}
	contrast, ok := first(contrastPatterns, sentence)
	if !ok {
		return "", false
	}
	return commitment + "+" + contrast, true
}

func first(patterns []pattern, s string) (string, bool) {
	for _, p := range patterns {
		if p.re.MatchString(s) {
			return p.name, true
		}
	}
	return "", false
}
