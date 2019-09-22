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
	mtx          sync.Mutex
}

func NewMockConn(rb []byte) *MockConn {
	return &MockConn{
		readOffset:   0,
		inputBuffer:  rb,
		outputBuffer: make([]byte, 0),
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

func TestSMTPConnectionSend(t *testing.T) {
	conn := NewMockConn([]byte{})
	smtpConn := NewSMTPConnection(conn)
	smtpConn.Send("220 Simple Mail Transfer Service ready")
	expected := "220 Simple Mail Transfer Service ready\r\n"
	actual := string(conn.CloneOutputBuffer())
	if actual != expected {
		t.Errorf("expected: %s, actual: %s", expected, actual)
	}
}

func TestHelloCommand(t *testing.T) {
	conn := NewMockConn([]byte{})
	smtpConn := NewSMTPConnection(conn)
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
