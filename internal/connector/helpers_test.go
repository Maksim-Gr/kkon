package connector

import (
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
)

// recordedRequest captures what the client sent, so tests can assert on it.
type recordedRequest struct {
	method string
	path   string // path including query string
	body   []byte
}

// newTestClient spins up an httptest server backed by handler and returns a
// client pointed at it plus a pointer to the recorded request.
func newTestClient(t *testing.T, handler http.HandlerFunc) (*Client, *recordedRequest) {
	t.Helper()
	rec := &recordedRequest{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		rec.method = r.Method
		rec.path = r.URL.RequestURI()
		rec.body, _ = io.ReadAll(r.Body)
		handler(w, r)
	}))
	t.Cleanup(srv.Close)
	return NewClient(srv.URL), rec
}
