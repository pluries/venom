package imap

import (
	"context"
	"encoding/base64"
	"mime"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/yesnault/go-imap/imap"

	"github.com/ovh/venom"
)

// Characterisation tests for extract (#2031): they pin what the decoder does today, odd parts
// included. The responses are built the way go-imap's parser hands them to fetchMails: a FETCH
// response whose third field is the attribute list, RFC822.HEADER and RFC822.TEXT as literals,
// UID as a number and FLAGS as a list of atoms. All addresses are synthetic.

// fetchResponse builds a "* 1 FETCH (<attrs>)" response.
func fetchResponse(attrs ...imap.Field) imap.Response {
	return imap.Response{Label: "FETCH", Fields: []imap.Field{uint32(1), "FETCH", attrs}}
}

func literal(s string) imap.Field {
	return imap.NewLiteral([]byte(s))
}

// headerBlock joins header lines the way RFC822.HEADER returns them: CRLF after each line and
// the blank line that closes the block.
func headerBlock(lines ...string) string {
	return strings.Join(lines, "\r\n") + "\r\n\r\n"
}

func headerOnly(lines ...string) imap.Response {
	return fetchResponse("RFC822.HEADER", literal(headerBlock(lines...)))
}

func mailResponse(header, text string) imap.Response {
	return fetchResponse("RFC822.HEADER", literal(header), "RFC822.TEXT", literal(text))
}

type extractCase struct {
	name    string
	rsp     imap.Response
	want    Mail
	wantErr string
}

func runExtractCases(t *testing.T, cases []extractCase) {
	t.Helper()
	venom.InitTestLogger(t)
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			got, err := extract(context.Background(), tt.rsp)
			if tt.wantErr != "" {
				require.EqualError(t, err, tt.wantErr)
				assert.Equal(t, Mail{}, got, "an error always comes with the zero Mail")
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestExtract_fetchAttributes(t *testing.T) {
	runExtractCases(t, []extractCase{
		{name: "no attribute gives the zero Mail", rsp: fetchResponse(), want: Mail{}},
		{name: "UID is read as a number", rsp: fetchResponse("UID", uint32(42)), want: Mail{UID: 42}},
		{name: "a UID that is not a number reads as zero", rsp: fetchResponse("UID", "42"), want: Mail{}},
		{
			name: "flags come back sorted",
			rsp:  fetchResponse("FLAGS", []imap.Field{`\Seen`, `\Answered`, "$Label1"}),
			want: Mail{Flags: []string{"$Label1", `\Answered`, `\Seen`}},
		},
		{name: "a single flag", rsp: fetchResponse("FLAGS", []imap.Field{`\Seen`}), want: Mail{Flags: []string{`\Seen`}}},
		{
			// D98: "()" minus its parentheses is "", and strings.Split("", " ") is [""].
			name: "an empty flag list gives one empty flag",
			rsp:  fetchResponse("FLAGS", []imap.Field{}),
			want: Mail{Flags: []string{""}},
		},
		{name: "a FLAGS value that is not a list gives one empty flag", rsp: fetchResponse("FLAGS", `\Seen`), want: Mail{Flags: []string{""}}},
		{
			name: "all attributes together",
			rsp: fetchResponse(
				"UID", uint32(7),
				"FLAGS", []imap.Field{`\Recent`},
				"RFC822.HEADER", literal(headerBlock("Subject: Hello", "From: alice@example.org", "To: bob@example.org", "Content-Type: text/plain")),
				"RFC822.TEXT", literal("Hi Bob\r\n"),
			),
			want: Mail{UID: 7, From: "alice@example.org", To: "bob@example.org", Subject: "Hello", Body: "Hi Bob", Flags: []string{`\Recent`}},
		},
	})
}

// D90: RFC822.TEXT is read through the message parsed from RFC822.HEADER; without the header the
// message is nil and extract panics instead of returning an error.
func TestExtract_textWithoutHeaderPanics(t *testing.T) {
	venom.InitTestLogger(t)
	assert.Panics(t, func() {
		got, err := extract(context.Background(), fetchResponse("RFC822.TEXT", literal("Hi Bob\r\n")))
		t.Errorf("extract returned instead of panicking: %v, %v", got, err)
	})
}

// D99: a response that is not a FETCH has no MessageInfo; extract dereferences it and panics.
func TestExtract_responseThatIsNotAFetchPanics(t *testing.T) {
	venom.InitTestLogger(t)
	assert.Panics(t, func() {
		got, err := extract(context.Background(), imap.Response{Label: "EXISTS", Fields: []imap.Field{uint32(3), "EXISTS"}})
		t.Errorf("extract returned instead of panicking: %v, %v", got, err)
	})
}

func TestExtract_headers(t *testing.T) {
	runExtractCases(t, []extractCase{
		{
			name: "plain Subject, From and To",
			rsp:  headerOnly("Subject: Weekly report", "From: Alice <alice@example.org>", "To: bob@example.org"),
			want: Mail{Subject: "Weekly report", From: "Alice <alice@example.org>", To: "bob@example.org"},
		},
		{
			name: "B-encoded UTF-8 Subject",
			rsp:  headerOnly("Subject: " + mime.BEncoding.Encode("UTF-8", "Café crème")),
			want: Mail{Subject: "Café crème"},
		},
		{
			name: "Q-encoded display name in From",
			rsp:  headerOnly("From: " + mime.QEncoding.Encode("UTF-8", "René") + " <rene@example.org>"),
			want: Mail{From: "René <rene@example.org>"},
		},
		{name: "ISO-8859-1 encoded word", rsp: headerOnly("Subject: =?ISO-8859-1?Q?caf=E9?="), want: Mail{Subject: "café"}},
		{name: "adjacent encoded words are joined", rsp: headerOnly("Subject: =?UTF-8?Q?a?= =?UTF-8?Q?b?="), want: Mail{Subject: "ab"}},
		{name: "a malformed encoded word stays literal", rsp: headerOnly("Subject: =?UTF-8?B?not base64!?="), want: Mail{Subject: "=?UTF-8?B?not base64!?="}},
		{
			// To and From are the raw header value (decoded words only), never parsed as addresses.
			name: "several To addresses stay one string",
			rsp:  headerOnly("To: " + mime.QEncoding.Encode("UTF-8", "Alïce") + " <alice@example.org>, bob@example.org"),
			want: Mail{To: "Alïce <alice@example.org>, bob@example.org"},
		},
		{name: "Cc is not extracted", rsp: headerOnly("To: alice@example.org", "Cc: bob@example.org, carol@example.org"), want: Mail{To: "alice@example.org"}},
		{name: "missing headers give empty fields", rsp: headerOnly("Message-ID: <1@example.org>"), want: Mail{}},
		{name: "lower-case header names are found", rsp: headerOnly("subject: hi", "from: alice@example.org"), want: Mail{Subject: "hi", From: "alice@example.org"}},
		{name: "a folded Subject is unfolded", rsp: headerOnly("Subject: a long", " subject"), want: Mail{Subject: "a long subject"}},
		{name: "Date in RFC 1123Z form is ignored", rsp: headerOnly("Subject: x", "Date: Mon, 02 Jan 2006 15:04:05 -0700"), want: Mail{Subject: "x"}},
		{name: "Date in obsolete RFC 850 form is ignored", rsp: headerOnly("Subject: x", "Date: Monday, 02-Jan-06 15:04:05 MST"), want: Mail{Subject: "x"}},
		{name: "an unparseable Date is ignored", rsp: headerOnly("Subject: x", "Date: yesterday"), want: Mail{Subject: "x"}},
		{
			name: "a header block without the closing blank line is accepted",
			rsp:  fetchResponse("RFC822.HEADER", literal("Subject: hi\r\n")),
			want: Mail{Subject: "hi"},
		},
		{name: "an unknown charset in Subject fails", rsp: headerOnly("Subject: =?KOI8-R?B?9MXT1A==?="), wantErr: `cannot decode Subject header: mime: unhandled charset "KOI8-R"`},
		{name: "an unknown charset in From fails", rsp: headerOnly("From: =?KOI8-R?B?9MXT1A==?= <alice@example.org>"), wantErr: `cannot decode From header: mime: unhandled charset "KOI8-R"`},
		{name: "an unknown charset in To fails", rsp: headerOnly("To: =?KOI8-R?B?9MXT1A==?= <bob@example.org>"), wantErr: `cannot decode To header: mime: unhandled charset "KOI8-R"`},
		{name: "a malformed header line fails", rsp: fetchResponse("RFC822.HEADER", literal("not a header line\r\n\r\n")), wantErr: `malformed header line: "not a header line"`},
		{name: "an empty header block fails", rsp: fetchResponse("RFC822.HEADER", literal("")), wantErr: "EOF"},
	})
}

func TestExtract_singlePartBody(t *testing.T) {
	plain := func(encoding, text string) imap.Response {
		return mailResponse(headerBlock("Content-Type: text/plain; charset=utf-8", "Content-Transfer-Encoding: "+encoding), text)
	}
	runExtractCases(t, []extractCase{
		{name: "7bit", rsp: plain("7bit", "Hello Alice\r\n"), want: Mail{Body: "Hello Alice"}},
		{name: "no transfer encoding", rsp: mailResponse(headerBlock("Content-Type: text/plain"), "Hello Alice\r\n"), want: Mail{Body: "Hello Alice"}},
		{name: "8bit", rsp: plain("8bit", "Grüße\r\n"), want: Mail{Body: "Grüße"}},
		{name: "binary", rsp: plain("binary", "raw\x00bytes\r\n"), want: Mail{Body: "raw\x00bytes"}},
		{name: "quoted-printable with a soft line break", rsp: plain("quoted-printable", "caf=C3=A9 cr=\r\n=C3=A8me\r\n"), want: Mail{Body: "café crème"}},
		{name: "base64", rsp: plain("base64", base64.StdEncoding.EncodeToString([]byte("Hello Bob\r\n"))), want: Mail{Body: "Hello Bob"}},
		{name: "base64 split over lines", rsp: plain("base64", "SGVs\r\nbG8g\r\n"), want: Mail{Body: "Hello "}},
		// D91: the transfer encoding is compared case-sensitively.
		{name: "upper-case BASE64 is not decoded", rsp: plain("BASE64", "SGVsbG8="), want: Mail{Body: "SGVsbG8="}},
		{name: "mixed-case Quoted-Printable is not decoded", rsp: plain("Quoted-Printable", "caf=C3=A9"), want: Mail{Body: "caf=C3=A9"}},
		{name: "an unknown transfer encoding is not decoded", rsp: plain("x-uuencode", "begin 644 x"), want: Mail{Body: "begin 644 x"}},
		{name: "HTML keeps its markup", rsp: mailResponse(headerBlock("Content-Type: text/html"), "<p>Hello <b>Alice</b></p>\r\n"), want: Mail{Body: "<p>Hello <b>Alice</b></p>"}},
		// D93: the charset parameter is never applied; the bytes come back as they are.
		{
			name: "ISO-8859-1 bytes are not converted",
			rsp:  mailResponse(headerBlock("Content-Type: text/plain; charset=ISO-8859-1", "Content-Transfer-Encoding: 8bit"), "caf\xe9\r\n"),
			want: Mail{Body: "caf\xe9"},
		},
		{
			name: "ISO-8859-1 quoted-printable is decoded but not converted",
			rsp:  mailResponse(headerBlock("Content-Type: text/plain; charset=ISO-8859-1", "Content-Transfer-Encoding: quoted-printable"), "caf=E9"),
			want: Mail{Body: "caf\xe9"},
		},
		{name: "only trailing CR and LF are trimmed", rsp: plain("7bit", "\r\n  Hello \r\n\r\n\n"), want: Mail{Body: "\r\n  Hello "}},
		{name: "a non-text type is returned raw", rsp: mailResponse(headerBlock("Content-Type: application/json"), `{"a":1}`), want: Mail{Body: `{"a":1}`}},
		// D94: no Content-Type means no body at all, although RFC 2045 defaults it to text/plain.
		{name: "no Content-Type gives an empty body", rsp: mailResponse(headerBlock("Subject: x"), "Hello Alice\r\n"), want: Mail{Subject: "x"}},
		// D95: only multipart/mixed and multipart/alternative are split.
		{
			name: "multipart/related is returned raw",
			rsp:  mailResponse(headerBlock("Content-Type: multipart/related; boundary=b"), "--b\r\nContent-Type: text/html\r\n\r\n<p>Hi</p>\r\n--b--\r\n"),
			want: Mail{Body: "--b\r\nContent-Type: text/html\r\n\r\n<p>Hi</p>\r\n--b--"},
		},
		{name: "invalid base64 fails", rsp: plain("base64", "@@@@"), wantErr: "illegal base64 data at input byte 0"},
		{name: "an invalid quoted-printable escape is kept as it is", rsp: plain("quoted-printable", "caf=ZZ"), want: Mail{Body: "caf=ZZ"}},
		{name: "a malformed Content-Type fails", rsp: mailResponse(headerBlock("Content-Type: text/plain; charset"), "x"), wantErr: "error while reading Content-Type:mime: invalid media parameter"},
	})
}

func TestExtract_multipartBody(t *testing.T) {
	multipart := func(contentType, text string) imap.Response {
		return mailResponse(headerBlock("Content-Type: "+contentType), text)
	}
	inner := "--b2\r\nContent-Type: text/plain\r\n\r\nHello Alice\r\n--b2\r\nContent-Type: text/html\r\n\r\n<p>Hello Alice</p>\r\n--b2--"
	nested := "--b1\r\nContent-Type: multipart/alternative; boundary=b2\r\n\r\n" + inner + "\r\n--b1\r\nContent-Type: text/plain\r\n\r\nsecond\r\n--b1--\r\n"
	emptyFirst := "--b1\r\nContent-Type: text/plain\r\n\r\n\r\n--b1\r\nContent-Type: text/plain\r\n\r\nsecond\r\n--b1--\r\n"
	runExtractCases(t, []extractCase{
		{
			name: "alternative returns the first part (plain)",
			rsp:  multipart("multipart/alternative; boundary=b1", "--b1\r\nContent-Type: text/plain\r\n\r\nHello Alice\r\n--b1\r\nContent-Type: text/html\r\n\r\n<p>Hello Alice</p>\r\n--b1--\r\n"),
			want: Mail{Body: "Hello Alice"},
		},
		{
			name: "alternative returns the first part (HTML)",
			rsp:  multipart("multipart/alternative; boundary=b1", "--b1\r\nContent-Type: text/html\r\n\r\n<p>Hello Alice</p>\r\n--b1\r\nContent-Type: text/plain\r\n\r\nHello Alice\r\n--b1--\r\n"),
			want: Mail{Body: "<p>Hello Alice</p>"},
		},
		{
			name: "mixed with an attachment returns the text part",
			rsp:  multipart("multipart/mixed; boundary=b1", "--b1\r\nContent-Type: text/plain\r\n\r\nSee attached\r\n--b1\r\nContent-Type: application/pdf\r\nContent-Disposition: attachment; filename=\"a.pdf\"\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERi0=\r\n--b1--\r\n"),
			want: Mail{Body: "See attached"},
		},
		// D92: a base64 part is not decoded (the multipart reader only decodes quoted-printable).
		{
			name: "an attachment first is returned still base64-encoded",
			rsp:  multipart("multipart/mixed; boundary=b1", "--b1\r\nContent-Type: application/pdf\r\nContent-Transfer-Encoding: base64\r\n\r\nJVBERi0=\r\n--b1\r\nContent-Type: text/plain\r\n\r\nSee attached\r\n--b1--\r\n"),
			want: Mail{Body: "JVBERi0="},
		},
		{
			name: "a quoted-printable part is decoded",
			rsp:  multipart("multipart/alternative; boundary=b1", "--b1\r\nContent-Type: text/plain; charset=utf-8\r\nContent-Transfer-Encoding: quoted-printable\r\n\r\ncaf=C3=A9\r\n--b1--\r\n"),
			want: Mail{Body: "café"},
		},
		{
			name: "a base64 text part is not decoded",
			rsp:  multipart("multipart/alternative; boundary=b1", "--b1\r\nContent-Type: text/plain\r\nContent-Transfer-Encoding: base64\r\n\r\nSGVsbG8gQWxpY2U=\r\n--b1--\r\n"),
			want: Mail{Body: "SGVsbG8gQWxpY2U="},
		},
		// D96: a nested multipart is not descended into; its raw MIME text is the body.
		{name: "a nested multipart returns the inner MIME text", rsp: multipart("multipart/mixed; boundary=b1", nested), want: Mail{Body: inner}},
		{
			name: "preamble and epilogue are skipped",
			rsp:  multipart("multipart/mixed; boundary=b1", "This is a multi-part message.\r\n--b1\r\nContent-Type: text/plain\r\n\r\nHello Alice\r\n--b1--\r\nepilogue\r\n"),
			want: Mail{Body: "Hello Alice"},
		},
		{
			name: "LF-only line endings",
			rsp:  multipart("multipart/mixed; boundary=b1", "--b1\nContent-Type: text/plain\n\nHello Alice\n--b1--\n"),
			want: Mail{Body: "Hello Alice"},
		},
		{
			name: "a quoted boundary with a space",
			rsp:  multipart(`multipart/mixed; boundary="=_b 1"`, "--=_b 1\r\nContent-Type: text/plain\r\n\r\nHello Alice\r\n--=_b 1--\r\n"),
			want: Mail{Body: "Hello Alice"},
		},
		{
			name: "a base64 multipart is decoded before it is split",
			rsp: mailResponse(
				headerBlock("Content-Type: multipart/mixed; boundary=b1", "Content-Transfer-Encoding: base64"),
				base64.StdEncoding.EncodeToString([]byte("--b1\r\nContent-Type: text/plain\r\n\r\nHello Alice\r\n--b1--\r\n")),
			),
			want: Mail{Body: "Hello Alice"},
		},
		// D96: an empty first part, or no part at all, falls back to the whole raw multipart text.
		{name: "an empty first part falls back to the raw body", rsp: multipart("multipart/mixed; boundary=b1", emptyFirst), want: Mail{Body: strings.TrimRight(emptyFirst, "\r\n")}},
		{name: "a close delimiter alone falls back to the raw body", rsp: multipart("multipart/mixed; boundary=b1", "--b1--\r\n"), want: Mail{Body: "--b1--"}},
		{name: "a body without any delimiter falls back to the raw body", rsp: multipart("multipart/mixed; boundary=b1", "hello\r\n"), want: Mail{Body: "hello"}},
		{name: "a missing boundary parameter falls back to the raw body", rsp: multipart("multipart/mixed", "--b1\r\n\r\nHello\r\n--b1--\r\n"), want: Mail{Body: "--b1\r\n\r\nHello\r\n--b1--"}},
	})
}
