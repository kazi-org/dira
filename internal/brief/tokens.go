package brief

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Tokens is dira's own token counter, and the whole of cst-0001's enforcement.
//
// # Why this is not a tokenizer
//
// cst-0001 caps the brief at 1,500 tokens and says the binary enforces it. dira
// may not call a model (dec-0003) and may not fetch anything at runtime
// (cst-0004, int-0002), so the alternatives were a vendored BPE table, an
// offline estimator, or a byte budget wearing the word "token". dec-0020 records
// the choice and what each option cost; the short version is that a vendored
// table is megabytes of vocabulary that is still only right for one vendor's
// tokenizer, and this is a guardrail rather than a billing meter.
//
// # What a dira token is
//
// One per line, plus, for each line, the greater of:
//
//   - its **word pieces** — a run of letters counts one piece per four
//     characters, a run of digits or punctuation one per character, and a
//     non-ASCII rune one per UTF-8 byte; and
//   - its **byte floor** — one per three bytes of the line with its whitespace
//     collapsed.
//
// Taking the greater of two independent estimates is the conservative part.
// Prose that packs into few words is caught by the byte floor; a dense line of
// ids and punctuation is caught by the word pieces. Both are calibrated to
// over-count against BPE tokenizers of the cl100k family: " constitutional" is
// one token there and four here, "dec-0002" is four there and six here.
//
// # The direction of the error is the requirement
//
// An estimator that is right on average is wrong low half the time, and a cap
// that is wrong low half the time is decorative. This one is wrong high: over
// this repository's own brief it counts 1,271 tokens for 3,637 bytes, which is
// 1.40x what the "about four characters per token" rule of thumb gives. That
// margin is the price of not shipping a vocabulary, and it is stated here rather
// than asserted by a test, because dira has no tokenizer to measure against —
// which is the whole reason this function exists (dec-0020).
//
// # Additivity, which the cap depends on
//
// Tokens(a + b) <= Tokens(a) + Tokens(b) whenever a ends in a newline. Every
// block the renderer measures does, which is what makes it safe to fill a budget
// block by block and still be under the ceiling when the pieces are concatenated
// — the whole is never more than the sum of what was paid for.
func Tokens(text string) int {
	if text == "" {
		return 0
	}

	total := 0
	for _, line := range strings.Split(text, "\n") {
		// One per line for the break itself and the indentation in front
		// of it, which BPE encodes as a single token far more often than
		// not, and which the byte floor below therefore does not count.
		total++

		words := strings.Fields(line)
		if len(words) == 0 {
			continue
		}

		pieces, content := 0, len(words)-1
		for _, word := range words {
			pieces += wordPieces(word)
			content += len(word)
		}
		if floor := (content + 2) / 3; floor > pieces {
			pieces = floor
		}
		total += pieces
	}
	return total
}

// runeClass is how a rune tokenizes, which is not the same question as what it
// means: letters merge into word pieces, digits and punctuation mostly do not,
// and anything outside ASCII is charged by its bytes because a tokenizer that
// has never seen it falls back to exactly that.
type runeClass int

const (
	classNone runeClass = iota
	classLetter
	classDigit
	classOther
)

// lettersPerPiece is how many letters a BPE merge typically covers in English.
// Four is the number the "a token is about four characters" rule of thumb comes
// from, applied per run rather than per document so that a line of ids does not
// get priced like a line of prose.
const lettersPerPiece = 4

// wordPieces estimates the tokens one whitespace-delimited word costs.
func wordPieces(word string) int {
	pieces, run, class := 0, 0, classNone

	flush := func() {
		if run == 0 {
			return
		}
		switch class {
		case classLetter:
			pieces += (run + lettersPerPiece - 1) / lettersPerPiece
		default:
			// A digit and a punctuation mark each buy at most one
			// token and often exactly one; charging per character is
			// the upper bound and the cheap direction to be wrong in.
			pieces += run
		}
		run = 0
	}

	for _, r := range word {
		if r > unicode.MaxASCII {
			// Byte fallback is the worst case every BPE shares: an
			// unknown rune costs one token per UTF-8 byte. Charging
			// that makes the count an upper bound for the characters
			// dira actually draws — the dashes, arrows and marks in
			// its own output.
			flush()
			class = classNone
			pieces += utf8.RuneLen(r)
			continue
		}

		var c runeClass
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z':
			c = classLetter
		case r >= '0' && r <= '9':
			c = classDigit
		default:
			c = classOther
		}
		if c != class {
			flush()
			class = c
		}
		run++
	}
	flush()

	if pieces == 0 {
		return 1
	}
	return pieces
}
