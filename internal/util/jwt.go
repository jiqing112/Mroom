package util

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"strings"
	"time"
)

// The Navidrome client protocol signs in with a JWT and then presents it as
// "X-ND-Authorization: Bearer <token>". mrome only needs the tiny HS256 subset,
// so this stays dependency-free and reuses the instance secret.

type jwtClaims struct {
	UID string `json:"uid"`
	Iat int64  `json:"iat"`
	Exp int64  `json:"exp"`
}

// SignJWT issues an HS256 token carrying the user id in Navidrome's "uid" claim.
func SignJWT(secret []byte, uid string, ttl time.Duration) string {
	head := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	now := time.Now().Unix()
	payload, _ := json.Marshal(jwtClaims{UID: uid, Iat: now, Exp: now + int64(ttl.Seconds())})
	body := head + "." + base64.RawURLEncoding.EncodeToString(payload)
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(body))
	return body + "." + base64.RawURLEncoding.EncodeToString(mac.Sum(nil))
}

// ParseJWT verifies the signature and expiry and returns the user id.
func ParseJWT(token string, secret []byte) (string, bool) {
	parts := strings.Split(strings.TrimSpace(token), ".")
	if len(parts) != 3 {
		return "", false
	}
	body := parts[0] + "." + parts[1]
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(body))
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil || !hmac.Equal(sig, mac.Sum(nil)) {
		return "", false
	}
	raw, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	var c jwtClaims
	if err := json.Unmarshal(raw, &c); err != nil {
		return "", false
	}
	if c.UID == "" || (c.Exp > 0 && c.Exp < time.Now().Unix()) {
		return "", false
	}
	return c.UID, true
}

// BearerToken pulls a token out of "Authorization: Bearer x" or the
// Navidrome-specific "X-ND-Authorization: Bearer x" header.
func BearerToken(h func(string) string) string {
	for _, name := range []string{"X-ND-Authorization", "Authorization"} {
		v := strings.TrimSpace(h(name))
		if v == "" {
			continue
		}
		if len(v) > 7 && strings.EqualFold(v[:7], "bearer ") {
			return strings.TrimSpace(v[7:])
		}
		return v
	}
	return ""
}
