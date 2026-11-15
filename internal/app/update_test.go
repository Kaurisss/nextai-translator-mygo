package app

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLatestVersion(t *testing.T) {
	for _, tc := range []struct {
		current, tag, want string
		invalid            bool
	}{
		{"0.0.1", "v0.0.2", "v0.0.2", false},
		{"0.9.9", "v0.10.0", "v0.10.0", false},
		{"1.0.0", "v0.99.99", "", false},
		{"0.0.1", "v0.0.1", "", false},
		{"0.0.1", "invalid", "", true},
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprintf(w, `{"tag_name":%q}`, tc.tag)
		}))
		got, err := latestVersion(context.Background(), server.Client(), server.URL, tc.current)
		server.Close()
		if got != tc.want || (err != nil) != tc.invalid {
			t.Errorf("current %q release %q: got %q, error %v", tc.current, tc.tag, got, err)
		}
	}
}
