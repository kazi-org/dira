// Package importadr is the corpus-agnostic half of `dira import`: it takes an
// ADR document's raw markdown bytes and returns what it found, and it takes a
// batch of what it found and decides what to write. Nothing in this package
// names a path — internal/ledger/boundary_test.go's allowlist gains no entry
// for it, the same restructuring internal/skill made for the same reason
// (dec-0005). cmd/dira/import.go is the only thing in this lane that walks a
// directory.
//
// extract.go is the structural extractor E2-L7-T2 ports from the neutrality
// experiment's `extract2.py` (never committed — session scratchpad only, per
// qst-0003-neutrality.md §7). It handles four document shapes without knowing
// which one it is reading:
//
//  1. inline-reason bullets — option and reason in one bullet (kazi's own
//     house style, "## Alternatives rejected");
//  2. sub-heading-per-option — an H3 per option with prose underneath, no
//     bulleted pros/cons (Sylius's variant of MADR);
//  3. names-here-reasons-elsewhere — MADR proper: "## Considered Options"
//     lists bare names, "## Pros and Cons of the Options" carries the
//     reasons in a separate H3-per-option block, joined by option identity;
//  4. no alternatives section at all (Nygard: Status/Context/Decision/
//     Consequences, nothing else).
//
// Three bugs the neutrality experiment found and fixed are named in
// docs/plan/tasks/E2-L7.md and reproduced as regression cases in
// extract_test.go, because a reimplementation that skips them rediscovers
// them:
//
//   - an "### Option N: …" heading *inside* a Pros-and-Cons block must never
//     be treated as if it opened a new top-level alternatives section — only
//     "## " headings are section boundaries, ever;
//   - an option's descriptive name is never split at its first colon (that
//     turned "Option 1: Assume DELETE requests will be mediated by other
//     systems" into the bare option "Option 1" and its own name into the
//     reason) — colon is never used as a label/reason delimiter anywhere in
//     this file;
//   - a sub-sub-heading inside an option's own prose ("#### Branches") is
//     never counted as a new option — only "## " and "### " headings are
//     section/option boundaries; anything deeper is body text.
package importadr

import (
	"regexp"
	"strings"
)

// Classification is a coarse read on how substantial an alternative's reason
// is. It exists for the controls (0002 vs 0003) and is not what T2's acc pins
// on the two named corpora, which count non-empty reasons, not this.
type Classification string

// The three classifications. thinMaxWords is the boundary between thin and
// reasoned: a reason no longer than a label ("too slow") teaches a reader
// nothing a bare option name did not already say.
const (
	ClassBare     Classification = "bare"
	ClassThin     Classification = "thin"
	ClassReasoned Classification = "reasoned"
)

// thinMaxWords is inclusive: a reason of this many words or fewer is thin.
const thinMaxWords = 3

// Alternative is a road not taken, extracted from a document's own text.
// Option and Reason mirror ledger.Alternative's Option/WhyNot; this package
// imports no ledger type because it commits to nothing about the schema — that
// mapping is draft.go's job, one layer up, where the state and provenance the
// schema also wants are decided.
type Alternative struct {
	Option         string
	Reason         string
	RevisitIf      string
	Classification Classification
}

// Document is one document's extraction result.
type Document struct {
	Alternatives []Alternative
}

// WithReason reports whether at least one alternative in d carries a non-empty
// Reason. This is the literal measure T2's acc pins against the two named
// corpora ("44 documents with ≥1 alternative carrying a non-empty reason"),
// deliberately not Classification, which the controls use instead.
func (d Document) WithReason() bool {
	for _, a := range d.Alternatives {
		if a.Reason != "" {
			return true
		}
	}
	return false
}

// TitleOf returns a document's title: the text of its first level-1 ("# ")
// heading, or "" if it has none. It is used one layer up (report.go) to name
// a scanned document without this package ever knowing the path it came
// from — the title lives in the document's own bytes, the same way every
// other fact this package extracts does.
func TitleOf(text string) string {
	for line := range strings.SplitSeq(strings.ReplaceAll(text, "\r\n", "\n"), "\n") {
		m := headingRe.FindStringSubmatch(line)
		if m != nil && len(m[1]) == 1 {
			return m[2]
		}
	}
	return ""
}

// headingRe matches an ATX heading line and captures its level (the number of
// `#`) and text.
var headingRe = regexp.MustCompile(`^(#{1,6})\s+(.*?)\s*$`)

// bulletStartRe reports whether a line opens a new bullet, without capturing
// its text — groupBullets uses this to find where one bullet ends and the
// next begins.
var bulletStartRe = regexp.MustCompile(`^(?:[*-]|\d+[.)])\s+`)

// groupBullets joins a wrapped bullet's continuation lines onto it — an
// indented line following a bullet, with no blank line and no new bullet
// marker between them, is that bullet's own text carrying on, and a real
// corpus wraps long reasons this way as routinely as this lane's own
// controls do. Each returned string is one bullet's full text with its
// leading marker stripped and internal line breaks collapsed to single
// spaces.
func groupBullets(lines []string) []string {
	var out []string
	var current strings.Builder
	flush := func() {
		if current.Len() > 0 {
			out = append(out, strings.TrimSpace(current.String()))
			current.Reset()
		}
	}
	for _, line := range lines {
		if bulletStartRe.MatchString(line) {
			flush()
			current.WriteString(bulletStartRe.ReplaceAllString(strings.TrimSpace(line), ""))
			continue
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			flush()
			continue
		}
		if current.Len() == 0 {
			continue // prose outside any bullet, not this function's job
		}
		current.WriteString(" ")
		current.WriteString(trimmed)
	}
	flush()
	return out
}

// heading is one ATX heading of a chosen level, and the raw lines that follow
// it up to (but not including) the next heading of *that same level* — so its
// body still carries every deeper heading nested inside it, verbatim, for
// whichever caller wants to split one level further.
//
// Splitting one level at a time — h2Sections splits doc at level 2, and
// h3Children splits one H2's own body at level 3 — is what keeps a `####
// Branches` from ever being mistaken for a section boundary the outer scan
// cares about, and what keeps a Pros-and-Cons `### Option N: …` from ever
// being mistaken for a new top-level section: neither scan ever looks past
// its own one level.
type heading struct {
	level int
	text  string
	body  []string // raw lines following this heading, until the next heading of the same level
}

// splitByLevel breaks lines into headings at exactly the given level. A
// heading at any other level — shallower or deeper — is left as an ordinary
// line inside whichever section is currently open, so it never opens or
// closes a section here.
func splitByLevel(lines []string, level int) []heading {
	var out []heading
	for _, line := range lines {
		if m := headingRe.FindStringSubmatch(line); m != nil && len(m[1]) == level {
			out = append(out, heading{level: level, text: m[2]})
			continue
		}
		if len(out) == 0 {
			continue // text before the first heading at this level carries no option
		}
		last := &out[len(out)-1]
		last.body = append(last.body, line)
	}
	return out
}

// h2Sections returns doc's top-level ("## ") sections, each carrying every
// deeper heading nested inside it as part of its own body.
func h2Sections(doc string) []heading {
	lines := strings.Split(strings.ReplaceAll(doc, "\r\n", "\n"), "\n")
	return splitByLevel(lines, 2)
}

// h3Children splits section's body into its level-3 subsections.
func h3Children(section heading) []heading {
	return splitByLevel(section.body, 3)
}

// optionTokenRe pulls the "Option N" / "N" identifier a lot of ADR corpora use
// to key an option across two sections, from the *start* of a string. It never
// consumes past the token — the rest of the label, including any colon, is
// left untouched (the colon-split bug this file exists not to repeat).
var optionTokenRe = regexp.MustCompile(`(?i)^"?(?:option\s+)?(\d+[a-z]?)\b`)

// optionToken returns the leading "Option N[letter]" identifier of s,
// lower-cased, and whether one was found.
func optionToken(s string) (string, bool) {
	m := optionTokenRe.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return "", false
	}
	return strings.ToLower(m[1]), true
}

// stripLeadingToken removes a leading "Option N[letter]" (or bare "N[letter]")
// token, and the punctuation after it, from s — for text comparison, never for
// building the stored Option label, which always keeps the token attached.
var stripLeadingTokenRe = regexp.MustCompile(`(?i)^"?(?:option\s+)?\d+[a-z]?[:.)-]?\s*`)

// markdownLinkRe turns a markdown link into its own link text, so "Use
// [(M)ADR documents](https://adr.github.io/madr/)" compares as "Use (M)ADR
// documents" — the words a "Chosen option" line actually quotes back never
// carry the URL.
var markdownLinkRe = regexp.MustCompile(`\[([^\]]*)\]\([^)]*\)`)

func normalizedText(s string) string {
	s = markdownLinkRe.ReplaceAllString(s, "$1")
	s = stripLeadingTokenRe.ReplaceAllString(strings.TrimSpace(s), "")
	s = strings.ToLower(s)
	s = strings.Trim(s, `"' `)
	return s
}

// sameOption reports whether a and b name the same option, by token when both
// carry one and by normalised text otherwise. It is deliberately generous
// (equality or containment) rather than exact, because a real corpus's own
// "Chosen option" line does not always describe an option in exactly the words
// its own heading used.
func sameOption(a, b string) bool {
	if ta, ok := optionToken(a); ok {
		if tb, ok := optionToken(b); ok {
			return ta == tb
		}
	}
	na, nb := normalizedText(a), normalizedText(b)
	if na == "" || nb == "" {
		return false
	}
	return na == nb || strings.Contains(na, nb) || strings.Contains(nb, na)
}

// chosenOutcomeRe pulls the label out of a "Chosen option: X" / "Chosen
// options: X" line, in whichever of the corpus's own phrasings it appears.
var chosenOutcomeRe = regexp.MustCompile(`(?i)^chosen options?:?\s+(.+?)\s*$`)

// chosenLabels returns every label a "## Decision Outcome" (or "## Decision")
// section names as chosen, and whether any were found at all. Almost always
// one, from a single "Chosen option: X" line — every corpus in this lane's
// fixtures states it that way. A handful of bbc/tams's own multi-axis
// decisions instead write a bare "Chosen options:" with the axes as a bullet
// list underneath and nothing on the header line itself; those are read as
// having no machine-parseable chosen label at all (hasChosen false for that
// document) rather than guessed at, which is the more conservative failure —
// this package would rather under-exclude a chosen option on a document shape
// it cannot parse than mis-exclude an option that was never chosen.
func chosenLabels(sections []heading) ([]string, bool) {
	for _, s := range sections {
		lower := strings.ToLower(s.text)
		if !strings.Contains(lower, "decision outcome") && strings.TrimSpace(lower) != "decision" {
			continue
		}
		for _, line := range s.body {
			if m := chosenOutcomeRe.FindStringSubmatch(strings.TrimSpace(line)); m != nil {
				label := strings.TrimSuffix(m[1], ".")
				label = strings.TrimSpace(strings.SplitN(label, ", because", 2)[0])
				if label == "" {
					continue
				}
				return []string{label}, true
			}
		}
	}
	return nil, false
}

// reasonBulletRe matches a MADR pros/cons bullet: "* Good, because …", "* Bad,
// because …", "* Neutral, because …".
var reasonBulletRe = regexp.MustCompile(`(?i)^(Good|Bad|Neutral),?\s+because\s+(.*?)\s*$`)

// revisitRe finds a "Revisit if …" clause wherever it starts inside a reason,
// case-insensitively — the one field dec-0028's own measurement says no public
// corpus ever supplies, and the one this lane's controls (0003) still have to
// prove the extractor can find when a corpus does carry it.
var revisitRe = regexp.MustCompile(`(?i)\.?\s*revisit if\s+(.*)$`)

// splitRevisit pulls a trailing "Revisit if …" clause off reason, if present.
func splitRevisit(reason string) (whyNot, revisitIf string) {
	loc := revisitRe.FindStringSubmatchIndex(reason)
	if loc == nil {
		return strings.TrimSpace(reason), ""
	}
	whyNot = strings.TrimSpace(reason[:loc[0]])
	revisitIf = strings.TrimSpace(reason[loc[2]:loc[3]])
	return whyNot, revisitIf
}

// classify reports how substantial reason is.
func classify(reason string) Classification {
	reason = strings.TrimSpace(reason)
	if reason == "" {
		return ClassBare
	}
	if len(strings.Fields(reason)) <= thinMaxWords {
		return ClassThin
	}
	return ClassReasoned
}

// sectionBodyText joins h's body lines into one trimmed block of prose,
// dropping blank lines at each end but keeping internal structure (bullets,
// blank lines between paragraphs) so a reason reads like the source did.
func sectionBodyText(lines []string) string {
	// Trim leading/trailing blank lines only.
	start, end := 0, len(lines)
	for start < end && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	for end > start && strings.TrimSpace(lines[end-1]) == "" {
		end--
	}
	return strings.TrimSpace(strings.Join(lines[start:end], "\n"))
}

// Extract reads one document's raw markdown text and returns what it found.
// text is the document's whole content, frontmatter included — a leading YAML
// block (as `docs/adr/*.md` in bbc/tams carries, `status: "accepted"`) is not a
// heading and is skipped by construction, since headings() only recognises
// ATX `#` lines.
func Extract(text string) Document {
	sections := h2Sections(text)

	chosen, hasChosen := chosenLabels(sections)

	// foundLabels is the set of option identities already captured, so a
	// later pass over the same document never adds the same option twice —
	// that is what "bare" means (dec-0028's own footnote on tams' 29: "a
	// full descriptive name but no separate Pros-and-Cons block for the
	// join to find").
	var found []Alternative
	var foundLabels []string

	isChosen := func(label string) bool {
		if !hasChosen {
			return false
		}
		for _, c := range chosen {
			if sameOption(label, c) {
				return true
			}
		}
		return false
	}
	add := func(a Alternative) {
		found = append(found, a)
		foundLabels = append(foundLabels, a.Option)
	}

	// Pass 1: every section that carries its own reason — Pros-and-Cons
	// (shape 3's reasoned half), the inline house style (shape 1), and a
	// Considered-Options section that turns out to be shape 2 (a sub-heading
	// per option with prose underneath, no separate Pros-and-Cons anywhere).
	// This runs to completion, over every section, before pass 2 — a document
	// always writes "## Considered Options" before "## Pros and Cons of the
	// Options", and reading sections in that source order would let pass 2's
	// bare fallback capture an option pass 1 was always going to reason about
	// two sections later. foundLabels is what pass 2 checks against, so it has
	// to hold pass 1's complete answer first.
	for _, section := range sections {
		lower := strings.ToLower(section.text)

		switch {
		case strings.Contains(lower, "pros and cons"):
			for _, opt := range h3Children(section) {
				if isChosen(opt.text) {
					continue
				}
				reason := reasonFromProsAndCons(opt.body)
				whyNot, revisitIf := splitRevisit(reason)
				add(Alternative{
					Option:         opt.text,
					Reason:         whyNot,
					RevisitIf:      revisitIf,
					Classification: classify(whyNot),
				})
			}

		case strings.Contains(lower, "alternatives rejected") || strings.Contains(lower, "alternatives considered"):
			// Shape 1: kazi's own house style. Every bullet here is by
			// definition a rejected alternative — the heading already says
			// so — so there is no "chosen" to exclude.
			for _, bullet := range groupBullets(section.body) {
				option, reason := splitInlineBullet(bullet)
				whyNot, revisitIf := splitRevisit(reason)
				add(Alternative{
					Option:         option,
					Reason:         whyNot,
					RevisitIf:      revisitIf,
					Classification: classify(whyNot),
				})
			}

		case strings.Contains(lower, "considered options"):
			children := h3Children(section)
			if len(children) == 0 {
				continue // shape 3's bare half: pass 2's job
			}
			// Shape 2: Sylius — a sub-heading per option, prose underneath.
			for _, opt := range children {
				if isChosen(opt.text) || alreadyHave(foundLabels, opt.text) {
					continue
				}
				reason := sectionBodyText(opt.body)
				whyNot, revisitIf := splitRevisit(reason)
				add(Alternative{
					Option:         opt.text,
					Reason:         whyNot,
					RevisitIf:      revisitIf,
					Classification: classify(whyNot),
				})
			}
		}
	}

	// Pass 2: shape 3's bare half. A Considered-Options bullet with no H3
	// heading of its own (a flat bullet list, MADR's usual shape) becomes a
	// bare alternative unless pass 1 already reasoned about it or it is the
	// chosen option — "bare" is exactly "listed, no join found", and pass 1
	// has already answered "was a join found" for every option in the
	// document by the time this pass runs.
	for _, section := range sections {
		if !strings.Contains(strings.ToLower(section.text), "considered options") {
			continue
		}
		if len(h3Children(section)) > 0 {
			continue // shape 2 already handled this section in pass 1
		}
		for _, option := range groupBullets(section.body) {
			if option == "" || isChosen(option) || alreadyHave(foundLabels, option) {
				continue
			}
			add(Alternative{Option: option, Classification: ClassBare})
		}
	}

	// Shape 4 (no alternatives section at all) falls out of the above by
	// producing no sections that match any case, hence no alternatives.
	return Document{Alternatives: found}
}

// alreadyHave reports whether label names an option already captured, so the
// Considered-Options bare pass never duplicates one the Pros-and-Cons pass (or
// the Sylius pass) already added.
func alreadyHave(labels []string, label string) bool {
	for _, l := range labels {
		if sameOption(l, label) {
			return true
		}
	}
	return false
}

// reasonFromProsAndCons builds one option's reason from its Pros-and-Cons
// body: "Bad, because …" and "Neutral, because …" bullets first — the
// drawbacks, which are what actually explain a rejection — falling back to
// every "Good, because …" bullet if that is genuinely all the option carries,
// and falling back again to the section's own prose (0000's shape, which
// explains a rejected option in a paragraph with no bulleted pros/cons at
// all). Empty only when none of those exist — a genuinely reason-less
// subsection.
//
// A rejected option that carries only upsides in this corpus's own words is
// still not a bare name: the author wrote a paragraph about it, and "why not"
// is answered by the tradeoff the whole subsection records, not only by its
// negative half — the neutrality experiment's own count of non-empty reasons
// over bbc/tams (44 of 49 documents, 237 alternatives) is what this
// reproduces, and a rule that discards every all-upside subsection undercounts
// it.
func reasonFromProsAndCons(body []string) string {
	var bad, neutral, good []string
	for _, bullet := range groupBullets(body) {
		m := reasonBulletRe.FindStringSubmatch(bullet)
		if m == nil {
			continue
		}
		switch strings.ToLower(m[1]) {
		case "bad":
			bad = append(bad, m[2])
		case "neutral":
			neutral = append(neutral, m[2])
		case "good":
			good = append(good, m[2])
		}
	}
	switch {
	case len(bad) > 0:
		return strings.Join(bad, "; ")
	case len(neutral) > 0:
		return strings.Join(neutral, "; ")
	case len(good) > 0:
		// Every bullet found was "Good, because …": the option carries
		// nothing but upsides in this corpus's own accounting, which is not
		// a rejection reason — it is bare, not reasoned.
		return ""
	default:
		return sectionBodyText(body)
	}
}

// inlineSeparator is the label/reason delimiter for shape 1, kazi's own house
// style. It is an em dash deliberately, never a colon — see the package
// comment's second bug.
const inlineSeparator = " — "

// splitInlineBullet splits one "## Alternatives rejected" bullet into its
// option and reason. A bullet with no separator is a bare option with no
// reason at all (control 0001).
func splitInlineBullet(text string) (option, reason string) {
	text = strings.TrimPrefix(text, "**")
	before, after, ok := strings.Cut(text, inlineSeparator)
	if !ok {
		return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(before), "**")), ""
	}
	option = strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(before), "**"))
	reason = strings.TrimSpace(after)
	return option, reason
}

// LabelStats summarises how long a set of alternatives' option labels run, and
// whether they are short enough that label-expansion is needed before a citing
// matcher (dira check) could ever hope to match them — the one conditional
// repair this lane carries (qst-0003-neutrality.md §4).
type LabelStats struct {
	MedianWords    int
	NeedsExpansion bool
}

// needsExpansionMaxWords is the threshold qst-0003-neutrality.md §4 measured
// against: kazi's own house style (6% unmatchable) sits above it, tams (1%)
// and Sylius (0%) sit at it or below, so ≤2 is what separates "this corpus
// needs the repair" from "it does not".
const needsExpansionMaxWords = 2

// ComputeLabelStats reports the label-length diagnostic over alts. An empty
// slice is not needed — there is nothing to expand — which is what keeps the
// two named corpora vacuously NOT-NEEDED on the axis they have no labels to
// have an opinion about (meadow) or too few short ones to trip it (tams).
func ComputeLabelStats(alts []Alternative) LabelStats {
	if len(alts) == 0 {
		return LabelStats{}
	}
	lengths := make([]int, len(alts))
	for i, a := range alts {
		lengths[i] = len(strings.Fields(a.Option))
	}
	median := medianInt(lengths)
	return LabelStats{
		MedianWords:    median,
		NeedsExpansion: median <= needsExpansionMaxWords,
	}
}

// medianInt returns the median of a non-empty slice, without mutating it.
func medianInt(values []int) int {
	sorted := append([]int(nil), values...)
	for i := 1; i < len(sorted); i++ {
		for j := i; j > 0 && sorted[j-1] > sorted[j]; j-- {
			sorted[j-1], sorted[j] = sorted[j], sorted[j-1]
		}
	}
	n := len(sorted)
	if n%2 == 1 {
		return sorted[n/2]
	}
	return (sorted[n/2-1] + sorted[n/2]) / 2
}
