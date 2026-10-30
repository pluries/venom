package imap

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ovh/venom"
)

func inboxStore() *mailStore {
	return newMailStore(map[string][]*storedMail{
		"INBOX": {
			mailOf(1, "alice@example.test", "hello", "first body"),
			mailOf(2, "bob@example.test", "report", "second body", `\Flagged`),
		},
		"Empty":   nil,
		"Archive": nil,
	})
}

// runMail runs the commands against store and returns the result of each one.
func runMail(t *testing.T, store *mailStore, cmds ...map[string]any) []CommandResult {
	t.Helper()
	venom.InitTestLogger(t)
	step := venom.TestStep{
		"auth":     map[string]any{"host": "127.0.0.1", "port": startMailStore(t, store), "user": "u", "password": "p"},
		"commands": toAny(cmds),
	}
	got, err := Executor{}.Run(context.Background(), step)
	require.NoError(t, err)
	res, ok := got.(Result)
	require.True(t, ok)
	require.Len(t, res.Commands, len(cmds))
	return res.Commands
}

func toAny(cmds []map[string]any) []any {
	out := make([]any, len(cmds))
	for i, c := range cmds {
		out[i] = c
	}
	return out
}

func cmd(name, mailbox string, args map[string]any) map[string]any {
	return map[string]any{"name": name, "search": map[string]any{"mailbox": mailbox, "subject": "report"}, "args": args}
}

func TestIMAP_Fetch(t *testing.T) {
	res := runMail(t, inboxStore(),
		cmd("fetch", "INBOX", nil),
		cmd("fetch", "Empty", nil),
		cmd("fetch", "Nope", nil),
		map[string]any{"name": "fetch"},
	)
	assert.Empty(t, res[0].Err)
	assert.Equal(t, uint32(2), res[0].Mail.UID)
	assert.Equal(t, "bob@example.test", res[0].Mail.From)
	assert.Equal(t, "second body", res[0].Search.Body)
	assert.Contains(t, res[1].Err, "empty mailbox")
	assert.Contains(t, res[2].Err, "error while counting number of messages")
	assert.Contains(t, res[3].Err, "'mailbox' is a mandatory field")
}

func TestIMAP_FetchNotFound(t *testing.T) {
	c := cmd("fetch", "INBOX", nil)
	c["search"] = map[string]any{"mailbox": "INBOX", "subject": "absent"}
	res := runMail(t, inboxStore(), c)
	assert.Contains(t, res[0].Err, "mail not found")
}

func TestIMAP_FetchServerFailure(t *testing.T) {
	st := inboxStore()
	st.failOn["SELECT"] = true
	res := runMail(t, st, cmd("fetch", "INBOX", nil))
	assert.Contains(t, res[0].Err, "error while fetching messages")
}

func TestIMAP_Flag(t *testing.T) {
	res := runMail(t, inboxStore(),
		cmd("flag", "INBOX", map[string]any{"add": []any{`\Answered`}}),
		cmd("flag", "INBOX", map[string]any{"remove": []any{`\Flagged`}}),
		cmd("flag", "INBOX", map[string]any{"set": []any{`\Draft`}}),
	)
	for i, r := range res {
		assert.Empty(t, r.Err, i)
		assert.Equal(t, uint32(2), r.Mail.UID, i)
	}
	assert.Contains(t, res[0].Mail.Flags, `\Answered`)
	assert.NotContains(t, res[1].Mail.Flags, `\Flagged`)
	assert.ElementsMatch(t, []string{`\Draft`, `\Seen`}, res[2].Mail.Flags)
}

func TestIMAP_FlagErrors(t *testing.T) {
	st := inboxStore()
	st.failOn["UID STORE"] = true
	res := runMail(t, st,
		cmd("flag", "INBOX", map[string]any{"add": []any{"x"}}),
		cmd("flag", "INBOX", map[string]any{"remove": []any{"x"}}),
		cmd("flag", "INBOX", map[string]any{"set": []any{"x"}}),
		cmd("flag", "Empty", map[string]any{"add": []any{"x"}}),
		cmd("flag", "Empty", map[string]any{"remove": []any{"x"}}),
		cmd("flag", "Empty", map[string]any{"set": []any{"x"}}),
	)
	assert.Contains(t, res[0].Err, "add: error while calling UID Store")
	assert.Contains(t, res[1].Err, "remove: error while calling UID Store")
	assert.Contains(t, res[2].Err, "set: error while calling UID Store")
	assert.Contains(t, res[3].Err, "add: error while retrieving mail")
	assert.Contains(t, res[4].Err, "remove: error while retrieving mail")
	assert.Contains(t, res[5].Err, "set: error while retrieving mail")
}

func TestIMAP_Delete(t *testing.T) {
	st := inboxStore()
	res := runMail(t, st,
		cmd("delete", "INBOX", nil),
		cmd("delete", "Empty", nil),
	)
	assert.Empty(t, res[0].Err)
	assert.Equal(t, uint32(2), res[0].Search.UID)
	assert.Len(t, st.boxes["INBOX"], 1)
	assert.Contains(t, res[1].Err, "empty mailbox")
}

func TestIMAP_DeleteErrors(t *testing.T) {
	for _, verb := range []string{"UID STORE", "UID EXPUNGE"} {
		st := inboxStore()
		st.failOn[verb] = true
		res := runMail(t, st, cmd("delete", "INBOX", nil))
		assert.Contains(t, res[0].Err, "Error while", verb)
	}
}

func TestIMAP_DeleteNotConfirmed(t *testing.T) {
	st := inboxStore()
	st.failOn[silentExpunge] = true
	res := runMail(t, st, cmd("delete", "INBOX", nil))
	assert.Contains(t, res[0].Err, "Supposedly deleted mail was found")
}

func TestIMAP_Move(t *testing.T) {
	st := inboxStore()
	res := runMail(t, st,
		cmd("move", "INBOX", map[string]any{"mailbox": "Missing"}),
		cmd("move", "INBOX", map[string]any{"mailbox": "Archive"}),
		cmd("move", "Empty", map[string]any{"mailbox": "Archive"}),
	)
	assert.Empty(t, res[1].Err)
	assert.Len(t, st.boxes["Archive"], 1)
	assert.Contains(t, res[0].Err, "Error while selecting mailbox")
	assert.Contains(t, res[2].Err, "empty mailbox")
}

func TestIMAP_Create(t *testing.T) {
	st := inboxStore()
	res := runMail(t, st, cmd("create", "", map[string]any{"mailbox": "Fresh"}))
	assert.Empty(t, res[0].Err)
	assert.Contains(t, st.boxes, "Fresh")

	st.failOn["STATUS"] = true
	res = runMail(t, st, cmd("create", "", map[string]any{"mailbox": "Other"}))
	assert.Contains(t, res[0].Err, "Error while waiting mailbox status")
}

func TestIMAP_Clear(t *testing.T) {
	st := inboxStore()
	res := runMail(t, st, cmd("clear", "", map[string]any{"mailboxes": []any{"INBOX"}}))
	assert.Empty(t, res[0].Err)
	assert.Empty(t, st.boxes["INBOX"])

	st = inboxStore()
	res = runMail(t, st, cmd("clear", "", map[string]any{"mailboxes": []any{"*"}}))
	assert.Empty(t, res[0].Err)
	assert.Empty(t, st.boxes["INBOX"])
}

func TestIMAP_ClearErrors(t *testing.T) {
	for _, verb := range []string{"LIST", "SELECT", "UID STORE", "UID EXPUNGE"} {
		st := inboxStore()
		st.failOn[verb] = true
		res := runMail(t, st, cmd("clear", "", map[string]any{"mailboxes": []any{"*"}}))
		assert.NotEmpty(t, res[0].Err, verb)
	}
}

func TestIMAP_ClearWithoutMailboxes(t *testing.T) {
	for name, args := range map[string]map[string]any{
		"missing": {},
		"empty":   {"mailboxes": []any{}},
	} {
		res := runMail(t, inboxStore(), cmd("clear", "", args))
		assert.Contains(t, res[0].Err, "invalid clear args", name)
		assert.Contains(t, res[0].Err, "at least one mailbox", name)
	}
}

func TestIMAP_ClearLeavesMessages(t *testing.T) {
	st := inboxStore()
	st.failOn[silentExpunge] = true
	res := runMail(t, st, cmd("clear", "", map[string]any{"mailboxes": []any{"INBOX"}}))
	assert.Contains(t, res[0].Err, "2 message(s) were found in mailbox")
}

func TestIMAP_Append(t *testing.T) {
	st := inboxStore()
	args := map[string]any{
		"mailbox": "Archive", "from": "carol@example.test", "to": "rcpt@example.test",
		"subject": "appended", "body": "line one", "flags": []any{`\Flagged`},
	}
	res := runMail(t, st, cmd("append", "", args))
	assert.Empty(t, res[0].Err)
	assert.Equal(t, "appended", res[0].Mail.Subject)
	assert.Equal(t, "line one", res[0].Mail.Body)
	assert.Len(t, st.boxes["Archive"], 1)
}

func TestIMAP_AppendRefused(t *testing.T) {
	st := inboxStore()
	st.failOn["APPEND"] = true
	res := runMail(t, st, cmd("append", "", map[string]any{"mailbox": "Archive", "from": "a@example.test"}))
	assert.Contains(t, res[0].Err, "Error while appending message")
}

func TestIMAP_InvalidArgsAndUnknown(t *testing.T) {
	bad := map[string]any{"mailbox": 5, "mailboxes": 5, "add": 5, "from": 5}
	res := runMail(t, inboxStore(),
		cmd("append", "INBOX", bad),
		cmd("clear", "INBOX", bad),
		cmd("create", "INBOX", bad),
		cmd("flag", "INBOX", bad),
		cmd("move", "INBOX", bad),
		cmd("explode", "INBOX", nil),
	)
	for i, name := range []string{"append", "clear", "create", "flag", "move"} {
		assert.Contains(t, res[i].Err, "invalid "+name+" args", name)
	}
	assert.Equal(t, "unknown command", res[5].Err)
}

func TestIMAP_ConnectionRefused(t *testing.T) {
	venom.InitTestLogger(t)
	_, err := Executor{}.Run(context.Background(), venom.TestStep{
		"auth": map[string]any{"host": "127.0.0.1", "port": hangUpPort(t)},
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "unable to dial")
}

func TestIsSearched(t *testing.T) {
	m := func() *Mail {
		return &Mail{UID: 3, From: "a@example.test", To: "b@example.test", Subject: "Hello World", Body: "line\r\n\tone"}
	}
	tests := map[string]struct {
		crit SearchCriteria
		want bool
		err  bool
	}{
		"all match":      {SearchCriteria{UID: 3, From: "^a@", To: "b@", Subject: "Hello", Body: "line\none"}, true, false},
		"uid differs":    {SearchCriteria{UID: 4}, false, false},
		"from differs":   {SearchCriteria{From: "zzz"}, false, false},
		"to differs":     {SearchCriteria{To: "zzz"}, false, false},
		"subject differ": {SearchCriteria{Subject: "zzz"}, false, false},
		"body differs":   {SearchCriteria{Body: "zzz"}, false, false},
		"bad from":       {SearchCriteria{From: "("}, false, true},
		"bad to":         {SearchCriteria{To: "("}, false, true},
		"bad subject":    {SearchCriteria{Subject: "("}, false, true},
		"bad body":       {SearchCriteria{Body: "("}, false, true},
	}
	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := m().isSearched(tc.crit)
			assert.Equal(t, tc.want, got)
			assert.Equal(t, tc.err, err != nil)
		})
	}
}
