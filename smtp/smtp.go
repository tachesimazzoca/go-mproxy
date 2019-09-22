package smtp

import (
	"bufio"
	"net"
	"net/textproto"
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

func (ss *SMTPState) HasStarted() bool {
	return len(ss.Hello) > 0
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
	conn.Send(
		"250-"+st.ServerName,
		"250-AUTH PLAIN",
		"250 HELP",
	)
	return nil
}

type MailCommand struct {
}

func (cmd *MailCommand) Execute(conn *SMTPConnection, line string) error {
	if !conn.State().HasStarted() {
		return conn.Send("550 Session has not started yet.")
	}
	conn.State().ReturnTo = line
	return nil
}

type RecipientCommand struct {
}

func (cmd *RecipientCommand) Execute(conn *SMTPConnection, line string) error {
	if !conn.State().HasStarted() {
		return conn.Send("550 Session has not started yet.")
	}
	st := conn.State()
	st.Recipients = append(st.Recipients, line)
	return nil
}
