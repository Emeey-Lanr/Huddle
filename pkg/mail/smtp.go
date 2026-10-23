// Package mail sends plain-text email over SMTP.
//
// Config comes from the environment:
//   - SMTP_HOST: the mail server, e.g. smtp-relay.brevo.com
//   - SMTP_PORT: defaults to 587. Port 465 uses TLS from the start; other
//     ports upgrade with STARTTLS when the server offers it.
//   - SMTP_USERNAME, SMTP_PASSWORD: optional; only ever sent over TLS
//   - SMTP_FROM: the sender, e.g. "Huddle <orders@huddle.ng>"
package mail

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"errors"
	"fmt"
	"mime"
	"mime/quotedprintable"
	"net"
	"net/mail"
	"net/smtp"
	"os"
	"strings"
	"time"
)

var ErrNotConfigured = errors.New("SMTP_HOST and SMTP_FROM are not set")

// Message is a plain-text email to one recipient.
type Message struct {
	To      string
	Subject string
	Body    string
}

// Sender sends one email.
type Sender interface {
	Send(ctx context.Context, m Message) error
}

type SMTP struct {
	Host     string
	Port     string
	Username string
	Password string
	From     string
	Timeout  time.Duration
	// TLS overrides the TLS settings (tests use it to trust their server).
	TLS *tls.Config
}

// FromEnv builds an SMTP sender from the environment.
func FromEnv() (*SMTP, error) {
	host, from := os.Getenv("SMTP_HOST"), os.Getenv("SMTP_FROM")
	if host == "" || from == "" {
		return nil, ErrNotConfigured
	}
	if _, err := mail.ParseAddress(from); err != nil {
		return nil, fmt.Errorf("SMTP_FROM: %w", err)
	}
	port := os.Getenv("SMTP_PORT")
	if port == "" {
		port = "587"
	}
	return &SMTP{
		Host: host, Port: port, From: from, Timeout: 30 * time.Second,
		Username: os.Getenv("SMTP_USERNAME"), Password: os.Getenv("SMTP_PASSWORD"),
	}, nil
}

func (s *SMTP) Send(ctx context.Context, m Message) error {
	from, err := mail.ParseAddress(s.From)
	if err != nil {
		return fmt.Errorf("sender address: %w", err)
	}
	to, err := mail.ParseAddress(m.To)
	if err != nil {
		return fmt.Errorf("recipient address: %w", err)
	}
	data, err := build(from, to, m, time.Now())
	if err != nil {
		return err
	}

	timeout := s.Timeout
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	deadline := time.Now().Add(timeout)
	if d, ok := ctx.Deadline(); ok && d.Before(deadline) {
		deadline = d
	}
	tlsConfig := s.TLS
	if tlsConfig == nil {
		tlsConfig = &tls.Config{ServerName: s.Host}
	}

	addr := net.JoinHostPort(s.Host, s.Port)
	dialer := &net.Dialer{Deadline: deadline}
	var conn net.Conn
	if s.Port == "465" {
		conn, err = (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", addr)
	} else {
		conn, err = dialer.DialContext(ctx, "tcp", addr)
	}
	if err != nil {
		return err
	}
	// net/smtp has no context support; the deadline bounds the whole exchange.
	conn.SetDeadline(deadline)

	c, err := smtp.NewClient(conn, s.Host)
	if err != nil {
		conn.Close()
		return err
	}
	defer c.Close()
	if s.Port != "465" {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(tlsConfig); err != nil {
				return err
			}
		}
	}
	if s.Username != "" {
		// PlainAuth refuses to send the password over an unencrypted
		// connection (except to localhost).
		if err := c.Auth(smtp.PlainAuth("", s.Username, s.Password, s.Host)); err != nil {
			return err
		}
	}
	if err := c.Mail(from.Address); err != nil {
		return err
	}
	if err := c.Rcpt(to.Address); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write(data); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}

// build renders the message. The subject is folded onto one line and
// encoded, and the body is quoted-printable, so nothing in them (such as a
// shopper's note) can add headers.
func build(from, to *mail.Address, m Message, now time.Time) ([]byte, error) {
	subject := strings.Join(strings.Fields(m.Subject), " ")
	domain := from.Address[strings.LastIndex(from.Address, "@")+1:]
	id := make([]byte, 16)
	rand.Read(id)

	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", from.String())
	fmt.Fprintf(&buf, "To: %s\r\n", to.String())
	fmt.Fprintf(&buf, "Subject: %s\r\n", mime.QEncoding.Encode("utf-8", subject))
	fmt.Fprintf(&buf, "Date: %s\r\n", now.Format(time.RFC1123Z))
	fmt.Fprintf(&buf, "Message-ID: <%s@%s>\r\n", hex.EncodeToString(id), domain)
	buf.WriteString("MIME-Version: 1.0\r\n")
	buf.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")

	qp := quotedprintable.NewWriter(&buf)
	if _, err := qp.Write([]byte(m.Body)); err != nil {
		return nil, err
	}
	if err := qp.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
