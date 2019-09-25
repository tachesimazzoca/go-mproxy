package smtp

import (
	"net"
	"sync"
	"testing"
	"time"
)

type MockConn struct {
	readOffset   int
	inputBuffer  []byte
	outputBuffer []byte
	closed       bool
	mtx          sync.Mutex
}

func NewMockConn(rb []byte) *MockConn {
	return &MockConn{
		readOffset:   0,
		inputBuffer:  rb,
		outputBuffer: make([]byte, 0),
		closed:       false,
	}
}

func (mc *MockConn) Read(b []byte) (int, error) {
	defer mc.mtx.Unlock()
	bn := len(b)
	mc.mtx.Lock()
	rbn := len(mc.inputBuffer)
	for i := 0; i < bn; i++ {
		if mc.readOffset == rbn {
			return i, nil
		}
		b[i] = mc.inputBuffer[mc.readOffset]
		mc.readOffset++
	}
	return bn, nil
}

func (mc *MockConn) Write(b []byte) (int, error) {
	defer mc.mtx.Unlock()
	mc.mtx.Lock()
	for _, v := range b {
		mc.outputBuffer = append(mc.outputBuffer, v)
	}
	return len(b), nil
}

func (mc *MockConn) Close() error {
	mc.closed = true
	return nil
}

func (mc *MockConn) LocalAddr() net.Addr {
	return nil
}

func (mc *MockConn) RemoteAddr() net.Addr {
	return nil
}

func (mc *MockConn) SetDeadline(t time.Time) error {
	return nil
}

func (mc *MockConn) SetReadDeadline(t time.Time) error {
	return nil
}

func (mc *MockConn) SetWriteDeadline(t time.Time) error {
	return nil
}

func (mc *MockConn) ResetInputBuffer(b []byte) {
	defer mc.mtx.Unlock()
	mc.mtx.Lock()
	mc.readOffset = 0
	mc.inputBuffer = b
}

func (mc *MockConn) CloneOutputBuffer() []byte {
	dest := make([]byte, len(mc.outputBuffer))
	copy(dest, mc.outputBuffer)
	return dest
}

func (mc *MockConn) ResetOutputBuffer() {
	mc.outputBuffer = make([]byte, 0)
}

func (mc *MockConn) IsClosed() bool {
	return mc.closed
}

func TestSMTPStateString(t *testing.T) {
	st := SMTPState{
		ReturnTo:   "foo@example.net",
		Recipients: []string{"user1@example.net", "user2@example.net"},
		Headers: []string{
			"From: Foo<foo@example.net>",
			"To: User1<user1@example.net>",
			"Cc: User2<user2@example.net>",
			"Subject: Reveal SMTP State Stringer",
		},
		Content: []byte("This is a test message.\r\n" +
			"Are you sure?\r\n"),
	}
	expected := "MAIL FROM: <foo@example.net>\r\n" +
		"RCPT TO: <user1@example.net>\r\n" +
		"RCPT TO: <user2@example.net>\r\n" +
		"DATA\r\n" +
		"From: Foo<foo@example.net>\r\n" +
		"To: User1<user1@example.net>\r\n" +
		"Cc: User2<user2@example.net>\r\n" +
		"Subject: Reveal SMTP State Stringer\r\n" +
		"\r\n" +
		"This is a test message.\r\n" +
		"Are you sure?\r\n"

	actual := st.String()
	if expected != actual {
		t.Errorf("expected: %s, actual: %s", expected, actual)
	}
}

func TestSMTPConnectionSend(t *testing.T) {
	conn := NewMockConn([]byte{})
	smtpConn := NewSMTPConnection(NewSMTPHandler(conn))
	smtpConn.Send("220 Simple Mail Transfer Service ready")
	expected := "220 Simple Mail Transfer Service ready\r\n"
	actual := string(conn.CloneOutputBuffer())
	if actual != expected {
		t.Errorf("expected: %s, actual: %s", expected, actual)
	}
}

func TestHelloCommand(t *testing.T) {
	conn := NewMockConn([]byte{})
	smtpConn := NewSMTPConnection(NewSMTPHandler(conn))
	st := smtpConn.State()
	st.ServerName = "test-server"
	cmd := &HelloCommand{}
	cmd.Execute(smtpConn, "EHLO test-client")
	expected := "250-test-server\r\n" +
		"250-AUTH PLAIN\r\n" +
		"250 HELP\r\n"
	actual := string(conn.CloneOutputBuffer())
	if actual != expected {
		t.Errorf("expected: %s, actual: %s", expected, actual)
	}
	if st.Hello != "EHLO" {
		t.Errorf("expected: EHLO, actual: %s", st.Hello)
	}
	if st.ClientName != "test-client" {
		t.Errorf("expected: test-client, actual: %s", st.ClientName)
	}
}

func TestMailCommand(t *testing.T) {
	conn := NewMockConn([]byte{})
	smtpConn := NewSMTPConnection(NewSMTPHandler(conn))
	st := smtpConn.State()
	st.Hello = "EHLO"
	cmd := &MailCommand{}
	conn.ResetOutputBuffer()
	cmd.Execute(smtpConn, "MAIL FROM: <foo@example.net>")
	if st.ReturnTo != "foo@example.net" {
		t.Errorf("expected: foo@example.net, actual: %s", st.ReturnTo)
	}
	expected := "250 OK\r\n"
	actual := string(conn.CloneOutputBuffer())
	if actual != expected {
		t.Errorf("expected: %s, actual: %s", expected, actual)
	}
}

func TestRecipientCommand(t *testing.T) {
	conn := NewMockConn([]byte{})
	smtpConn := NewSMTPConnection(NewSMTPHandler(conn))
	st := smtpConn.State()
	st.Hello = "EHLO"
	cmd := &RecipientCommand{}
	conn.ResetOutputBuffer()
	cmd.Execute(smtpConn, "RCPT TO: <user1@example.net>")
	if len(st.Recipients) != 1 ||
		st.Recipients[0] != "user1@example.net" {
		t.Errorf("expected: [user1@example.net], actual: %s", st.Recipients)
	}
	expected := "250 OK\r\n"
	actual := string(conn.CloneOutputBuffer())
	if actual != expected {
		t.Errorf("expected: %s, actual: %s", expected, actual)
	}
	conn.ResetOutputBuffer()
	cmd.Execute(smtpConn, "RCPT TO: <user2@example.net>")
	if len(st.Recipients) != 2 ||
		st.Recipients[0] != "user1@example.net" ||
		st.Recipients[1] != "user2@example.net" {
		t.Errorf("expected: [user1@example.net user2@example.net], actual: %s",
			st.Recipients)
	}
}

func TestResetCommand(t *testing.T) {
	conn := NewMockConn([]byte{})
	smtpConn := NewSMTPConnection(NewSMTPHandler(conn))
	st := smtpConn.State()
	st.Hello = "EHLO"
	st.ServerName = "test-server"
	st.ReturnTo = "foo@example.net"
	st.Recipients = []string{"user1@example.net"}
	st.Headers = []string{"Subject: Awesome products here"}
	st.Content = []byte("Please visit our online shop!")
	cmd := &ResetCommand{}
	conn.ResetOutputBuffer()
	cmd.Execute(smtpConn, "RSET")
	expected := "250 OK\r\n"
	actual := string(conn.CloneOutputBuffer())
	if actual != expected {
		t.Errorf("expected: %s, actual: %s", expected, actual)
	}
	if st.ReturnTo != "" {
		t.Errorf("ReturnTo must be empty")
	}
	if len(st.Recipients) > 0 {
		t.Errorf("Recipients must be empty")
	}
	if len(st.Headers) > 0 {
		t.Errorf("Headers must be empty")
	}
	if len(st.Content) > 0 {
		t.Errorf("Content must be empty")
	}
}

func TestQuitCommand(t *testing.T) {
	conn := NewMockConn([]byte{})
	smtpConn := NewSMTPConnection(NewSMTPHandler(conn))
	st := smtpConn.State()
	st.Hello = "EHLO"
	st.ServerName = "test-server"
	cmd := &QuitCommand{}
	conn.ResetOutputBuffer()
	cmd.Execute(smtpConn, "QUIT")
	expected := "221 Bye\r\n"
	actual := string(conn.CloneOutputBuffer())
	if actual != expected {
		t.Errorf("expected: %s, actual: %s", expected, actual)
	}
	if !conn.IsClosed() {
		t.Error("net.Conn must be closed")
	}
}
