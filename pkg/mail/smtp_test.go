package mail

import (
	"bufio"
	"context"
	"io"
	"net"
	"net/mail"
	"net/textproto"
	"strings"
	"testing"
	"time"
)

// fakeServer accepts one SMTP session (no TLS, no auth) and returns what it
// received: the envelope commands and the message data.
func fakeServer(t *testing.T) (port string, got <-chan []string) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ln.Close() })
	ch := make(chan []string, 1)
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		tp := textproto.NewConn(conn)
		var lines []string
		tp.PrintfLine("220 fake ESMTP")
		for {
			line, err := tp.ReadLine()
			if err != nil {
				ch <- lines
				return
			}
			cmd := strings.ToUpper(strings.Fields(line + " ")[0])
			switch cmd {
			case "EHLO", "HELO":
				tp.PrintfLine("250 fake")
			case "MAIL", "RCPT":
				lines = append(lines, line)
				tp.PrintfLine("250 OK")
			case "DATA":
				tp.PrintfLine("354 go ahead")
				data, _ := io.ReadAll(tp.DotReader())
				lines = append(lines, string(data))
				tp.PrintfLine("250 queued")
			case "QUIT":
				tp.PrintfLine("221 bye")
				ch <- lines
				return
			default:
				tp.PrintfLine("502 not implemented")
			}
		}
	}()
	_, port, _ = net.SplitHostPort(ln.Addr().String())
	return port, ch
}

func TestSend_PlainTextMessage(t *testing.T) {
	port, got := fakeServer(t)
	s := &SMTP{Host: "127.0.0.1", Port: port, From: "Huddle <orders@huddle.ng>", Timeout: 5 * time.Second}

	err := s.Send(context.Background(), Message{
		To:      "seller@example.com",
		Subject: "New paid order\r\nBcc: victim@example.com",
		Body:    "Total: ₦15,000.00\n.\nNote: Bcc: someone@example.com\n",
	})
	if err != nil {
		t.Fatalf("send: %v", err)
	}
	lines := <-got
	if len(lines) != 3 || lines[0] != "MAIL FROM:<orders@huddle.ng>" || !strings.HasPrefix(lines[1], "RCPT TO:<seller@example.com>") {
		t.Fatalf("session = %q", lines)
	}

	msg, err := mail.ReadMessage(bufio.NewReader(strings.NewReader(lines[2])))
	if err != nil {
		t.Fatalf("parse message: %v", err)
	}
	if bcc := msg.Header.Get("Bcc"); bcc != "" {
		t.Errorf("subject added a Bcc header: %q", bcc)
	}
	if s := msg.Header.Get("Subject"); s != "New paid order Bcc: victim@example.com" {
		t.Errorf("subject = %q, want one line", s)
	}
	if msg.Header.Get("To") != "<seller@example.com>" || msg.Header.Get("Content-Transfer-Encoding") != "quoted-printable" {
		t.Errorf("headers = %v", msg.Header)
	}
	body, _ := io.ReadAll(msg.Body)
	if !strings.Contains(string(body), "Total: =E2=82=A615,000.00") {
		t.Errorf("body not quoted-printable: %q", body)
	}
}

func TestSend_RejectsBadRecipient(t *testing.T) {
	s := &SMTP{Host: "127.0.0.1", Port: "1", From: "orders@huddle.ng"}
	if err := s.Send(context.Background(), Message{To: "not an address", Subject: "x", Body: "x"}); err == nil {
		t.Error("bad recipient accepted")
	}
}

func TestFromEnv(t *testing.T) {
	t.Setenv("SMTP_HOST", "")
	t.Setenv("SMTP_FROM", "")
	if _, err := FromEnv(); err != ErrNotConfigured {
		t.Errorf("unset: %v, want ErrNotConfigured", err)
	}
	t.Setenv("SMTP_HOST", "smtp.example.com")
	t.Setenv("SMTP_FROM", "not an address")
	if _, err := FromEnv(); err == nil {
		t.Error("bad SMTP_FROM accepted")
	}
	t.Setenv("SMTP_FROM", "Huddle <orders@huddle.ng>")
	t.Setenv("SMTP_PORT", "")
	s, err := FromEnv()
	if err != nil || s.Port != "587" || s.Host != "smtp.example.com" {
		t.Errorf("FromEnv = %+v, %v", s, err)
	}
}
