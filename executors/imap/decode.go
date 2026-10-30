package imap

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"mime/quotedprintable"
	"net/mail"
	"strings"

	"github.com/yesnault/go-imap/imap"

	"github.com/ovh/venom"
)

func decodeHeader(msg *mail.Message, headerName string) (string, error) {
	dec := new(mime.WordDecoder)
	s, err := dec.DecodeHeader(msg.Header.Get(headerName))
	if err != nil {
		return msg.Header.Get(headerName), err
	}
	return s, nil
}

// extract tries to read the content of the mail contained in rsp
func extract(ctx context.Context, rsp imap.Response) (Mail, error) {
	var (
		m   = Mail{}
		msg *mail.Message
		err error
	)

	if uidAttr, ok := rsp.MessageInfo().Attrs["UID"]; ok {
		m.UID = imap.AsNumber(uidAttr)
	}

	if headerAttr, ok := rsp.MessageInfo().Attrs["RFC822.HEADER"]; ok {
		msg, err = readHeaders(imap.AsBytes(headerAttr), &m)
		if err != nil {
			return Mail{}, err
		}
	}

	if textAttr, ok := rsp.MessageInfo().Attrs["RFC822.TEXT"]; ok {
		// msg is still nil when the response carries no RFC822.HEADER: readBody panics on it.
		m.Body, err = readBody(ctx, msg, imap.AsBytes(textAttr))
		if err != nil {
			return Mail{}, err
		}
	}

	if flagsAttr, ok := rsp.MessageInfo().Attrs["FLAGS"]; ok {
		flagSetStr := imap.AsFlagSet(flagsAttr).String()
		// Flags without embracing parenthesis
		m.Flags = strings.Split(flagSetStr[1:len(flagSetStr)-1], " ")
	}

	return m, nil
}

// readHeaders parses the RFC822 header block and decodes Subject, From and To into m.
func readHeaders(header []byte, m *Mail) (*mail.Message, error) {
	msg, err := mail.ReadMessage(bytes.NewReader(header))
	if err != nil {
		return nil, err
	}
	m.Subject, err = decodeHeader(msg, "Subject")
	if err != nil {
		return nil, fmt.Errorf("cannot decode Subject header: %s", err)
	}
	m.From, err = decodeHeader(msg, "From")
	if err != nil {
		return nil, fmt.Errorf("cannot decode From header: %s", err)
	}
	m.To, err = decodeHeader(msg, "To")
	if err != nil {
		return nil, fmt.Errorf("cannot decode To header: %s", err)
	}
	return msg, nil
}

// transferDecoder undoes the Content-Transfer-Encoding; the name is compared case-sensitively.
func transferDecoder(encoding string, r io.Reader) io.Reader {
	switch encoding {
	case "7bit", "8bit", "binary":
		// noop, reader already initialized.
	case "quoted-printable":
		r = quotedprintable.NewReader(r)
	case "base64":
		r = base64.NewDecoder(base64.StdEncoding, r)
	}
	return r
}

// readBody returns the text of the mail: the first readable part of a multipart/mixed or
// multipart/alternative body, the decoded body for any other content type, nothing without one.
func readBody(ctx context.Context, msg *mail.Message, body []byte) (string, error) {
	encoding := msg.Header.Get("Content-Transfer-Encoding")
	r := transferDecoder(encoding, bytes.NewReader(body))
	venom.Debug(ctx, "Mail Content-Transfer-Encoding is %s ", encoding)

	contentType, params, err := mime.ParseMediaType(msg.Header.Get("Content-Type"))
	// Using strings.Contains because "mime: no media type" error is not defined in mime package
	// and thus, cannot be used with errors.Is
	if err != nil && !strings.Contains(err.Error(), "mime: no media type") {
		return "", fmt.Errorf("error while reading Content-Type:%s", err)
	} else if err != nil && strings.Contains(err.Error(), "mime: no media type") {
		// Error "mime: no media type" is not a blocking error (returns when no Content-Type header is present)
		// But it means we cannot read body
		venom.Warn(ctx, "Content-Type header empty, skipping body reading")
	}
	if contentType == "" {
		return "", nil
	}

	var text string
	if contentType == "multipart/mixed" || contentType == "multipart/alternative" {
		if boundary, ok := params["boundary"]; ok {
			text = firstPart(ctx, r, boundary, err)
		}
	} else {
		body, err = io.ReadAll(r)
		if err != nil {
			return "", err
		}
		body = bytes.TrimRight(body, "\r\n")
	}
	if text == "" {
		// A multipart without a readable part falls back to the raw, still encoded, body.
		body = bytes.TrimRight(body, "\r\n")
		text = string(body)
	}
	return text, nil
}

// firstPart returns the first part of a multipart body that reads without error, or "" when none does.
// The debug lines log mediaErr, the Content-Type parsing error, not the part's own error.
func firstPart(ctx context.Context, r io.Reader, boundary string, mediaErr error) string {
	mr := multipart.NewReader(r, boundary)
	for {
		p, errm := mr.NextPart()
		if errm == io.EOF {
			continue
		}
		if errm != nil {
			venom.Debug(ctx, "Error while read Part:%s", mediaErr)
			break
		}
		slurp, errm := io.ReadAll(p)
		if errm != nil {
			venom.Debug(ctx, "Error while ReadAll Part:%s", mediaErr)
			continue
		}
		slurp = bytes.TrimRight(slurp, "\r\n")
		return string(slurp)
	}
	return ""
}
