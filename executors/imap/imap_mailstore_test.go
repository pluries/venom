package imap

import (
	"fmt"
	"io"
	"net"
	"net/textproto"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
)

// mailStore is a tiny in-memory IMAP server: a few mailboxes holding messages, and just the commands the executor
// sends (LOGIN, CAPABILITY, STATUS, SELECT, FETCH, UID STORE, UID EXPUNGE, UID MOVE, CREATE, LIST, APPEND, LOGOUT).
// failOn lists verbs the server answers with NO, to reach the error paths of the executor.
type mailStore struct {
	mu     sync.Mutex
	boxes  map[string][]*storedMail
	next   uint32
	failOn map[string]bool
}

type storedMail struct {
	uid    uint32
	flags  []string
	header string
	body   string
}

// mailSession is one client connection: the store and the mailbox it selected.
type mailSession struct {
	store    *mailStore
	tp       *textproto.Conn
	selected string
}

type verbHandler func(s *mailSession, tag, args string) []string

var verbHandlers = map[string]verbHandler{
	"LOGIN":       func(_ *mailSession, tag, _ string) []string { return []string{tag + " OK LOGIN completed"} },
	"CAPABILITY":  (*mailSession).capability,
	"STATUS":      (*mailSession).status,
	"SELECT":      (*mailSession).selectBox,
	"FETCH":       (*mailSession).fetch,
	"UID STORE":   (*mailSession).store1,
	"UID EXPUNGE": (*mailSession).expunge,
	"EXPUNGE":     (*mailSession).expunge,
	"UID MOVE":    (*mailSession).move,
	"CREATE":      (*mailSession).create,
	"LIST":        (*mailSession).list,
	"APPEND":      (*mailSession).appendMsg,
}

const mailCaps = "IMAP4rev1 UIDPLUS MOVE X-TEST-LOGIN-OK"

func newMailStore(boxes map[string][]*storedMail) *mailStore {
	return &mailStore{boxes: boxes, next: 100, failOn: map[string]bool{}}
}

func mailOf(uid uint32, from, subject, body string, flags ...string) *storedMail {
	header := "From: " + from + "\r\nTo: rcpt@example.test\r\nSubject: " + subject +
		"\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n"
	return &storedMail{uid: uid, flags: flags, header: header, body: body}
}

// startMailStore serves store on 127.0.0.1 and returns its port.
func startMailStore(t *testing.T, store *mailStore) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	_, port, err := net.SplitHostPort(lis.Addr().String())
	require.NoError(t, err)
	var conns []net.Conn
	var sessions sync.WaitGroup
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			conn, err := lis.Accept()
			if err != nil {
				return
			}
			conns = append(conns, conn)
			sessions.Add(1)
			go func() {
				defer sessions.Done()
				store.serve(conn)
			}()
		}
	}()
	t.Cleanup(func() {
		lis.Close()
		<-done
		for _, c := range conns {
			c.Close()
		}
		sessions.Wait()
	})
	return port
}

func (st *mailStore) serve(conn net.Conn) {
	defer conn.Close()
	s := &mailSession{store: st, tp: textproto.NewConn(conn)}
	if err := s.tp.PrintfLine("* OK [CAPABILITY %s] venom mail store ready", mailCaps); err != nil {
		return
	}
	for {
		line, err := s.tp.ReadLine()
		if err != nil {
			return
		}
		replies, logout := s.answer(line)
		for _, r := range replies {
			if err := s.tp.PrintfLine("%s", r); err != nil {
				return
			}
		}
		if logout {
			return
		}
	}
}

// splitVerb returns the tag, the verb ("UID STORE" counts as one) and the rest of the line.
func splitVerb(line string) (tag, verb, args string) {
	tag, rest, _ := strings.Cut(line, " ")
	verb, args, _ = strings.Cut(rest, " ")
	verb = strings.ToUpper(verb)
	if verb == "UID" {
		second, more, _ := strings.Cut(args, " ")
		return tag, verb + " " + strings.ToUpper(second), more
	}
	return tag, verb, args
}

func (s *mailSession) answer(line string) (replies []string, logout bool) {
	tag, verb, args := splitVerb(line)
	if verb == "LOGOUT" {
		return []string{"* BYE bye", tag + " OK LOGOUT completed"}, true
	}
	h, ok := verbHandlers[verb]
	if !ok {
		return []string{tag + " BAD unknown command"}, false
	}
	s.store.mu.Lock()
	defer s.store.mu.Unlock()
	if s.store.failOn[verb] {
		return []string{tag + " NO " + verb + " refused"}, false
	}
	return h(s, tag, args), false
}

func unquote(s string) string { return strings.Trim(s, `"`) }

func (s *mailSession) capability(tag, _ string) []string {
	return []string{"* CAPABILITY " + mailCaps, tag + " OK CAPABILITY completed"}
}

func (s *mailSession) status(tag, args string) []string {
	name, _, _ := strings.Cut(args, " ")
	name = unquote(name)
	box, ok := s.store.boxes[name]
	if !ok {
		return []string{tag + " NO no such mailbox"}
	}
	return []string{
		fmt.Sprintf(`* STATUS "%s" (MESSAGES %d RECENT 0 UIDNEXT 200 UIDVALIDITY 1 UNSEEN 0)`, name, len(box)),
		tag + " OK STATUS completed",
	}
}

func (s *mailSession) selectBox(tag, args string) []string {
	name := unquote(args)
	box, ok := s.store.boxes[name]
	if !ok {
		return []string{tag + " NO no such mailbox"}
	}
	s.selected = name
	return []string{fmt.Sprintf("* %d EXISTS", len(box)), tag + " OK [READ-WRITE] SELECT completed"}
}

// fetch answers FETCH 1:*; reading the text marks the message \Seen, as a real server does.
func (s *mailSession) fetch(tag, _ string) []string {
	var out []string
	for i, m := range s.store.boxes[s.selected] {
		if !hasFlag(m.flags, `\Seen`) {
			m.flags = append(m.flags, `\Seen`)
		}
		out = append(out, fmt.Sprintf("* %d FETCH (UID %d FLAGS (%s) RFC822.HEADER {%d}\r\n%s RFC822.TEXT {%d}\r\n%s)",
			i+1, m.uid, strings.Join(m.flags, " "), len(m.header), m.header, len(m.body), m.body))
	}
	return append(out, tag+" OK FETCH completed")
}

func hasFlag(flags []string, f string) bool {
	for _, x := range flags {
		if x == f {
			return true
		}
	}
	return false
}

// matching returns the messages of the selected mailbox a sequence ("1:*" or one uid) designates.
func (s *mailSession) matching(seq string) []*storedMail {
	if strings.HasSuffix(seq, ":*") {
		return s.store.boxes[s.selected]
	}
	uid, err := strconv.Atoi(seq)
	if err != nil {
		return nil
	}
	var out []*storedMail
	for _, m := range s.store.boxes[s.selected] {
		if int(m.uid) == uid {
			out = append(out, m)
		}
	}
	return out
}

func (s *mailSession) store1(tag, args string) []string {
	f := strings.Fields(args)
	op := strings.ToUpper(f[1])
	flags := strings.Fields(strings.Trim(strings.Join(f[2:], " "), "()"))
	for _, m := range s.matching(f[0]) {
		switch {
		case strings.HasPrefix(op, "+"):
			m.flags = append(m.flags, flags...)
		case strings.HasPrefix(op, "-"):
			m.flags = without(m.flags, flags)
		default:
			m.flags = append([]string{}, flags...)
		}
	}
	return []string{tag + " OK STORE completed"}
}

func without(flags, drop []string) []string {
	kept := []string{}
	for _, f := range flags {
		if !hasFlag(drop, f) {
			kept = append(kept, f)
		}
	}
	return kept
}

// silentExpunge, set in failOn, makes EXPUNGE answer OK without removing anything.
const silentExpunge = "SILENT EXPUNGE"

// hangUpPort listens on loopback and closes every connection at once: a server that never greets.
func hangUpPort(t *testing.T) string {
	t.Helper()
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	t.Cleanup(func() { lis.Close() })
	go func() {
		for {
			c, err := lis.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	_, port, err := net.SplitHostPort(lis.Addr().String())
	require.NoError(t, err)
	return port
}

func (s *mailSession) expunge(tag, _ string) []string {
	if s.store.failOn[silentExpunge] {
		return []string{tag + " OK EXPUNGE completed"}
	}
	kept := []*storedMail{}
	for _, m := range s.store.boxes[s.selected] {
		if !hasFlag(m.flags, `\Deleted`) {
			kept = append(kept, m)
		}
	}
	s.store.boxes[s.selected] = kept
	return []string{tag + " OK EXPUNGE completed"}
}

func (s *mailSession) move(tag, args string) []string {
	seq, dest, _ := strings.Cut(args, " ")
	dest = unquote(dest)
	if _, ok := s.store.boxes[dest]; !ok {
		return []string{tag + " NO no such mailbox"}
	}
	moved := s.matching(seq)
	s.store.boxes[s.selected] = without2(s.store.boxes[s.selected], moved)
	s.store.boxes[dest] = append(s.store.boxes[dest], moved...)
	return []string{tag + " OK MOVE completed"}
}

func without2(box, drop []*storedMail) []*storedMail {
	kept := []*storedMail{}
	for _, m := range box {
		skip := false
		for _, d := range drop {
			skip = skip || d == m
		}
		if !skip {
			kept = append(kept, m)
		}
	}
	return kept
}

func (s *mailSession) create(tag, args string) []string {
	name := unquote(args)
	if _, ok := s.store.boxes[name]; !ok {
		s.store.boxes[name] = nil
	}
	return []string{tag + " OK CREATE completed"}
}

func (s *mailSession) list(tag, _ string) []string {
	names := make([]string, 0, len(s.store.boxes))
	for n := range s.store.boxes {
		names = append(names, n)
	}
	sort.Strings(names)
	out := make([]string, 0, len(names)+1)
	for _, n := range names {
		out = append(out, fmt.Sprintf(`* LIST () "/" "%s"`, n))
	}
	return append(out, tag+" OK LIST completed")
}

// appendMsg answers APPEND mbox [(flags)] {n}: asks for the literal, reads it and stores it.
func (s *mailSession) appendMsg(tag, args string) []string {
	open, closing := strings.LastIndex(args, "{"), strings.LastIndex(args, "}")
	size, err := strconv.Atoi(args[open+1 : closing])
	if err != nil {
		return []string{tag + " BAD bad literal"}
	}
	name, rest, _ := strings.Cut(args[:open], " ")
	var flags []string
	if strings.Contains(rest, "(") {
		flags = strings.Fields(strings.Trim(strings.TrimSpace(rest), "()"))
	}
	if err := s.tp.PrintfLine("+ go ahead"); err != nil {
		return nil
	}
	raw := make([]byte, size)
	if _, err := io.ReadFull(s.tp.R, raw); err != nil {
		return nil
	}
	if _, err := s.tp.ReadLine(); err != nil { // the CRLF that ends the command
		return nil
	}
	head, body, _ := strings.Cut(string(raw), "\r\n\r\n")
	s.store.next++
	s.store.boxes[unquote(name)] = append(s.store.boxes[unquote(name)],
		&storedMail{uid: s.store.next, flags: flags, header: head + "\r\n\r\n", body: body})
	return []string{tag + " OK APPEND completed"}
}
