package imap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/pkg/errors"
	"github.com/yesnault/go-imap/imap"

	"github.com/ovh/venom"
)

// clearMailboxes clears the given mailbox. If mailbox is '*', clears all mailboxes.
func (c *Client) clearMailboxes(ctx context.Context, args commandClearArgs) CommandResult {
	start := time.Now()
	result := CommandResult{}

	if args.mailboxes[0] == "*" {
		venom.Debug(ctx, "\"*\" argument found, clearing all mailboxes")

		cmd, err := imap.Wait(c.List("", "*"))
		if err != nil {
			return CommandResult{Err: fmt.Sprintf("Error while retrieving mailboxes: %v", err)}
		}
		mailboxes := make([]string, len(cmd.Data))
		for i, rsp := range cmd.Data {
			mailboxes[i] = rsp.MailboxInfo().Name
		}
		args.mailboxes = mailboxes
	}
	for _, mailbox := range args.mailboxes {
		err := c.clearMailbox(ctx, mailbox)
		if err != nil {
			return CommandResult{Err: err.Error()}
		}
	}

	result.TimeSeconds = time.Since(start).Seconds()
	venom.Debug(ctx, "Clear command executed in %.2f seconds", result.TimeSeconds)
	return result
}

func (c *Client) clearMailbox(ctx context.Context, mailbox string) error {
	venom.Debug(ctx, "Clearing mailbox %q", mailbox)

	seq, err := imap.NewSeqSet("1:*")
	if err != nil {
		return fmt.Errorf("error while building request to select all messages: %v", err)
	}

	venom.Debug(ctx, "Selecting mailbox %q", mailbox)
	if _, err = c.Select(mailbox, false); err != nil {
		return fmt.Errorf("error while selecting mailbox %q: %v", mailbox, err)
	}
	venom.Debug(ctx, "Adding tag '\\Deleted' to all messages")
	_, err = imap.Wait(c.UIDStore(seq, "+FLAGS.SILENT", imap.NewFlagSet(`\Deleted`)))
	if err != nil {
		return fmt.Errorf("error while adding flag '\\Deleted' to all messages in mailbox %q: %v", mailbox, err)
	}

	venom.Debug(ctx, "Expunging mailbox")
	if _, err = imap.Wait(c.Expunge(seq)); err != nil {
		return fmt.Errorf("error while expunging mails in mailbox %q: %v", mailbox, err)
	}

	// Command execution verifications
	venom.Debug(ctx, "Searching mails in mailbox %q to make sure they were all deleted", mailbox)
	count, err := c.countNumberOfMessagesInMailbox(mailbox)
	if err != nil {
		return fmt.Errorf("error while counting number of messages in mailbox %q after clear command: %v", mailbox, err)
	}
	if count > 0 {
		return fmt.Errorf("%d message(s) were found in mailbox %q", count, mailbox)
	}
	venom.Debug(ctx, "No message found in mailbox %q: clear command successfully completed!", mailbox)
	return nil
}

func (c *Client) appendMail(ctx context.Context, args commandAppendArgs) CommandResult {
	start := time.Now()
	result := CommandResult{}

	message := []string{
		fmt.Sprintf("From: %s", args.from),
		fmt.Sprintf("Subject: %s", args.subject),
		fmt.Sprintf("To: %s", args.to),
		"Content-Type: text/plain; charset=utf-8",
		"",
		args.body,
		"",
	}
	messageBytes := []byte(strings.Join(message, "\r\n"))
	literal := imap.NewLiteral(messageBytes)
	flags := imap.NewFlagSet(args.flags...)

	venom.Debug(ctx, "Appending message with fields: %+v", args)
	_, err := imap.Wait(c.Append(args.mailbox, flags, nil, literal))
	if err != nil {
		return CommandResult{Err: fmt.Sprintf("Error while appending message: %v", err)}
	}
	venom.Debug(ctx, "Successfully appended message!")

	// Command execution verifications
	venom.Debug(ctx, "Searching mail to make sure it was created")
	mail, err := c.getFirstFoundMail(ctx, SearchCriteria{
		Mailbox: args.mailbox,
		From:    args.from,
		To:      args.to,
		Subject: args.subject,
		Body:    args.body,
	})
	if err != nil {
		result.Err = fmt.Sprintf("Error while retrieving mail after append command was executed: %v", err)
		venom.Error(ctx, result.Err, nil)
	} else {
		venom.Debug(ctx, "Mail was retrieved: append command successfully completed")
		result.Mail = mail
	}

	result.TimeSeconds = time.Since(start).Seconds()
	venom.Debug(ctx, "Append command executed in %.2f seconds", result.TimeSeconds)
	return result
}

func (c *Client) createMailbox(ctx context.Context, args commandCreateArgs) CommandResult {
	start := time.Now()
	result := CommandResult{}

	venom.Debug(ctx, "Creating mailbox %q", args.mailbox)
	_, err := c.Create(args.mailbox)
	if err != nil {
		return CommandResult{Err: fmt.Sprintf("Error while creating mailbox: %v", err)}
	}
	venom.Debug(ctx, "Successfully created mailbox!")

	// Command execution verifications
	venom.Debug(ctx, "Selecting mailbox after command to make sure it exists")
	_, err = imap.Wait(c.Status(args.mailbox))
	if err != nil {
		return CommandResult{Err: fmt.Sprintf("Error while waiting mailbox status after create command was executed: %v", err)}
	}
	venom.Debug(ctx, "Mailbox was indeed created: create command successfully completed!")

	result.TimeSeconds = time.Since(start).Seconds()
	venom.Debug(ctx, "Create command executed in %.2f seconds", result.TimeSeconds)
	return result
}

func (c *Client) flagMail(ctx context.Context, mailToFind SearchCriteria, args commandFlagArgs) CommandResult {
	start := time.Now()
	result := CommandResult{}

	// As setting flags overrides any existing one, there is no need to add or remove flags if 'set' field is set
	if len(args.set) > 0 {
		result = c.setFlagsOfMail(ctx, mailToFind, args.set)
	} else {
		if len(args.add) > 0 {
			result = c.addFlagsToMail(ctx, mailToFind, args.add)
		}
		if len(args.remove) > 0 {
			result = c.removeFlagsFromMail(ctx, mailToFind, args.remove)
		}
	}

	result.TimeSeconds = time.Since(start).Seconds()
	venom.Debug(ctx, "Flag command executed in %.2f seconds", result.TimeSeconds)
	return result
}

// addFlagsToMail adds the flags contained in flags to the retrieved mail
func (c *Client) addFlagsToMail(ctx context.Context, mailToFind SearchCriteria, flags []string) CommandResult {
	result := CommandResult{}

	venom.Debug(ctx, "Searching mail with criteria: %+v", mailToFind)
	mail, err := c.getFirstFoundMail(ctx, mailToFind)
	if err != nil {
		return CommandResult{Err: fmt.Sprintf("add: error while retrieving mail: %v", err)}
	}
	result.Search = mail

	seq := &imap.SeqSet{} // equivalent to NewSeqSet(""), which always fails to parse "" and returns this same empty, still-usable set; AddNum below populates it directly
	seq.AddNum(mail.UID)

	venom.Debug(ctx, "Adding following flags from mail: %v", flags)
	_, err = imap.Wait(c.UIDStore(seq, "+FLAGS.SILENT", imap.NewFlagSet(flags...)))
	if err != nil {
		result.Err = fmt.Sprintf("add: error while calling UID Store command: %v", err)
		return result
	}
	venom.Debug(ctx, "Successfully added flags to mail!")

	// Making sure new flags are the expected ones
	mail, err = c.getFirstFoundMail(ctx, mailToFind)
	if err != nil {
		result.Err = fmt.Sprintf("add: error while retrieving mail: %v", err)
		return result
	}
	result.Mail = mail

	if !result.Mail.containsFlags(flags) {
		result.Err = fmt.Sprintf("add: mail has flags %v while it should contain flags %v", mail.Flags, flags)
		return result
	}

	return result
}

// removeFlagsFromMail removes the flags contained in flags from the retrieved mail
func (c *Client) removeFlagsFromMail(ctx context.Context, mailToFind SearchCriteria, flags []string) CommandResult {
	result := CommandResult{}

	venom.Debug(ctx, "Searching mail with criteria: %+v", mailToFind)
	mail, err := c.getFirstFoundMail(ctx, mailToFind)
	if err != nil {
		return CommandResult{Err: fmt.Sprintf("remove: error while retrieving mail: %v", err)}
	}
	result.Search = mail

	seq := &imap.SeqSet{} // equivalent to NewSeqSet(""), which always fails to parse "" and returns this same empty, still-usable set; AddNum below populates it directly
	seq.AddNum(mail.UID)

	venom.Debug(ctx, "Removing following flags from mail: %v", flags)
	_, err = imap.Wait(c.UIDStore(seq, "-FLAGS.SILENT", imap.NewFlagSet(flags...)))
	if err != nil {
		result.Err = fmt.Sprintf("remove: error while calling UID Store command: %v", err)
		return result
	}
	venom.Debug(ctx, "Successfully removed flags from mail!")

	// Making sure new flags are the expected ones
	mail, err = c.getFirstFoundMail(ctx, mailToFind)
	if err != nil {
		result.Err = fmt.Sprintf("remove: error while retrieving mail: %v", err)
		return result
	}
	result.Mail = mail

	// Fetching the mail and reading its text section added the "\Seen" flag.
	if result.Mail.containsAnyButSeenOrRecent(flags) {
		result.Err = fmt.Sprintf("remove: mail has flags %v while it should not have flags %v", mail.Flags, flags)
		return result
	}

	return result
}

// setFlagsOfMail sets the flags of the retrieved mail from flags
func (c *Client) setFlagsOfMail(ctx context.Context, mailToFind SearchCriteria, flags []string) CommandResult {
	result := CommandResult{}

	venom.Debug(ctx, "Searching mail with criteria: %+v", mailToFind)
	mail, err := c.getFirstFoundMail(ctx, mailToFind)
	if err != nil {
		return CommandResult{Err: fmt.Sprintf("set: error while retrieving mail: %v", err)}
	}
	result.Search = mail

	seq := &imap.SeqSet{} // equivalent to NewSeqSet(""), which always fails to parse "" and returns this same empty, still-usable set; AddNum below populates it directly
	seq.AddNum(mail.UID)

	venom.Debug(ctx, "Setting following mail flags: %v", flags)
	_, err = imap.Wait(c.UIDStore(seq, "FLAGS.SILENT", imap.NewFlagSet(flags...)))
	if err != nil {
		result.Err = fmt.Sprintf("set: error while calling UID Store command: %v", err)
		return result
	}
	venom.Debug(ctx, "Successfully set mail flags!")

	// Making sure new flags are the expected ones
	mail, err = c.getFirstFoundMail(ctx, mailToFind)
	if err != nil {
		result.Err = fmt.Sprintf("set: error while retrieving mail: %v", err)
		return result
	}
	result.Mail = mail

	// Fetching the mail and reading its text section added the "\Seen" flag.
	flags = append(flags, `\Seen`)
	if !result.Mail.hasSameFlags(flags) {
		result.Err = fmt.Sprintf("set: mail has flags %v while it should have flags %v", mail.Flags, flags)
		return result
	}

	return result
}

func (c *Client) fetchMail(ctx context.Context, mailToFind SearchCriteria) CommandResult {
	start := time.Now()
	result := CommandResult{}

	venom.Debug(ctx, "Searching mail with criteria: %+v", mailToFind)
	mail, err := c.getFirstFoundMail(ctx, mailToFind)
	if err != nil {
		return CommandResult{Err: fmt.Sprintf("error while retrieving mail: %v", err)}
	}
	venom.Debug(ctx, "Found mail: %+v", mail)
	// To avoid confusing the user about whether field to test, both are valid as the command does not modify the mail
	result.Search = mail
	result.Mail = mail

	result.TimeSeconds = time.Since(start).Seconds()
	venom.Debug(ctx, "Move command executed in %.2f seconds", result.TimeSeconds)
	return result
}

func (c *Client) deleteMail(ctx context.Context, mailToFind SearchCriteria) CommandResult {
	start := time.Now()
	result := CommandResult{}

	venom.Debug(ctx, "Searching mail with criteria: %+v", mailToFind)
	mail, err := c.getFirstFoundMail(ctx, mailToFind)
	if err != nil {
		return CommandResult{Err: fmt.Sprintf("error while retrieving mail: %v", err)}
	}
	venom.Debug(ctx, "Found mail: %+v", mail)
	result.Search = mail

	seq := &imap.SeqSet{} // equivalent to NewSeqSet(""), which always fails to parse "" and returns this same empty, still-usable set; AddNum below populates it directly
	seq.AddNum(mail.UID)

	venom.Debug(ctx, "Selecting mailbox %q", mailToFind.Mailbox)
	if _, err = c.Select(mailToFind.Mailbox, false); err != nil {
		result.Err = fmt.Sprintf("Error while selecting mailbox %q: %v", mailToFind.Mailbox, err)
		return result
	}
	venom.Debug(ctx, "Adding flag '\\Deleted' to mail")
	if _, err = imap.Wait(c.UIDStore(seq, "+FLAGS.SILENT", imap.NewFlagSet(`\Deleted`))); err != nil {
		result.Err = fmt.Sprintf("Error while adding '\\Deleted' flag to mail %d: %v", mail.UID, err)
		return result
	}
	venom.Debug(ctx, "Expunging mailbox %q", mailToFind.Mailbox)
	if _, err = imap.Wait(c.Expunge(seq)); err != nil {
		result.Err = fmt.Sprintf("Error while expunging mailbox %q: %v", mailToFind.Mailbox, err)
		return result
	}
	venom.Debug(ctx, "Mailbox successfully expunged")

	// Command execution verifications
	venom.Debug(ctx, "Searching mail to make sure it was deleted")
	mailToFind.UID = mail.UID
	mail, err = c.getFirstFoundMail(ctx, mailToFind)
	switch {
	case err != nil && (err == errMailNotFound || err == errEmptyMailbox):
		venom.Debug(ctx, "Mail was successfully deleted: delete command successfully completed!")
	case err == nil:
		result.Err = "Supposedly deleted mail was found after delete command was executed"
		venom.Error(ctx, result.Err, nil)
		result.Mail = mail
	default:
		result.Err = fmt.Sprintf("Error while trying to confirm mail was deleted: %v", err)
		venom.Warn(ctx, result.Err, nil)
	}

	result.TimeSeconds = time.Since(start).Seconds()
	venom.Debug(ctx, "Delete command executed in %.2f seconds", result.TimeSeconds)
	return result
}

func (c *Client) moveMail(ctx context.Context, mailToFind SearchCriteria, args commandMoveArgs) CommandResult {
	start := time.Now()
	result := CommandResult{}

	venom.Debug(ctx, "Searching mail with criteria: %+v", mailToFind)
	mail, err := c.getFirstFoundMail(ctx, mailToFind)
	if err != nil {
		return CommandResult{Err: fmt.Sprintf("error while retrieving mail: %v", err)}
	}
	venom.Debug(ctx, "Found mail: %+v", mail)
	result.Search = mail

	seq := &imap.SeqSet{} // equivalent to NewSeqSet(""), which always fails to parse "" and returns this same empty, still-usable set; AddNum below populates it directly
	seq.AddNum(mail.UID)

	venom.Debug(ctx, "Moving message to mailbox %q", mailToFind.Mailbox)
	if _, err = imap.Wait(c.UIDMove(seq, args.mailbox)); err != nil {
		return CommandResult{Err: fmt.Sprintf("Error while selecting mailbox %q: %v", mailToFind.Mailbox, err)}
	}
	venom.Debug(ctx, "Successfully moved message to mailbox %q", mailToFind.Mailbox)

	// Command execution verifications
	venom.Debug(ctx, "Searching mail to make sure it was moved")
	mailToFind.Mailbox = args.mailbox
	mail, err = c.getFirstFoundMail(ctx, mailToFind)
	if err != nil {
		result.Err = fmt.Sprintf("Error while trying to confirm mail was moved: %v", err)
		venom.Warn(ctx, result.Err, nil)
	}
	result.Mail = mail
	venom.Debug(ctx, "Mail was successfully moved: move command successfully completed!")

	result.TimeSeconds = time.Since(start).Seconds()
	venom.Debug(ctx, "Move command executed in %.2f seconds", result.TimeSeconds)
	return result
}

// getFirstFoundMail returns the first mail found through search criteria
// If peek is true, mail body text will not be read thus leaving flags unchanged (for more information, refer to RFC 3501 section 6.4.5)
func (c *Client) getFirstFoundMail(ctx context.Context, mailToFind SearchCriteria) (Mail, error) {
	if mailToFind.isAnyMandatoryFieldEmpty() {
		return Mail{}, errors.New("empty search criteria: 'mailbox' is a mandatory field")
	}

	count, err := c.countNumberOfMessagesInMailbox(mailToFind.Mailbox)
	if err != nil {
		return Mail{}, fmt.Errorf("error while counting number of messages in mailbox %q: %w", mailToFind.Mailbox, err)
	}
	if count == 0 {
		return Mail{}, errEmptyMailbox
	}

	messages, err := c.fetchMails(ctx, mailToFind.Mailbox)
	if err != nil {
		return Mail{}, fmt.Errorf("error while fetching messages in mailbox %q: %v", mailToFind.Mailbox, err)
	}

	for _, msg := range messages {
		mail, err := extract(ctx, msg)
		if err != nil {
			venom.Warn(ctx, "Cannot extract the content of the mail: %v", err)
			continue
		}

		// Handle the first found mail only
		found, err := mail.isSearched(mailToFind)
		if err != nil {
			return Mail{}, err
		}

		if found {
			return mail, nil
		}
	}

	return Mail{}, errMailNotFound
}

func (c *Client) fetchMails(ctx context.Context, mailbox string) ([]imap.Response, error) {
	venom.Debug(ctx, "Selecting mailbox %q", mailbox)
	if _, err := c.Select(mailbox, false); err != nil {
		venom.Error(ctx, "Error with select %s", err)
		return []imap.Response{}, err
	}

	seqset, err := imap.NewSeqSet("1:*")
	if err != nil {
		return []imap.Response{}, fmt.Errorf("error while building request to select all messages: %v", err)
	}

	messages := []imap.Response{}
	cmd, err := imap.Wait(c.Fetch(seqset, "UID", "ENVELOPE", "FLAGS", "RFC822.HEADER", "RFC822.TEXT", "BODY.PEEK[TEXT]"))
	if err != nil {
		venom.Error(ctx, "Error with FETCH command: %v", err)
		return []imap.Response{}, err
	}

	// Process command data
	for _, rsp := range cmd.Data {
		messages = append(messages, *rsp)
	}

	venom.Debug(ctx, "Fetched %d message(s) from mailbox %q", len(messages), mailbox)
	return messages, nil
}

func (c *Client) countNumberOfMessagesInMailbox(mailbox string) (uint32, error) {
	cmd, err := imap.Wait(c.Status(mailbox))
	if err != nil {
		return 0, err
	}

	var count uint32
	for _, result := range cmd.Data {
		mailboxStatus := result.MailboxStatus()
		if mailboxStatus != nil {
			count += mailboxStatus.Messages
		}
	}

	return count, nil
}
