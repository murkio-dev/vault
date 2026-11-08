package entities

import (
	"net"
	"net/http"
	"strings"
	"uuid"
)

type ApiHeader struct {
	ClientIP  string
	UserAgent string
	Origin    string
	RequestID string
	Country   string
	CdnRayID  string
	Actor     Actor
}

func ExtractApiHeader(r *http.Request) ApiHeader {
	// Os headers são preenchidos pelo Caddy (borda), que fica na frente do vault.
	ip := strings.TrimSpace(r.Header.Get("CF-Connecting-IP"))
	if ip == "" {
		first, _, _ := strings.Cut(r.Header.Get("X-Forwarded-For"), ",")
		ip = strings.TrimSpace(first)
	}
	if ip == "" {
		ip, _, _ = net.SplitHostPort(r.RemoteAddr)
	}

	reqID := r.Header.Get("X-Request-ID")
	if reqID == "" {
		reqID = uuid.New().String()
	}

	return ApiHeader{
		ClientIP:  ip,
		UserAgent: r.UserAgent(),
		Origin:    r.Header.Get("Origin"),
		RequestID: reqID,
		Country:   r.Header.Get("CF-IPCountry"),
		CdnRayID:  r.Header.Get("CF-Ray"),
		Actor:     ActorFrom(r.Context()),
	}
}
