package sniff

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"strings"
)

// Scope is how much of a transcript one invocation looks at.
type Scope int

// The two scopes, and why there are exactly two.
//
// The Stop hook fires after every turn against a transcript that only ever
// grows, so a run that read the whole file would re-propose the same sentence on
// every turn for the rest of the session. LastTurn is therefore the default:
// what was said since the human last spoke. PreCompact is the other case — it
// fires once, immediately before the session's lossiest moment, and there the
// whole file is exactly the right scope.
const (
	LastTurn Scope = iota
	Whole
)

// A Segment is one piece of prose from a transcript, with its speaker.
type Segment struct {
	// Role is "user" or "assistant". A segment read from plain text carries
	// an empty role.
	Role string

	// Text is the prose. Tool calls and tool results never produce a
	// Segment, so nothing that reaches the matcher was machine output.
	Text string
}

// A transcript record, decoded only as far as the prose in it.
//
// Claude Code writes one JSON object per line carrying a good deal more than
// this — uuids, request ids, working directories, git branches, file-history
// deltas. None of it is read. What a capture tier needs is who spoke and what
// they said, and a decoder that reached for the rest would be a decoder that
// breaks when a field it never used is renamed.
type record struct {
	Type    string `json:"type"`
	Message *struct {
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
	} `json:"message"`
}

// A content block inside an assistant or user message.
type block struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// ParseTranscript reads a Claude Code transcript into its prose.
//
// The format is JSONL. Input that is not JSONL is not an error: it is returned
// as a single plain-text segment, so a human can pipe notes at `dira sniff` and
// a truncated or half-written transcript still yields whatever prose it holds
// rather than failing a hook.
//
// Three block types are dropped and never reach the matcher:
//
//	tool_use     the command an agent ran
//	tool_result  what it printed
//	thinking     reasoning the session did not say out loud
//
// The first two are the precision-and-privacy rule in the package comment: a
// command's output is where secrets and customer data actually live in a
// transcript, and it is never anybody's decision. The third is dropped because
// thinking is where options are weighed — it is the densest source of exactly
// the hypotheticals that must not stage.
func ParseTranscript(r io.Reader) ([]Segment, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading the transcript: %w", err)
	}
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, nil
	}

	if !looksLikeJSONL(data) {
		return []Segment{{Text: string(data)}}, nil
	}

	var out []Segment
	scanner := bufio.NewScanner(bytes.NewReader(data))
	// Transcript lines carry whole assistant turns and routinely exceed
	// bufio's 64KB default, which would truncate a turn mid-sentence and
	// silently change what the matcher saw.
	scanner.Buffer(make([]byte, 0, 64*1024), 8*1024*1024)

	for scanner.Scan() {
		line := bytes.TrimSpace(scanner.Bytes())
		if len(line) == 0 || line[0] != '{' {
			continue
		}
		var rec record
		if err := json.Unmarshal(line, &rec); err != nil {
			// A malformed line is skipped rather than fatal. A hook
			// that fails on one bad record captures nothing from a
			// session that is otherwise entirely readable.
			continue
		}
		if rec.Type != "user" && rec.Type != "assistant" {
			continue
		}
		if rec.Message == nil {
			continue
		}
		for _, text := range messageText(rec.Message.Content) {
			if strings.TrimSpace(text) == "" {
				continue
			}
			out = append(out, Segment{Role: rec.Type, Text: text})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("reading the transcript: %w", err)
	}
	return out, nil
}

// messageText pulls the prose out of a message's content, which is either a
// bare string (how a typed user message is stored) or a list of typed blocks.
func messageText(content json.RawMessage) []string {
	if len(content) == 0 {
		return nil
	}

	var s string
	if err := json.Unmarshal(content, &s); err == nil {
		return []string{s}
	}

	var blocks []block
	if err := json.Unmarshal(content, &blocks); err != nil {
		return nil
	}
	var out []string
	for _, b := range blocks {
		if b.Type != "text" {
			continue
		}
		out = append(out, b.Text)
	}
	return out
}

// looksLikeJSONL reports whether the input is a transcript rather than prose. It
// asks whether the first non-blank line is a JSON object, which is true of every
// Claude Code transcript and false of every note a human types.
func looksLikeJSONL(data []byte) bool {
	for _, line := range bytes.SplitN(data, []byte("\n"), 8) {
		line = bytes.TrimSpace(line)
		if len(line) == 0 {
			continue
		}
		if line[0] != '{' {
			return false
		}
		return json.Valid(line)
	}
	return false
}

// lastTurn narrows a transcript to what was said since the human last spoke.
//
// A transcript with no user message at all is returned whole: it is a session
// that has only just begun, or a fixture, and returning nothing would make the
// scope silently swallow the content.
func lastTurn(segments []Segment) []Segment {
	last := -1
	for i, s := range segments {
		if s.Role == "user" {
			last = i
		}
	}
	if last < 0 {
		return segments
	}
	return segments[last:]
}
