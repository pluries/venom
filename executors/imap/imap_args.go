package imap

import "fmt"

type CommandName string

const (
	// CommandAppend creates a new message at the end of a specified mailbox.
	CommandAppend CommandName = "append"
	// CommandCreate creates a new mailbox.
	CommandCreate CommandName = "create"
	// CommandClear permanently removes all messages from given mailboxes.
	CommandClear CommandName = "clear"
	// CommandDelete permanently removes a message retrieved through Search field
	CommandDelete CommandName = "delete"
	// CommandFetch retrieves a mail through Search field (no args)
	CommandFetch CommandName = "fetch"
	// CommandFlag adds, removes or sets flags to a message retrieved through Search field
	CommandFlag CommandName = "flag"
	// CommandMove moves a message retrieved through Search field from one mailbox to another.
	CommandMove CommandName = "move"
)

// stringArg reads a string argument from a command's args map. It returns ("", nil) when the key
// is absent (the field then keeps its zero value, as before this helper existed), and an error
// when the key is present with a non-string value instead of silently truncating to "" or panicking.
func stringArg(args map[string]any, key string) (string, error) {
	v, ok := args[key]
	if !ok {
		return "", nil
	}
	s, ok := v.(string)
	if !ok {
		return "", fmt.Errorf("%q: expected a string, got %T", key, v)
	}
	return s, nil
}

// stringSliceArg is stringArg for a []any argument expected to hold only strings.
func stringSliceArg(args map[string]any, key string) ([]string, error) {
	v, ok := args[key]
	if !ok {
		return nil, nil
	}
	anySlice, ok := v.([]any)
	if !ok {
		return nil, fmt.Errorf("%q: expected a list, got %T", key, v)
	}
	strs := make([]string, len(anySlice))
	for i, item := range anySlice {
		s, ok := item.(string)
		if !ok {
			return nil, fmt.Errorf("%q[%d]: expected a string, got %T", key, i, item)
		}
		strs[i] = s
	}
	return strs, nil
}

// commandAppendArgs represents the arguments to the append command.
// from and mailbox are mandatory fields
type commandAppendArgs struct {
	mailbox string
	from    string
	to      string
	subject string
	body    string
	flags   []string
}

func (a *commandAppendArgs) initFrom(args map[string]any) error {
	var err error
	if a.mailbox, err = stringArg(args, "mailbox"); err != nil {
		return err
	}
	if a.from, err = stringArg(args, "from"); err != nil {
		return err
	}
	if a.to, err = stringArg(args, "to"); err != nil {
		return err
	}
	if a.subject, err = stringArg(args, "subject"); err != nil {
		return err
	}
	if a.body, err = stringArg(args, "body"); err != nil {
		return err
	}
	if a.flags, err = stringSliceArg(args, "flags"); err != nil {
		return err
	}
	return nil
}

// commandCreateArgs represents the arguments to the create command
type commandCreateArgs struct {
	// The name of the mailbox to create
	mailbox string
}

func (a *commandCreateArgs) initFrom(args map[string]any) error {
	var err error
	a.mailbox, err = stringArg(args, "mailbox")
	return err
}

// commandClearArgs represents the arguments to the clear command
type commandClearArgs struct {
	// The names of the mailboxes to clear, or "*" for all of them. At least one is mandatory
	mailboxes []string
}

func (a *commandClearArgs) initFrom(args map[string]any) error {
	var err error
	a.mailboxes, err = stringSliceArg(args, "mailboxes")
	if err == nil && len(a.mailboxes) == 0 {
		err = fmt.Errorf("\"mailboxes\": at least one mailbox is required")
	}
	return err
}

// commandFlagArgs represents the arguments to the flag command.
// One of add, remove or set fields is mandatory
type commandFlagArgs struct {
	// add new flags to the mail
	add []string
	// remove flags from the mail
	remove []string
	// set mail flags (overwrites current mail flags).
	// If set is not empty, add and remove fields will not be considered
	set []string
}

func (a *commandFlagArgs) initFrom(args map[string]any) error {
	var err error
	if a.add, err = stringSliceArg(args, "add"); err != nil {
		return err
	}
	if a.remove, err = stringSliceArg(args, "remove"); err != nil {
		return err
	}
	if a.set, err = stringSliceArg(args, "set"); err != nil {
		return err
	}
	return nil
}

// commandMoveArgs represents the arguments to the move command.
// mailbox is a mandatory field
type commandMoveArgs struct {
	// The name of the mailbox to move the mail to
	mailbox string
}

func (a *commandMoveArgs) initFrom(args map[string]any) error {
	var err error
	a.mailbox, err = stringArg(args, "mailbox")
	return err
}
