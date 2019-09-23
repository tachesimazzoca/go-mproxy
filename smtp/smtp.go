package smtp

import (
	"bufio"
	"net"
	"net/textproto"
	"regexp"
	"strings"
)

type SMTPState struct {
	Hello      string
	ServerName string
	ClientName string
	ReturnTo   string
	Recipients []string
	Headers    []string
	Content    []byte
}

func (st *SMTPState) HasStarted() bool {
	return len(st.Hello) > 0
}

func (st *SMTPState) Reset() {
	st.ReturnTo = ""
	st.Recipients = make([]string, 0)
	st.Headers = make([]string, 0)
	st.Content = make([]byte, 0)
}

type SMTPConnection struct {
	reader    *textproto.Reader
	writer    *textproto.Writer
	smtpState *SMTPState
}

func NewSMTPConnection(conn net.Conn) *SMTPConnection {
	return &SMTPConnection{
		reader:    textproto.NewReader(bufio.NewReader(conn)),
		writer:    textproto.NewWriter(bufio.NewWriter(conn)),
		smtpState: &SMTPState{},
	}
}

func (smtpConn *SMTPConnection) State() *SMTPState {
	return smtpConn.smtpState
}

func (smtpConn *SMTPConnection) Send(msg ...string) error {
	for _, x := range msg {
		if err := smtpConn.writer.PrintfLine(x); err != nil {
			return err
		}
	}
	return nil
}

type SMTPCommand interface {
	Excecute(conn *SMTPConnection, s string) error
}

type HelloCommand struct {
}

func (cmd *HelloCommand) Execute(conn *SMTPConnection, s string) error {
	if conn.State().HasStarted() {
		return conn.Send("550 Session has started")
	}
	xs := strings.SplitN(strings.TrimSpace(s), " ", 2)
	if len(xs) < 2 {
		return conn.Send("550 Invalid syntax (EHLO|HELO) domain")
	}
	st := conn.State()
	st.Hello = xs[0]
	st.ClientName = xs[1]
	return conn.Send(
		"250-"+st.ServerName,
		"250-AUTH PLAIN",
		"250 HELP",
	)
}

var mailCommandPattern = regexp.MustCompile("^MAIL FROM: *<([^>]+)> *$")

type MailCommand struct {
}

func (cmd *MailCommand) Execute(conn *SMTPConnection, line string) error {
	if !conn.State().HasStarted() {
		return conn.Send("550 Session has not started yet.")
	}
	xs := mailCommandPattern.FindStringSubmatch(line)
	if xs == nil || len(xs) != 2 {
		return conn.Send("550 Invalid syntax MAIL FROM: <foo@example.net>")
	}
	conn.State().ReturnTo = xs[1]
	return conn.Send("250 OK")
}

var recipientCommandPattern = regexp.MustCompile("^RCPT TO: *<([^>]+)> *$")

type RecipientCommand struct {
}

func (cmd *RecipientCommand) Execute(conn *SMTPConnection, line string) error {
	if !conn.State().HasStarted() {
		return conn.Send("550 Session has not started yet.")
	}

	// TODO: Check if MAIL FROM is specified?

	xs := recipientCommandPattern.FindStringSubmatch(line)
	if xs == nil || len(xs) != 2 {
		return conn.Send("550 Invalid syntax RCPT TO: <foo@example.net>")
	}
	st := conn.State()
	st.Recipients = append(st.Recipients, xs[1])
	return conn.Send("250 OK")
}

type ResetCommand struct {
}

func (cmd *ResetCommand) Execute(conn *SMTPConnection, line string) error {
	conn.State().Reset()
	return conn.Send("250 OK")
}

type VerifyCommand struct {
}

func (cmd *VerifyCommand) Execute(conn *SMTPConnection, line string) error {
	return conn.Send("550 VRFY not supported")
}

type NopeCommand struct {
}

func (cmd *NopeCommand) Execute(conn *SMTPConnection, line string) error {
	return conn.Send("250 OK")
}
