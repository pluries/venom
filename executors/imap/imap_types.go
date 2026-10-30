package imap

import (
	"slices"

	"github.com/yesnault/go-imap/imap"
)

// SearchCriteria represents the search criteria to fetch mails through FETCH command (https://www.rfc-editor.org/rfc/rfc3501#section-6.4.5)
type SearchCriteria struct {
	Mailbox string `json:"mailbox,omitempty" yaml:"mailbox,omitempty"`
	UID     uint32 `json:"uid,omitempty" yaml:"uid,omitempty"`
	From    string `json:"from,omitempty" yaml:"from,omitempty"`
	To      string `json:"to,omitempty" yaml:"to,omitempty"`
	Subject string `json:"subject,omitempty" yaml:"subject,omitempty"`
	Body    string `json:"body,omitempty" yaml:"body,omitempty"`
}

func (s *SearchCriteria) isAnyMandatoryFieldEmpty() bool {
	return s.Mailbox == ""
}

// Command represents a command that can be performed to messages or mailboxes.
type Command struct {
	// Search defines the search criteria to retrieve a mail. Some commands need to act upon a mail retrieved through Search.
	Search SearchCriteria `json:"search" yaml:"search"`
	// Name defines an IMAP command to execute.
	Name CommandName `json:"name" yaml:"name"`
	// Args defines the arguments to the command. Arguments associated to the command are listed in the README file
	Args map[string]any `json:"args" yaml:"args"`
}

type Mail struct {
	UID     uint32   `json:"uid,omitempty" yaml:"uid,omitempty"`
	From    string   `json:"from,omitempty" yaml:"from,omitempty"`
	To      string   `json:"to,omitempty" yaml:"to,omitempty"`
	Subject string   `json:"subject,omitempty" yaml:"subject,omitempty"`
	Body    string   `json:"body,omitempty" yaml:"body,omitempty"`
	Flags   []string `json:"flags,omitempty" yaml:"flags,omitempty"`
}

func (m *Mail) containsFlag(flag string) bool {
	for _, mFlag := range m.Flags {
		if flag == mFlag {
			return true
		}
	}
	return false
}

func (m *Mail) containsFlags(flags []string) bool {
	for _, flag := range flags {
		found := m.containsFlag(flag)
		if !found {
			return false
		}
	}
	return true
}

// containsAnyButSeenOrRecent returns true if m contains any flag in flags (except "\Seen" or "\Recent")
func (m *Mail) containsAnyButSeenOrRecent(flags []string) bool {
	for _, flag := range flags {
		if flag == `\Seen` || flag == `\Recent` {
			continue
		}
		found := m.containsFlag(flag)
		if found {
			return true
		}
	}
	return false
}

func (m *Mail) hasSameFlags(flags []string) bool {
	slices.Sort(m.Flags)
	slices.Sort(flags)
	return slices.Equal(m.Flags, flags)
}

// CommandResult contains the results of a command with the states of the mail before and after the command.
type CommandResult struct {
	// Search represents the result of the Command's search field
	Search Mail `json:"search,omitempty" yaml:"search,omitempty"`
	// Mail represents the state of the searched mail after the command was executed
	Mail        Mail    `json:"mail,omitempty" yaml:"mail,omitempty"`
	Err         string  `json:"err,omitempty" yaml:"err,omitempty"`
	TimeSeconds float64 `json:"timeseconds,omitempty" yaml:"timeseconds,omitempty"`
}

// Result represents a step result. It contains the results of every command that was executed
type Result struct {
	Commands []CommandResult `json:"commands,omitempty" yaml:"commands,omitempty"`
}

type Client struct {
	*imap.Client
}

type AuthConfig struct {
	WithTLS bool `json:"withtls,omitempty" yaml:"withtls,omitempty"`
	// VerifyTLS checks the server certificate against the system roots and Host when WithTLS is set. Unset keeps the
	// historical behaviour: the certificate is not verified.
	VerifyTLS bool   `json:"verifytls,omitempty" yaml:"verifytls,omitempty"`
	Host      string `json:"host,omitempty" yaml:"host,omitempty"`
	Port      string `json:"port,omitempty" yaml:"port,omitempty"`
	User      string `json:"user,omitempty" yaml:"user,omitempty"`
	Password  string `json:"password,omitempty" yaml:"password,omitempty"`
	// TLSRootCA is trusted on top of the system roots when VerifyTLS is set; TLSClientCert and TLSClientKey are the
	// client certificate, sent whether VerifyTLS is set or not. Each is PEM text or a file path relative to the suite
	// (ADR-0009). They are used only with WithTLS.
	TLSRootCA     string `json:"tls_root_ca" yaml:"tls_root_ca" mapstructure:"tls_root_ca"`
	TLSClientCert string `json:"tls_client_cert" yaml:"tls_client_cert" mapstructure:"tls_client_cert"`
	TLSClientKey  string `json:"tls_client_key" yaml:"tls_client_key" mapstructure:"tls_client_key"`
}
