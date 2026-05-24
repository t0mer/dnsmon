package settings

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

// SessionCookie is the name of the UI session cookie.
const SessionCookie = "dnsmon_session"

// SignSession returns a signed session token for username that expires after
// ttl. The token format is base64url(username|expiryUnix).hexHMAC, signed with
// secret. ParseSession reverses it.
func SignSession(secret, username string, ttl time.Duration) string {
	payload := username + "|" + strconv.FormatInt(time.Now().Add(ttl).Unix(), 10)
	enc := base64.RawURLEncoding.EncodeToString([]byte(payload))
	return enc + "." + sign(secret, enc)
}

// ParseSession validates a session token and returns the username it carries.
// It returns ok=false if the signature is invalid or the token has expired.
func ParseSession(secret, token string) (username string, ok bool) {
	enc, mac, found := strings.Cut(token, ".")
	if !found {
		return "", false
	}
	if !hmac.Equal([]byte(mac), []byte(sign(secret, enc))) {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(enc)
	if err != nil {
		return "", false
	}
	name, expStr, found := strings.Cut(string(raw), "|")
	if !found {
		return "", false
	}
	exp, err := strconv.ParseInt(expStr, 10, 64)
	if err != nil || time.Now().Unix() >= exp {
		return "", false
	}
	return name, true
}

func sign(secret, msg string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(msg))
	return base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}
