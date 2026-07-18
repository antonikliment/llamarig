package public_http

import (
	"crypto/sha256"
	"crypto/subtle"
	"net"
	"net/http"
	"net/url"

	"llamarig/core/control"
)

func originGuard(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin == "" || sameOriginHost(origin, r.Host, r) {
			next.ServeHTTP(w, r)
			return
		}
		writeCoreError(w, control.Errorf(control.ErrorPermission, "origin is not allowed"))
	})
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if bearerTokenMatches(r.Header.Get("Authorization"), s.authToken) {
			next(w, r)
			return
		}
		writeCoreError(w, control.Errorf(control.ErrorPermission, "authorization required"))
	}
}

func (s *Server) requireAuthHandler(next http.Handler) http.Handler {
	return s.requireAuth(next.ServeHTTP)
}

func sameOriginHost(origin string, requestHost string, r *http.Request) bool {
	parsed, err := url.Parse(origin)
	if err != nil {
		return false
	}
	originHost := parsed.Hostname()
	originPort := parsed.Port()
	if originPort == "" {
		originPort = defaultOriginPort(parsed.Scheme)
	}
	requestOnlyHost, requestPort, err := net.SplitHostPort(requestHost)
	if err != nil {
		requestOnlyHost = requestHost
		requestPort = defaultRequestPort(r)
	}
	if len(requestOnlyHost) >= 2 && requestOnlyHost[0] == '[' && requestOnlyHost[len(requestOnlyHost)-1] == ']' {
		requestOnlyHost = requestOnlyHost[1 : len(requestOnlyHost)-1]
	}
	return originHost == requestOnlyHost && originPort == requestPort && isTrustedRequestHost(requestOnlyHost)
}

func bearerTokenMatches(authHeader string, token string) bool {
	if token == "" {
		return true
	}
	expected := "Bearer " + token
	authHeaderHash := sha256.Sum256([]byte(authHeader))
	expectedHash := sha256.Sum256([]byte(expected))
	return subtle.ConstantTimeCompare(authHeaderHash[:], expectedHash[:]) == 1
}

func isTrustedRequestHost(host string) bool {
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && (ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast())
}

func defaultOriginPort(scheme string) string {
	if scheme == "https" {
		return "443"
	}
	return "80"
}

func defaultRequestPort(r *http.Request) string {
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		return "443"
	}
	return "80"
}
