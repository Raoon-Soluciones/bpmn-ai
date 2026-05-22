package middleware

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
)

// CSRFSecure controls whether the CSRF cookie has the Secure flag set.
// Set to true in production when using HTTPS.
var CSRFSecure bool

func CSRF(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead ||
			r.Method == http.MethodOptions || r.Method == http.MethodTrace {
			next.ServeHTTP(w, r)
			return
		}

		headerToken := r.Header.Get("X-CSRF-Token")
		cookieToken := ""

		if cookie, err := r.Cookie("csrf_token"); err == nil {
			cookieToken = cookie.Value
		}

		if headerToken == "" || cookieToken == "" || headerToken != cookieToken {
			http.Error(w, "invalid or missing CSRF token", http.StatusForbidden)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func GenerateCSRFToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

func SetCSRFCookie(w http.ResponseWriter, token string) {
	http.SetCookie(w, &http.Cookie{
		Name:     "csrf_token",
		Value:    token,
		Path:     "/",
		HttpOnly: false,
		Secure:   CSRFSecure,
		SameSite: http.SameSiteStrictMode,
	})
}
