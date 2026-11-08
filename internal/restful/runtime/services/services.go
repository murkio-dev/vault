package services

import (
	"net/http"
	"strings"
)

func ParseEnv(r *http.Request) (uriID string, names []string) {
	if raw := r.URL.Query().Get("names"); raw != "" {
		names = strings.Split(raw, ",")
	}
	return r.PathValue("uri_id"), names
}
