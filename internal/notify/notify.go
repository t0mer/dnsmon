// Package notify delivers messages to configured notification channels:
// a generic Shoutrrr URL, WhatsApp via GreenAPI, or WhatsApp via a
// self-hosted go-whatsapp-web-multidevice instance.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/containrrr/shoutrrr"
	"github.com/t0mer/dnsmon/internal/settings"
)

const greenAPIDefaultBase = "https://api.green-api.com"

// Sender delivers notifications. The zero value is not usable; use New.
type Sender struct {
	http *http.Client
}

// New returns a Sender with a sensible HTTP timeout.
func New() *Sender {
	return &Sender{http: &http.Client{Timeout: 15 * time.Second}}
}

// Send delivers message to the given channel, dispatching by its type.
func (s *Sender) Send(ctx context.Context, ch settings.NotificationChannel, message string) error {
	switch ch.Type {
	case settings.ChannelShoutrrr:
		return s.sendShoutrrr(ch, message)
	case settings.ChannelGreenAPI:
		return s.sendGreenAPI(ctx, ch, message)
	case settings.ChannelGoWA:
		return s.sendGoWA(ctx, ch, message)
	default:
		return fmt.Errorf("unsupported channel type: %q", ch.Type)
	}
}

func (s *Sender) sendShoutrrr(ch settings.NotificationChannel, message string) error {
	url := strings.TrimSpace(ch.Config["url"])
	if url == "" {
		return fmt.Errorf("shoutrrr url is required")
	}
	if err := shoutrrr.Send(url, message); err != nil {
		return fmt.Errorf("shoutrrr: %w", err)
	}
	return nil
}

func (s *Sender) sendGreenAPI(ctx context.Context, ch settings.NotificationChannel, message string) error {
	instanceID := strings.TrimSpace(ch.Config["instance_id"])
	token := strings.TrimSpace(ch.Config["token"])
	recipient := strings.TrimSpace(ch.Config["recipient"])
	if instanceID == "" || token == "" || recipient == "" {
		return fmt.Errorf("greenapi requires instance_id, token, and recipient")
	}

	base := strings.TrimSpace(ch.Config["api_url"])
	if base == "" {
		base = greenAPIDefaultBase
	}

	chatID := recipient
	if !strings.Contains(chatID, "@") {
		chatID += "@c.us"
	}

	url := fmt.Sprintf("%s/waInstance%s/sendMessage/%s", strings.TrimRight(base, "/"), instanceID, token)
	body, _ := json.Marshal(map[string]string{"chatId": chatID, "message": message})

	return s.postJSON(ctx, url, body, nil, "greenapi")
}

func (s *Sender) sendGoWA(ctx context.Context, ch settings.NotificationChannel, message string) error {
	base := strings.TrimSpace(ch.Config["base_url"])
	recipient := strings.TrimSpace(ch.Config["recipient"])
	if base == "" || recipient == "" {
		return fmt.Errorf("go-whatsapp-web requires base_url and recipient")
	}

	url := strings.TrimRight(base, "/") + "/send/message"
	body, _ := json.Marshal(map[string]string{"phone": recipient, "message": message})

	var auth func(*http.Request)
	if user := strings.TrimSpace(ch.Config["username"]); user != "" {
		password := ch.Config["password"]
		auth = func(req *http.Request) { req.SetBasicAuth(user, password) }
	}

	return s.postJSON(ctx, url, body, auth, "go-whatsapp-web")
}

func (s *Sender) postJSON(ctx context.Context, url string, body []byte, auth func(*http.Request), label string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("%s: building request: %w", label, err)
	}
	req.Header.Set("Content-Type", "application/json")
	if auth != nil {
		auth(req)
	}

	resp, err := s.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s: %w", label, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		snippet, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return fmt.Errorf("%s: status %d: %s", label, resp.StatusCode, strings.TrimSpace(string(snippet)))
	}
	return nil
}
