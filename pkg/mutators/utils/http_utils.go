package utils

import (
	"net/http"
)

// UrlGetter wraps calls to the Go HTTP library, so we can mock it in tests
type UrlGetter interface {
	Do(req *http.Request) (*http.Response, error)
}
