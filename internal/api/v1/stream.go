package v1

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	"github.com/t0mer/dnsmon/internal/api/apierr"
	"github.com/t0mer/dnsmon/internal/checker"
	"github.com/t0mer/dnsmon/internal/dnsclient"
)

type wsStreamRequest struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Resolvers []string `json:"resolvers"`
}

type wsMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

type wsDoneData struct {
	Summary dnsclient.CheckSummary `json:"summary"`
	ID      string                 `json:"id"`
}

type wsErrorData struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// StreamCheck handles GET /api/v1/check/stream (WebSocket).
// baseURL is the configured server base URL (e.g. "https://dnsmon.example.com"). When
// non-empty it is used as the sole allowed WebSocket origin, preventing cross-site
// WebSocket hijacking. When empty the library's default same-origin policy applies.
func StreamCheck(chkr *checker.Checker, baseURL string) http.HandlerFunc {
	opts := &websocket.AcceptOptions{}
	if baseURL != "" {
		opts.OriginPatterns = []string{baseURL}
	}
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := websocket.Accept(w, r, opts)
		if err != nil {
			apierr.WriteError(w, r, http.StatusBadRequest, apierr.ErrCodeInvalidInput,
				"Failed to upgrade WebSocket connection.", nil)
			return
		}
		defer conn.CloseNow()

		// Read the request frame first, then switch to read-close so a client
		// disconnect cancels the stream (CloseRead would otherwise consume the
		// request frame and close the connection before we read it).
		var req wsStreamRequest
		if err := wsjson.Read(r.Context(), conn, &req); err != nil {
			sendWSError(r.Context(), conn, apierr.ErrCodeInvalidInput, "Failed to read request.")
			return
		}

		if req.Name == "" || req.Type == "" {
			sendWSError(r.Context(), conn, apierr.ErrCodeInvalidInput, "name and type are required.")
			return
		}

		ctx := conn.CloseRead(r.Context())

		total, id, resultCh, doneCh, err := chkr.Stream(ctx, checker.StreamRequest{
			Name:        req.Name,
			Type:        req.Type,
			ResolverIDs: req.Resolvers,
		}, true)
		if err != nil {
			sendWSError(ctx, conn, apierr.ErrCodeInvalidInput, err.Error())
			return
		}

		// Announce the resolver count up front so the client can show progress.
		if err := wsjson.Write(ctx, conn, wsMessage{Type: "total", Data: map[string]int{"total": total}}); err != nil {
			return
		}

		for result := range resultCh {
			msg := wsMessage{Type: "result", Data: result}
			if err := wsjson.Write(ctx, conn, msg); err != nil {
				return
			}
		}

		if summary, ok := <-doneCh; ok && summary != nil {
			msg := wsMessage{
				Type: "done",
				Data: wsDoneData{Summary: *summary, ID: id},
			}
			_ = wsjson.Write(ctx, conn, msg)
		}

		conn.Close(websocket.StatusNormalClosure, "done")
	}
}

func sendWSError(ctx context.Context, conn *websocket.Conn, code, message string) {
	msg := wsMessage{
		Type: "error",
		Data: wsErrorData{Code: code, Message: message},
	}
	data, _ := json.Marshal(msg)
	_ = conn.Write(ctx, websocket.MessageText, data)
}
