package imap

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/mitchellh/mapstructure"
	"github.com/pkg/errors"
	"github.com/yesnault/go-imap/imap"

	"github.com/ovh/venom"
)

const (
	// Name for test imap
	Name = "imap"
	// imapClientTimeout represents the timeout for the IMAP client and, as a result, the timeout for the testcase
	imapClientTimeout = 5 * time.Second
)

var (
	// imapLogMask is the go-imap debug log mask of every client: nothing is logged, so the login command, which carries
	// the password, never reaches a log. It is applied through imap.DefaultLogMask (see init), never with SetLogMask.
	imapLogMask = imap.LogNone

	errMailNotFound = errors.New("mail not found")
	errEmptyMailbox = errors.New("empty mailbox")
)

// init sets the mask every new client starts with (#2326). connect used to call SetLogMask on the client right after
// the dial, and again after the login, while go-imap's receiver goroutine, started by the dial, reads the field: a data
// race the race detector reports. The library takes the mask of a new client from DefaultLogMask, so it is set here
// once, before any client or goroutine exists. Do not call SetLogMask on a connected client.
func init() {
	imap.DefaultLogMask = imapLogMask
}

// New returns a new Test Exec
func New() venom.Executor {
	return &Executor{}
}

// Executor represents a Test Executor
type Executor struct {
	Auth     AuthConfig `json:"auth" yaml:"auth"`
	Commands []Command  `json:"commands,omitempty" yaml:"commands,omitempty"`
}

// ZeroValueResult return an empty implementation of this executor result
func (Executor) ZeroValueResult() interface{} {
	return Result{}
}

// GetDefaultAssertions return default assertions for type imap
func (Executor) GetDefaultAssertions() *venom.StepAssertions {
	return &venom.StepAssertions{Assertions: []venom.Assertion{"result.err ShouldBeEmpty"}}
}

// Run execute TestStep of type exec
func (Executor) Run(ctx context.Context, step venom.TestStep) (interface{}, error) {
	var e Executor
	if err := mapstructure.Decode(step, &e); err != nil {
		return nil, err
	}

	e.Auth.warnInsecureTLS(ctx)
	workdir := venom.StringVarFromCtx(ctx, "venom.testsuite.workdir")
	c, err := e.connect(ctx, workdir, e.Auth.Host, e.Auth.Port, e.Auth.User, e.Auth.Password)
	if err != nil {
		return nil, fmt.Errorf("error while connecting: %w", err)
	}
	defer c.Logout(imapClientTimeout) //nolint:errcheck // deferred logout on exit; the command result is already decided
	client := &Client{c}

	result := Result{}
	result.Commands = e.handleCommands(ctx, client)

	return result, nil
}

// handleCommands executes every Command from Executor and returns their result
func (e Executor) handleCommands(ctx context.Context, c *Client) []CommandResult {
	results := make([]CommandResult, len(e.Commands))
	for i, command := range e.Commands {
		switch command.Name {
		case CommandAppend:
			args := commandAppendArgs{}
			if err := args.initFrom(command.Args); err != nil {
				results[i] = CommandResult{Err: fmt.Sprintf("invalid append args: %v", err)}
				break
			}
			results[i] = c.appendMail(ctx, args)
		case CommandClear:
			args := commandClearArgs{}
			if err := args.initFrom(command.Args); err != nil {
				results[i] = CommandResult{Err: fmt.Sprintf("invalid clear args: %v", err)}
				break
			}
			results[i] = c.clearMailboxes(ctx, args)
		case CommandCreate:
			args := commandCreateArgs{}
			if err := args.initFrom(command.Args); err != nil {
				results[i] = CommandResult{Err: fmt.Sprintf("invalid create args: %v", err)}
				break
			}
			results[i] = c.createMailbox(ctx, args)
		case CommandDelete:
			results[i] = c.deleteMail(ctx, command.Search)
		case CommandFetch:
			results[i] = c.fetchMail(ctx, command.Search)
		case CommandFlag:
			args := commandFlagArgs{}
			if err := args.initFrom(command.Args); err != nil {
				results[i] = CommandResult{Err: fmt.Sprintf("invalid flag args: %v", err)}
				break
			}
			results[i] = c.flagMail(ctx, command.Search, args)
		case CommandMove:
			args := commandMoveArgs{}
			if err := args.initFrom(command.Args); err != nil {
				results[i] = CommandResult{Err: fmt.Sprintf("invalid move args: %v", err)}
				break
			}
			results[i] = c.moveMail(ctx, command.Search, args)
		default:
			results[i].Err = "unknown command"
		}
		if results[i].Err != "" {
			venom.Debug(ctx, "Failed to handle command %s: %s", command.Name, results[i].Err)
		}
	}
	return results
}

func (m *Mail) isSearched(mailToFind SearchCriteria) (bool, error) {
	var (
		matched bool
		err     error
	)
	if mailToFind.UID != 0 {
		if mailToFind.UID != m.UID {
			return false, nil
		}
	}
	if mailToFind.From != "" {
		matched, err = regexp.MatchString(mailToFind.From, m.From)
		if err != nil || !matched {
			return false, err
		}
	}
	if mailToFind.To != "" {
		matched, err = regexp.MatchString(mailToFind.To, m.To)
		if err != nil || !matched {
			return false, err
		}
	}
	if mailToFind.Subject != "" {
		matched, err = regexp.MatchString(mailToFind.Subject, m.Subject)
		if err != nil || !matched {
			return false, err
		}
	}
	if mailToFind.Body != "" {
		// Remove all new line, return and tab characters that can make the match fail
		replacer := strings.NewReplacer("\n", "", "\r", "", "\t", "")
		m.Body = replacer.Replace(m.Body)
		mailToFind.Body = replacer.Replace(mailToFind.Body)
		matched, err = regexp.MatchString(mailToFind.Body, m.Body)
		if err != nil || !matched {
			return false, err
		}
	}
	return true, nil
}
