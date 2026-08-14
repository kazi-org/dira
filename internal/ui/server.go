package ui

import (
	"context"
	"embed"
	"errors"
	"fmt"
	"html/template"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/kazi-org/dira/internal/distill"
	"github.com/kazi-org/dira/internal/ledger"
	"github.com/kazi-org/dira/internal/why"
)

//go:embed templates/*.gohtml
var templates embed.FS

// A Server renders the read-only surfaces over HTTP.
//
// It owns no state a restart would lose and no port permanently: `dira ui` is a
// foreground process that stops when it stops (int-0002). There is no daemon,
// no background reindex, and no write path — every route here is a GET, and a
// read verb that mutated the ledger is how a derived-status product acquires
// stored status by accident.
type Server struct {
	src   Source
	store ledger.Store
	name  string
	tpl   *template.Template
	mux   *http.ServeMux
}

// NewServer builds the handler. name is what the ledger is called on the page —
// the repository the .dira/ directory sits in. store is the raw ledger the read
// surfaces (Source) cannot reach through: /distill calls internal/distill.Staged
// directly, and Staged takes a ledger.Store rather than an *index.Index
// (internal/index/index.go does not expose one — the index answers "which
// entries", never "read me this ledger raw", and the distill queue's whole job
// needs the second question).
func NewServer(src Source, store ledger.Store, name string) (*Server, error) {
	tpl, err := template.ParseFS(templates, "templates/*.gohtml")
	if err != nil {
		return nil, fmt.Errorf("ui: parsing templates: %w", err)
	}
	s := &Server{src: src, store: store, name: name, tpl: tpl, mux: http.NewServeMux()}

	s.mux.HandleFunc("/", s.route)
	for _, a := range []struct{ path, file string }{
		{"/tokens.css", "assets/tokens.css"},
		{"/decision.css", "assets/decision.css"},
		{"/index.css", "assets/index.css"},
		{"/distill.css", "assets/distill.css"},
	} {
		s.mux.HandleFunc(a.path, s.static(a.file, "text/css; charset=utf-8"))
	}

	// The fonts (dec-0016). The route set is derived from what is embedded
	// rather than typed out beside it, so a face compiled into the binary
	// cannot be left unreachable — which is the shape of the defect this
	// whole change is fixing, one level up.
	//
	// The path is "/assets/fonts/<name>" because that is what tokens.css's
	// relative url() resolves to from "/tokens.css", and the same relative
	// url() resolves to the real file when the mockups are read straight out
	// of the working tree. One string in the stylesheet, correct in both.
	fonts, err := Fonts()
	if err != nil {
		return nil, err
	}
	if len(fonts) == 0 {
		return nil, fmt.Errorf("ui: no fonts embedded; tokens.css asks for faces this binary does not carry")
	}
	for _, f := range fonts {
		s.mux.HandleFunc("/"+f, s.static(f, "font/woff2"))
	}
	return s, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) { s.mux.ServeHTTP(w, r) }

// route dispatches the page routes. It is hand-written rather than a
// third-party router for the same reason main.go's subcommand dispatch is:
// nothing in the command path may cost int-0002's budget, and this is one
// prefix test.
func (s *Server) route(w http.ResponseWriter, r *http.Request) {
	// /distill/<id>/<action> is checked first and unconditionally, ahead of
	// the read-only method gate below: it is the one family of routes that
	// takes a POST at all (T3, T4), and it enforces its own method (POST
	// only) rather than borrowing the GET-only refusal every other route
	// shares.
	if action, id, ok := distillActionPath(r.URL.Path); ok {
		s.distillAction(w, r, action, id)
		return
	}
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		s.fail(w, r, http.StatusMethodNotAllowed, "This surface is read-only.",
			"Every route here is a GET. dira's write path is the CLI and the hooks.")
		return
	}
	switch {
	case r.URL.Path == "/":
		s.index(w, r)
	case r.URL.Path == "/distill":
		s.distill(w, r)
	case strings.HasPrefix(r.URL.Path, "/e/"):
		s.decision(w, r, strings.TrimPrefix(r.URL.Path, "/e/"))
	default:
		s.fail(w, r, http.StatusNotFound, "No such page.",
			"The surfaces are the ledger index at /, a decision at /e/<id>, and the distill queue at /distill.")
	}
}

// distillActionPath recognizes "/distill/<id>/<action>" and splits it. It
// does not validate id or action — distillAction does, so that a malformed id
// and an unknown action are both refused the same way (a 404 that names what
// a valid one looks like) rather than one of them 404ing through route's
// default case with a different message.
func distillActionPath(path string) (action, id string, ok bool) {
	rest, found := strings.CutPrefix(path, "/distill/")
	if !found || rest == "" {
		return "", "", false
	}
	i := strings.LastIndex(rest, "/")
	if i <= 0 || i == len(rest)-1 {
		return "", "", false
	}
	return rest[i+1:], rest[:i], true
}

func (s *Server) index(w http.ResponseWriter, r *http.Request) {
	view, err := BuildIndex(r.Context(), s.src, s.name)
	if err != nil {
		s.oops(w, r, err)
		return
	}
	s.render(w, r, "index.gohtml", view)
}

// distill renders the deck. It is the read half only — no form action does
// anything yet; T3 and T4 wire those, over the same store this reads from.
func (s *Server) distill(w http.ResponseWriter, r *http.Request) {
	view, err := BuildDistill(r.Context(), s.store, s.name)
	if err != nil {
		s.oops(w, r, err)
		return
	}
	s.render(w, r, "distill.gohtml", view)
}

// distillAction is the write half of the deck: confirm and discard, both
// POST-only (T4 adds edit). It refuses a GET the same way the read routes
// refuse a POST — a 405, not a silent success — because the point of a form
// POST is that a page reload never resubmits it, and that only holds if the
// method is enforced.
func (s *Server) distillAction(w http.ResponseWriter, r *http.Request, action, id string) {
	if r.Method != http.MethodPost {
		s.fail(w, r, http.StatusMethodNotAllowed, "This action takes a form POST.",
			"Confirm and reject each answer a POST from the deck's own forms; nothing here answers a GET.")
		return
	}
	if !ledger.ValidID(id) {
		s.fail(w, r, http.StatusNotFound, "That is not an entry id.",
			"An id looks like dec-0001 — a kind prefix and four digits.")
		return
	}
	switch action {
	case "confirm":
		s.distillConfirm(w, r, id)
	case "discard":
		s.distillDiscard(w, r, id)
	default:
		s.fail(w, r, http.StatusNotFound, "No such action.",
			"The distill queue offers confirm and reject.")
	}
}

// distillEntry reads the entry a /distill action targets, translating
// ledger.ErrNotFound into the same 404 the read routes give a stranger's
// stale link.
func (s *Server) distillEntry(w http.ResponseWriter, r *http.Request, id string) (*ledger.Entry, bool) {
	entry, err := s.store.Get(r.Context(), id)
	if err != nil {
		if errors.Is(err, ledger.ErrNotFound) {
			s.fail(w, r, http.StatusNotFound, "No entry with that id.",
				"It may have been an id in another ledger, or a link written before the entry existed.")
			return nil, false
		}
		s.oops(w, r, err)
		return nil, false
	}
	return entry, true
}

// distillConfirm is `y`: POST /distill/<id>/confirm.
func (s *Server) distillConfirm(w http.ResponseWriter, r *http.Request, id string) {
	entry, ok := s.distillEntry(w, r, id)
	if !ok {
		return
	}
	if _, err := distill.Confirm(r.Context(), s.store, entry, time.Now()); err != nil {
		s.fail(w, r, http.StatusBadRequest, "That entry could not be confirmed.", err.Error())
		return
	}
	// 303, not 302: a reload of the response must re-issue the GET the
	// redirect points at, never resubmit the POST that got here.
	http.Redirect(w, r, "/distill", http.StatusSeeOther)
}

// distillDiscard is `n`: POST /distill/<id>/discard.
func (s *Server) distillDiscard(w http.ResponseWriter, r *http.Request, id string) {
	entry, ok := s.distillEntry(w, r, id)
	if !ok {
		return
	}
	if _, err := distill.Discard(r.Context(), s.store, entry); err != nil {
		s.fail(w, r, http.StatusBadRequest, "That entry could not be rejected.", err.Error())
		return
	}
	http.Redirect(w, r, "/distill", http.StatusSeeOther)
}

func (s *Server) decision(w http.ResponseWriter, r *http.Request, id string) {
	if id == "" || !ledger.ValidID(id) {
		// A malformed id is a 404 and not a 400: from a reader's side
		// "that is not an entry here" and "that is not an entry anywhere"
		// are the same answer, and the id came from a link they clicked.
		s.fail(w, r, http.StatusNotFound, "That is not an entry id.",
			"An id looks like dec-0001 — a kind prefix and four digits.")
		return
	}

	ctx := r.Context()
	entry, err := s.src.Entry(ctx, id)
	if err != nil {
		if errors.Is(err, ledger.ErrNotFound) {
			s.fail(w, r, http.StatusNotFound, "No entry with that id.",
				"It may have been an id in another ledger, or a link written before the entry existed.")
			return
		}
		s.oops(w, r, err)
		return
	}

	// The query is the id, because that is what the reader asked for. The
	// invocation line has to show what produced the page — a stranger
	// landing from a link has not typed anything (DESIGN.md's 3-second
	// interaction).
	chain, err := why.Build(ctx, s.src, id, id)
	if err != nil {
		s.oops(w, r, err)
		return
	}
	view, err := BuildDecision(ctx, s.src, chain, entry)
	if err != nil {
		s.oops(w, r, err)
		return
	}
	s.render(w, r, "decision.gohtml", view)
}

// render executes a template into a buffer before writing anything, so a
// template error cannot produce a 200 with half a page in it.
func (s *Server) render(w http.ResponseWriter, r *http.Request, name string, data any) {
	var b strings.Builder
	if err := s.tpl.ExecuteTemplate(&b, name, data); err != nil {
		s.oops(w, r, err)
		return
	}
	w.Header().Set("content-type", "text/html; charset=utf-8")
	// No caching. The ledger is files on disk that a hook rewrites mid
	// session; a cached decision page is a page arguing from a record that
	// has moved.
	w.Header().Set("cache-control", "no-store")
	_, _ = w.Write([]byte(b.String()))
}

// static serves one embedded byte blob. no-store for the same reason the pages
// carry it: a cache is a second copy of something this process is the only
// source of, and over loopback from memory there is nothing to save.
func (s *Server) static(file, contentType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		b, err := asset(file)
		if err != nil {
			s.oops(w, r, err)
			return
		}
		w.Header().Set("content-type", contentType)
		w.Header().Set("cache-control", "no-store")
		_, _ = w.Write(b)
	}
}

// oops is the 500. It says what failed, because the reader is the person who
// owns the ledger and a stack trace helps them less than a sentence does.
func (s *Server) oops(w http.ResponseWriter, r *http.Request, err error) {
	s.fail(w, r, http.StatusInternalServerError, "dira could not read the ledger.", err.Error())
}

// fail renders an error as a real page rather than as bare text. A 404 that is
// a stack trace on the surface a stranger lands on is a page that says the tool
// is broken.
func (s *Server) fail(w http.ResponseWriter, r *http.Request, code int, headline, detail string) {
	w.Header().Set("content-type", "text/html; charset=utf-8")
	w.WriteHeader(code)
	var b strings.Builder
	if err := s.tpl.ExecuteTemplate(&b, "error.gohtml", struct {
		Code     int
		Headline string
		Detail   string
	}{code, headline, detail}); err != nil {
		_, _ = fmt.Fprintf(w, "<!doctype html><title>%d</title><p>%s", code, headline)
		return
	}
	_, _ = w.Write([]byte(b.String()))
}

// ErrNotLoopback is what a request to bind anything but the loopback interface
// gets.
//
// cst-0004 says dira never requires a network service, and int-0002 says it owns
// no port permanently. A `dira ui` reachable from the LAN is a ledger — often a
// private one (cst-0003) — published to a network nobody asked to publish it to,
// by a flag typed once. So the refusal is structural rather than documented.
var ErrNotLoopback = errors.New("dira ui binds loopback only")

// Listen opens a listener on addr after checking it is loopback. An empty addr
// means an ephemeral port on 127.0.0.1.
func Listen(addr string) (net.Listener, error) {
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, fmt.Errorf("ui: %q is not a host:port address: %w", addr, err)
	}
	if host == "" {
		// A bare ":8080" binds every interface, which is the accident
		// this check exists for.
		return nil, fmt.Errorf("%w: %q has no host, which binds every interface — use 127.0.0.1:%s (cst-0004)",
			ErrNotLoopback, addr, port)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		if host != "localhost" {
			return nil, fmt.Errorf("%w: %q is a hostname dira will not resolve; use 127.0.0.1 (cst-0004)",
				ErrNotLoopback, host)
		}
	} else if !ip.IsLoopback() {
		return nil, fmt.Errorf("%w: %q is not a loopback address, and a ledger served off this machine "+
			"is a ledger published by accident (cst-0004, cst-0003)", ErrNotLoopback, host)
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return nil, fmt.Errorf("ui: binding %s: %w", addr, err)
	}
	return ln, nil
}

// Serve runs the server until ctx is cancelled, then shuts it down and releases
// the port. A process that leaves its port held after Ctrl-C is a process that
// cannot be restarted, which on a foreground tool is the whole failure.
func Serve(ctx context.Context, ln net.Listener, h http.Handler) error {
	srv := &http.Server{Handler: h}
	done := make(chan error, 1)
	go func() { done <- srv.Serve(ln) }()

	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		if err := srv.Close(); err != nil {
			return fmt.Errorf("ui: closing: %w", err)
		}
		<-done
		return nil
	}
}
