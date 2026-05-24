// Package settings holds the user-editable application configuration that is
// persisted via the storage layer and managed from the /settings page:
// notification channels, optional UI authentication, external API tokens,
// resolver enable/disable state, and monitoring schedules.
package settings

import "time"

// Channel type identifiers for NotificationChannel.Type.
const (
	ChannelShoutrrr = "shoutrrr" // generic, via github.com/containrrr/shoutrrr (PR2)
	ChannelGreenAPI = "greenapi" // WhatsApp via GreenAPI cloud REST (PR2)
	ChannelGoWA     = "gowa"     // WhatsApp via a self-hosted go-whatsapp-web-multidevice instance (PR2)
)

// Settings is the singleton application configuration editable from the UI.
type Settings struct {
	Auth              AuthSettings          `json:"auth"`
	Notifications     []NotificationChannel `json:"notifications"`
	DisabledResolvers []string              `json:"disabled_resolvers"`
	// SessionSecret signs UI session cookies. Generated on demand; persisted
	// but never exposed to API clients.
	SessionSecret string `json:"session_secret"`
}

// AuthSettings controls optional username/password protection of the UI.
type AuthSettings struct {
	Enabled bool   `json:"enabled"`
	Username string `json:"username"`
	// PasswordHash is the argon2id-encoded admin password. It is persisted but
	// never exposed to API clients (the handler layer redacts it).
	PasswordHash string `json:"password_hash"`
}

// NotificationChannel describes one configured notification target. The Config
// map holds type-specific fields (e.g. shoutrrr "url"; greenapi "instance_id"
// and "token"; gowa "base_url", "username", "password", "recipient").
type NotificationChannel struct {
	ID      string            `json:"id"`
	Type    string            `json:"type"`
	Name    string            `json:"name"`
	Enabled bool              `json:"enabled"`
	Config  map[string]string `json:"config"`
}

// APIToken is an external bearer token. The raw secret is shown once at creation;
// only its argon2id hash is persisted.
type APIToken struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	Hash       string     `json:"-"`
	Prefix     string     `json:"prefix"`
	CreatedAt  time.Time  `json:"created_at"`
	LastUsedAt *time.Time `json:"last_used_at,omitempty"`
}

// Schedule is a reusable cadence (a cron expression or macro such as @daily /
// @hourly). Monitors attach to a schedule to decide when they run; the schedule
// itself carries no domain or record. Execution is implemented in a later change.
type Schedule struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Cron      string    `json:"cron"` // "@hourly", "@daily", or a cron expression
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Default returns an empty Settings with sensible zero values.
func Default() *Settings {
	return &Settings{
		Notifications:     []NotificationChannel{},
		DisabledResolvers: []string{},
	}
}
