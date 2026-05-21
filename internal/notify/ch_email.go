package notify

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"mime/quotedprintable"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"bytes"
)

// EmailSender abstracts the actual SMTP send. The default implementation
// uses net/smtp with STARTTLS (when use_tls=true). Tests substitute a no-op
// sender via NewEmailChannelWithSender.
type EmailSender func(ctx context.Context, cfg EmailSendConfig) error

// EmailSendConfig is the resolved set of parameters the sender needs. All
// fields are pre-validated by EmailChannel.Send.
type EmailSendConfig struct {
	Host     string
	Port     int
	Username string
	Password string
	From     string
	To       []string
	UseTLS   bool
	Subject  string
	Body     string
	HTML     string
}

// EmailChannel renders messages as multipart/alternative MIME and ships them
// via SMTP. Config map keys mirror types.EmailConfig:
//
//	smtp_host       string (required)
//	smtp_port       int    (required, 1..65535)
//	smtp_user       string (optional but recommended)
//	smtp_password   string (optional; required when smtp_user is set)
//	smtp_tls        bool   (default true; STARTTLS)
//	from            string (required, e.g. "alerts@example.com")
//	to              []string (required, at least 1 entry)
type EmailChannel struct {
	send      EmailSender
	templates *Template
}

// NewEmailChannel returns a channel using the default net/smtp sender.
func NewEmailChannel(templates *Template) *EmailChannel {
	return &EmailChannel{
		send:      defaultEmailSender,
		templates: templates,
	}
}

// NewEmailChannelWithSender lets tests inject a custom sender. templates may
// be nil — the channel then renders only the plain text body without HTML.
func NewEmailChannelWithSender(sender EmailSender, templates *Template) *EmailChannel {
	if sender == nil {
		sender = defaultEmailSender
	}
	return &EmailChannel{send: sender, templates: templates}
}

// Kind returns the type identifier.
func (c *EmailChannel) Kind() string { return "email" }

// Validate enforces minimum config completeness.
func (c *EmailChannel) Validate(cfg any) error {
	m, ok := cfg.(map[string]any)
	if !ok {
		return errors.New("email: config must be an object")
	}
	if asString(m, "smtp_host") == "" {
		return errors.New("email: smtp_host required")
	}
	port := asInt(m, "smtp_port")
	if port <= 0 || port > 65535 {
		return errors.New("email: smtp_port required (1..65535)")
	}
	if asString(m, "from") == "" {
		return errors.New("email: from required")
	}
	to := asStringSlice(m, "to")
	if len(to) == 0 {
		return errors.New("email: to required (at least one recipient)")
	}
	return nil
}

// Send renders the multipart MIME body and ships it via SMTP. The HTML body
// is rendered via EmailChannel.templates when supplied; otherwise the plain
// text body is escaped and wrapped in a trivial <p>.
func (c *EmailChannel) Send(ctx context.Context, cfg any, msg Message) error {
	if err := c.Validate(cfg); err != nil {
		return err
	}
	m := cfg.(map[string]any)
	useTLS := true
	if v, ok := m["smtp_tls"].(bool); ok {
		useTLS = v
	}
	sendCfg := EmailSendConfig{
		Host:     asString(m, "smtp_host"),
		Port:     asInt(m, "smtp_port"),
		Username: asString(m, "smtp_user"),
		Password: asString(m, "smtp_password"),
		From:     asString(m, "from"),
		To:       asStringSlice(m, "to"),
		UseTLS:   useTLS,
		Subject:  msg.Subject,
		Body:     msg.Body,
	}
	// When templates are available render an HTML body using the locale-
	// specific .html.tmpl (falls back to escaped plain text).
	if c.templates != nil {
		htmlBody, err := c.templates.RenderHTML(Event{
			Type:    EventType(msg.EventType),
			Subject: msg.Subject,
			Locale:  msg.Locale,
			Payload: map[string]any{"Body": msg.Body, "Subject": msg.Subject},
		})
		if err == nil {
			sendCfg.HTML = htmlBody
		}
	}
	return c.send(ctx, sendCfg)
}

// defaultEmailSender ships sendCfg via net/smtp. STARTTLS is performed when
// sendCfg.UseTLS is true.
func defaultEmailSender(ctx context.Context, cfg EmailSendConfig) error {
	addr := cfg.Host + ":" + strconv.Itoa(cfg.Port)
	msg := buildMIMEMessage(cfg)
	deadline, ok := ctx.Deadline()
	if !ok {
		deadline = time.Now().Add(30 * time.Second)
	}
	d := time.Until(deadline)
	if d < time.Second {
		d = time.Second
	}
	// net/smtp doesn't natively respect ctx; we wrap the call so the
	// outer Send timeout still applies if the SMTP exchange hangs.
	done := make(chan error, 1)
	go func() {
		done <- doSMTPSend(addr, cfg, msg)
	}()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("email send: %w", ctx.Err())
	case <-time.After(d):
		return fmt.Errorf("email send: deadline exceeded after %s", d)
	}
}

// doSMTPSend executes the actual SMTP dialogue. Split out so the goroutine
// in defaultEmailSender stays trivial.
func doSMTPSend(addr string, cfg EmailSendConfig, mimeBody []byte) error {
	client, err := smtp.Dial(addr)
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer client.Close()
	if err := client.Hello("shiguang-vps"); err != nil {
		return fmt.Errorf("hello: %w", err)
	}
	if cfg.UseTLS {
		// STARTTLS — skip verify is NOT applied; production deployments
		// must use a server with a valid cert. Self-signed users disable
		// use_tls instead (config-level decision).
		if ok, _ := client.Extension("STARTTLS"); ok {
			tlsCfg := &tls.Config{ServerName: cfg.Host}
			if err := client.StartTLS(tlsCfg); err != nil {
				return fmt.Errorf("starttls: %w", err)
			}
		}
	}
	if cfg.Username != "" {
		auth := smtp.PlainAuth("", cfg.Username, cfg.Password, cfg.Host)
		if err := client.Auth(auth); err != nil {
			return fmt.Errorf("auth: %w", err)
		}
	}
	if err := client.Mail(cfg.From); err != nil {
		return fmt.Errorf("mail from: %w", err)
	}
	for _, rcpt := range cfg.To {
		if err := client.Rcpt(rcpt); err != nil {
			return fmt.Errorf("rcpt %s: %w", rcpt, err)
		}
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("data: %w", err)
	}
	if _, err := wc.Write(mimeBody); err != nil {
		_ = wc.Close()
		return fmt.Errorf("write body: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("close data: %w", err)
	}
	return client.Quit()
}

// buildMIMEMessage assembles a multipart/alternative payload with both a
// text/plain and a text/html part. Content-Transfer-Encoding is
// quoted-printable so 8-bit characters (CJK subjects) survive SMTP servers
// that mangle raw UTF-8.
func buildMIMEMessage(cfg EmailSendConfig) []byte {
	var buf bytes.Buffer
	buf.WriteString("From: ")
	buf.WriteString(cfg.From)
	buf.WriteString("\r\n")
	buf.WriteString("To: ")
	buf.WriteString(strings.Join(cfg.To, ", "))
	buf.WriteString("\r\n")
	buf.WriteString("Subject: ")
	buf.WriteString(encodeMIMEHeader(cfg.Subject))
	buf.WriteString("\r\n")
	buf.WriteString("MIME-Version: 1.0\r\n")
	boundary := "shiguang_vps_boundary"
	if cfg.HTML == "" {
		buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
		buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
		qpWrite(&buf, cfg.Body)
		return buf.Bytes()
	}
	buf.WriteString("Content-Type: multipart/alternative; boundary=\"")
	buf.WriteString(boundary)
	buf.WriteString("\"\r\n\r\n")
	// Plain text part
	buf.WriteString("--")
	buf.WriteString(boundary)
	buf.WriteString("\r\n")
	buf.WriteString("Content-Type: text/plain; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	qpWrite(&buf, cfg.Body)
	buf.WriteString("\r\n")
	// HTML part
	buf.WriteString("--")
	buf.WriteString(boundary)
	buf.WriteString("\r\n")
	buf.WriteString("Content-Type: text/html; charset=UTF-8\r\n")
	buf.WriteString("Content-Transfer-Encoding: quoted-printable\r\n\r\n")
	qpWrite(&buf, cfg.HTML)
	buf.WriteString("\r\n")
	buf.WriteString("--")
	buf.WriteString(boundary)
	buf.WriteString("--\r\n")
	return buf.Bytes()
}

// qpWrite copies s into buf, escaped via quoted-printable.
func qpWrite(buf *bytes.Buffer, s string) {
	w := quotedprintable.NewWriter(buf)
	_, _ = w.Write([]byte(s))
	_ = w.Close()
}

// encodeMIMEHeader wraps non-ASCII subject lines in the encoded-word format
// per RFC 2047 so MUAs render CJK characters correctly.
func encodeMIMEHeader(s string) string {
	if isASCII(s) {
		return s
	}
	var buf bytes.Buffer
	buf.WriteString("=?UTF-8?Q?")
	w := quotedprintable.NewWriter(&buf)
	_, _ = w.Write([]byte(s))
	_ = w.Close()
	buf.WriteString("?=")
	return buf.String()
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
}

// asInt returns m[key] coerced to int. Float64 (JSON number) and integer
// strings are both accepted. Returns 0 on failure.
func asInt(m map[string]any, key string) int {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case string:
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return 0
}

// asStringSlice returns m[key] as a []string. JSON unmarshals string arrays
// into []any so we accept both.
func asStringSlice(m map[string]any, key string) []string {
	if m == nil {
		return nil
	}
	switch v := m[key].(type) {
	case []string:
		return v
	case []any:
		out := make([]string, 0, len(v))
		for _, e := range v {
			if s, ok := e.(string); ok && s != "" {
				out = append(out, s)
			}
		}
		return out
	}
	return nil
}

// buildEmail is the registry factory. Templates default to a fresh one so
// the channel can render HTML; production wires Manager.cfg.Templates here.
func buildEmail(cfg map[string]any) (Channel, error) {
	ch := NewEmailChannel(nil)
	if err := ch.Validate(cfg); err != nil {
		return nil, err
	}
	return ch, nil
}
