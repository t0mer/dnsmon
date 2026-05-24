package notify

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/t0mer/dnsmon/internal/settings"
)

func TestSendGreenAPI(t *testing.T) {
	var gotPath string
	var gotBody map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"idMessage":"abc"}`))
	}))
	defer srv.Close()

	ch := settings.NotificationChannel{
		Type: settings.ChannelGreenAPI,
		Config: map[string]string{
			"api_url":     srv.URL,
			"instance_id": "1101",
			"token":       "tok",
			"recipient":   "15551230000",
		},
	}

	if err := New().Send(context.Background(), ch, "hello"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotPath != "/waInstance1101/sendMessage/tok" {
		t.Errorf("path = %q", gotPath)
	}
	if gotBody["chatId"] != "15551230000@c.us" {
		t.Errorf("chatId = %q, want 15551230000@c.us", gotBody["chatId"])
	}
	if gotBody["message"] != "hello" {
		t.Errorf("message = %q", gotBody["message"])
	}
}

func TestSendGreenAPI_ErrorStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`bad token`))
	}))
	defer srv.Close()

	ch := settings.NotificationChannel{
		Type:   settings.ChannelGreenAPI,
		Config: map[string]string{"api_url": srv.URL, "instance_id": "1", "token": "x", "recipient": "1"},
	}
	err := New().Send(context.Background(), ch, "hi")
	if err == nil || !strings.Contains(err.Error(), "401") {
		t.Fatalf("expected 401 error, got %v", err)
	}
}

func TestSendGoWA(t *testing.T) {
	var gotPath, gotAuthUser string
	var gotBody map[string]string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuthUser, _, _ = r.BasicAuth()
		b, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(b, &gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	ch := settings.NotificationChannel{
		Type: settings.ChannelGoWA,
		Config: map[string]string{
			"base_url":  srv.URL,
			"username":  "admin",
			"password":  "secret",
			"recipient": "6281234567890",
		},
	}

	if err := New().Send(context.Background(), ch, "ping"); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if gotPath != "/send/message" {
		t.Errorf("path = %q", gotPath)
	}
	if gotAuthUser != "admin" {
		t.Errorf("basic auth user = %q", gotAuthUser)
	}
	if gotBody["phone"] != "6281234567890" || gotBody["message"] != "ping" {
		t.Errorf("body = %+v", gotBody)
	}
}

func TestSend_MissingConfig(t *testing.T) {
	cases := []settings.NotificationChannel{
		{Type: settings.ChannelShoutrrr, Config: map[string]string{}},
		{Type: settings.ChannelGreenAPI, Config: map[string]string{"instance_id": "1"}},
		{Type: settings.ChannelGoWA, Config: map[string]string{}},
		{Type: "bogus", Config: map[string]string{}},
	}
	for _, ch := range cases {
		if err := New().Send(context.Background(), ch, "x"); err == nil {
			t.Errorf("expected error for %q with config %v", ch.Type, ch.Config)
		}
	}
}
