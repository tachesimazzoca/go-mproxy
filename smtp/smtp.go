package smtp

import (
	"bufio"
	"fmt"
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

func (st *SMTPState) String() string {
	s := ""
	s += fmt.Sprintf("MAIL FROM: <%s>\r\n", st.ReturnTo)
	for _, x := range st.Recipients {
		s += fmt.Sprintf("RCPT TO: <%s>\r\n", x)
	}
	s += "DATA\r\n"
	for _, x := range st.Headers {
		s += fmt.Sprintf("%s\r\n", x)
	}
	s += "\r\n"
	s += string(st.Content)
	return s
}

type SMTPConnection struct {
	handler   *SMTPHandler
	reader    *textproto.Reader
	writer    *textproto.Writer
	smtpState *SMTPState
}

func NewSMTPConnection(h *SMTPHandler) *SMTPConnection {
	return &SMTPConnection{
		handler:   h,
		reader:    textproto.NewReader(bufio.NewReader(h.Conn())),
		writer:    textproto.NewWriter(bufio.NewWriter(h.Conn())),
		smtpState: &SMTPState{},
	}
}

func (smtpConn *SMTPConnection) State() *SMTPState {
	return smtpConn.smtpState
}

func (smtpConn *SMTPConnection) ReadLine() (string, error) {
	return smtpConn.reader.ReadLine()
}

func (smtpConn *SMTPConnection) ReadDotLines() ([]string, error) {
	return smtpConn.reader.ReadDotLines()
}

func (smtpConn *SMTPConnection) Send(msg ...string) error {
	for _, x := range msg {
		if err := smtpConn.writer.PrintfLine(x); err != nil {
			return err
		}
	}
	return nil
}

func (smtpConn *SMTPConnection) Quit() error {
	return smtpConn.handler.Close()
}

type SMTPCommand interface {
	Execute(conn *SMTPConnection, s string) error
}

type HelloCommand struct {
}

func (cmnd *HelloCommand) Execute(conn *SMTPConnection, s string) error {
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

func (cmnd *MailCommand) Execute(conn *SMTPConnection, line string) error {
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

func (cmnd *RecipientCommand) Execute(conn *SMTPConnection, line string) error {
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

func (cmnd *ResetCommand) Execute(conn *SMTPConnection, line string) error {
	conn.State().Reset()
	return conn.Send("250 OK")
}

type VerifyCommand struct {
}

func (cmnd *VerifyCommand) Execute(conn *SMTPConnection, line string) error {
	return conn.Send("550 VRFY not supported")
}

type NoopCommand struct {
}

func (cmnd *NoopCommand) Execute(conn *SMTPConnection, line string) error {
	return conn.Send("250 OK")
}

type QuitCommand struct {
}

func (cmnd *QuitCommand) Execute(conn *SMTPConnection, line string) error {
	if err := conn.Quit(); err != nil {
		return err
	}
	return conn.Send("221 Bye")
}

type DataCommand struct {
}

func (cmnd *DataCommand) Execute(conn *SMTPConnection, line string) error {
	var err error
	if err = conn.Send("250 OK"); err != nil {
		return err
	}
	lines, err := conn.ReadDotLines()
	if err != nil {
		return err
	}
	headers := make([]string, 0)
	content := make([]byte, 0)
	inBody := false
	for _, x := range lines {
		if !inBody && len(strings.TrimSpace(x)) == 0 {
			inBody = true
			continue
		}
		if inBody {
			content = append(content, []byte(x+"\r\n")...)
		} else {
			headers = append(headers, x)
		}
	}
	st := conn.State()
	st.Headers = headers
	st.Content = content
	return nil
}

type SMTPHandler struct {
	conn    net.Conn
	closing bool
}

var smtpCommandMap = map[string]SMTPCommand{
	"HELO": &HelloCommand{},
	"EHLO": &HelloCommand{},
	"MAIL": &MailCommand{},
	"RCPT": &RecipientCommand{},
	"RSET": &ResetCommand{},
	"VRFY": &VerifyCommand{},
	"NOOP": &NoopCommand{},
	"QUIT": &QuitCommand{},
	"DATA": &DataCommand{},
}

func NewSMTPHandler(conn net.Conn) *SMTPHandler {
	return &SMTPHandler{
		conn:    conn,
		closing: false,
	}
}

func (h *SMTPHandler) Conn() net.Conn {
	return h.conn
}

func (h *SMTPHandler) Run() error {
	defer h.Close()
	smtpConn := NewSMTPConnection(h)
	smtpConn.Send("220 Simple Mail Transfer service ready")
	for !h.closing {
		line, err := smtpConn.ReadLine()
		if err != nil {
			return err
		}
		xs := strings.SplitN(strings.TrimSpace(line), " ", 2)
		if len(xs) == 0 {
			if err := smtpConn.Send("550 Command must not be empty"); err != nil {
				return err
			}
		}
		if cmnd, ok := smtpCommandMap[xs[0]]; ok {
			if err := cmnd.Execute(smtpConn, line); err != nil {
				return err
			}
		} else {
			if err := smtpConn.Send("550 Command not recognized"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *SMTPHandler) Close() error {
	h.closing = true
	return h.conn.Close()
}
