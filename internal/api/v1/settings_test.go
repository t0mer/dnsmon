package v1_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/t0mer/dnsmon/internal/api/v1"
	sqlitestore "github.com/t0mer/dnsmon/internal/storage/sqlite"
)

func newSettingsStore(t *testing.T) *sqlitestore.Store {
	t.Helper()
	s, err := sqlitestore.New(context.Background(), "file:"+t.TempDir()+"/settings.db?_fk=1")
	require.NoError(t, err)
	t.Cleanup(func() { s.Close() })
	return s
}

func TestGetSettings_Defaults(t *testing.T) {
	store := newSettingsStore(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
	rr := httptest.NewRecorder()

	v1.GetSettings(store).ServeHTTP(rr, req)

	assert.Equal(t, http.StatusOK, rr.Code)
	var body map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body))
	auth := body["auth"].(map[string]any)
	assert.Equal(t, false, auth["enabled"])
	assert.Equal(t, false, auth["password_set"])
}

func TestUpdateSettings_EnableAuthAndRedact(t *testing.T) {
	store := newSettingsStore(t)

	body := `{"auth":{"enabled":true,"username":"admin","password":"hunter2"},"notifications":[{"type":"shoutrrr","name":"ops","enabled":true,"config":{"url":"slack://x"}}],"disabled_resolvers":["google-us"]}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	v1.UpdateSettings(store).ServeHTTP(rr, req)
	require.Equal(t, http.StatusOK, rr.Code)

	// Re-read; password hash must never be exposed, but password_set is true.
	rr = httptest.NewRecorder()
	v1.GetSettings(store).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.NotContains(t, rr.Body.String(), "password_hash")
	assert.NotContains(t, rr.Body.String(), "argon2")

	var body2 map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &body2))
	auth := body2["auth"].(map[string]any)
	assert.Equal(t, true, auth["enabled"])
	assert.Equal(t, "admin", auth["username"])
	assert.Equal(t, true, auth["password_set"])
	assert.Len(t, body2["notifications"], 1)

	// The channel should have been assigned an ID.
	ch := body2["notifications"].([]any)[0].(map[string]any)
	assert.NotEmpty(t, ch["id"])
}

func TestUpdateSettings_EnableAuthWithoutPasswordFails(t *testing.T) {
	store := newSettingsStore(t)
	body := `{"auth":{"enabled":true,"username":"admin"}}`
	req := httptest.NewRequest(http.MethodPut, "/api/v1/settings", bytes.NewBufferString(body))
	rr := httptest.NewRecorder()
	v1.UpdateSettings(store).ServeHTTP(rr, req)
	assert.Equal(t, http.StatusBadRequest, rr.Code)
}

func TestCreateToken_ShownOnceThenListedWithoutSecret(t *testing.T) {
	store := newSettingsStore(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/settings/tokens", bytes.NewBufferString(`{"name":"ci"}`))
	rr := httptest.NewRecorder()
	v1.CreateToken(store).ServeHTTP(rr, req)
	require.Equal(t, http.StatusCreated, rr.Code)

	var created map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	raw, ok := created["token"].(string)
	require.True(t, ok)
	assert.NotEmpty(t, raw)

	// List must not include the raw token or its hash.
	rr = httptest.NewRecorder()
	v1.ListTokens(store).ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/settings/tokens", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.NotContains(t, rr.Body.String(), raw)
	assert.NotContains(t, rr.Body.String(), "hash")

	var list []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &list))
	require.Len(t, list, 1)
	assert.Equal(t, "ci", list[0]["name"])
}

func TestCreateSchedule_AndInvalidCron(t *testing.T) {
	store := newSettingsStore(t)
	r := chi.NewRouter()
	r.Post("/api/v1/settings/schedules", v1.CreateSchedule(store))
	r.Get("/api/v1/settings/schedules", v1.ListSchedules(store))

	// Valid macro cadence.
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/settings/schedules",
		bytes.NewBufferString(`{"name":"nightly","cron":"@daily","enabled":true}`)))
	require.Equal(t, http.StatusCreated, rr.Code)

	// Valid 5-field cron expression.
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/settings/schedules",
		bytes.NewBufferString(`{"name":"every15","cron":"*/15 * * * *","enabled":true}`)))
	require.Equal(t, http.StatusCreated, rr.Code)

	// Invalid cron.
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/settings/schedules",
		bytes.NewBufferString(`{"name":"bad","cron":"not-a-cron"}`)))
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	// Missing cron.
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/settings/schedules",
		bytes.NewBufferString(`{"name":"nocron"}`)))
	assert.Equal(t, http.StatusBadRequest, rr.Code)

	// List shows the two valid schedules.
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/settings/schedules", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var list []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &list))
	assert.Len(t, list, 2)
}
