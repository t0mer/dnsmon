package v1_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	v1 "github.com/t0mer/dnsmon/internal/api/v1"
	"github.com/t0mer/dnsmon/internal/storage"
)

func monitorRouter(store storage.Storage) *chi.Mux {
	r := chi.NewRouter()
	r.Get("/api/v1/settings/monitors", v1.ListMonitors(store))
	r.Post("/api/v1/settings/monitors", v1.CreateMonitor(store))
	r.Put("/api/v1/settings/monitors/{id}", v1.UpdateMonitor(store))
	r.Delete("/api/v1/settings/monitors/{id}", v1.DeleteMonitor(store))
	r.Get("/api/v1/settings/monitors/{id}/history", v1.MonitorHistory(store))
	return r
}

func TestCreateMonitor_PropagationAndChange(t *testing.T) {
	store := newSettingsStore(t)
	r := monitorRouter(store)

	// Propagation monitor with expected values.
	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/settings/monitors",
		bytes.NewBufferString(`{"type":"propagation","fqdn":"example.com","record_type":"A","expected":["1.2.3.4"],"enabled":true}`)))
	require.Equal(t, http.StatusCreated, rr.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	id := created["id"].(string)
	assert.Equal(t, "example.com", created["name"]) // name defaults to FQDN

	// Change monitor (no expected required).
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/settings/monitors",
		bytes.NewBufferString(`{"type":"change","name":"drift","fqdn":"example.org","record_type":"AAAA","expected":["::1"]}`)))
	require.Equal(t, http.StatusCreated, rr.Code)

	// The propagation monitor's changelog has the initial "created" event.
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/settings/monitors/"+id+"/history", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var events []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &events))
	require.Len(t, events, 1)
	assert.Equal(t, "created", events[0]["status"])

	// List shows both.
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodGet, "/api/v1/settings/monitors", nil))
	require.Equal(t, http.StatusOK, rr.Code)
	var list []map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &list))
	assert.Len(t, list, 2)
}

func TestCreateMonitor_Validation(t *testing.T) {
	store := newSettingsStore(t)
	r := monitorRouter(store)

	cases := []string{
		`{"type":"propagation","fqdn":"example.com","record_type":"A"}`,      // propagation needs expected
		`{"type":"bogus","fqdn":"example.com","record_type":"A","expected":["x"]}`, // bad type
		`{"type":"change","fqdn":"bad domain!","record_type":"A"}`,           // bad fqdn
		`{"type":"change","fqdn":"example.com","record_type":"ZZZ"}`,         // bad record type
	}
	for _, body := range cases {
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/settings/monitors", bytes.NewBufferString(body)))
		assert.Equal(t, http.StatusBadRequest, rr.Code, body)
	}
}

func TestUpdateAndDeleteMonitor(t *testing.T) {
	store := newSettingsStore(t)
	r := monitorRouter(store)

	rr := httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPost, "/api/v1/settings/monitors",
		bytes.NewBufferString(`{"type":"change","fqdn":"example.com","record_type":"A","expected":["1.2.3.4"]}`)))
	require.Equal(t, http.StatusCreated, rr.Code)
	var created map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &created))
	id := created["id"].(string)

	// Update.
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/v1/settings/monitors/"+id,
		bytes.NewBufferString(`{"type":"change","name":"renamed","fqdn":"example.com","record_type":"A","expected":["5.6.7.8"]}`)))
	require.Equal(t, http.StatusOK, rr.Code)
	var updated map[string]any
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &updated))
	assert.Equal(t, "renamed", updated["name"])

	// Delete.
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodDelete, "/api/v1/settings/monitors/"+id, nil))
	require.Equal(t, http.StatusNoContent, rr.Code)

	// Update missing -> 404.
	rr = httptest.NewRecorder()
	r.ServeHTTP(rr, httptest.NewRequest(http.MethodPut, "/api/v1/settings/monitors/"+id,
		bytes.NewBufferString(`{"type":"change","fqdn":"example.com","record_type":"A","expected":["x"]}`)))
	assert.Equal(t, http.StatusNotFound, rr.Code)
}
