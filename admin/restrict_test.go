package admin

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestParseNets(t *testing.T) {
	nets, err := ParseNets(" 10.0.0.0/16, 100.64.0.0/10 ,,")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(nets) != 2 {
		t.Fatalf("got %d networks, want 2", len(nets))
	}
	if _, err := ParseNets("10.0.0.0/33"); err == nil {
		t.Error("expected an error for an invalid network")
	}
	if nets, _ := ParseNets(""); len(nets) != 0 {
		t.Errorf("got %d networks for an empty string, want 0", len(nets))
	}
}

func TestRestrictTo(t *testing.T) {
	nets, err := ParseNets("10.0.0.0/16,fd7a:115c:a1e0::/48")
	if err != nil {
		t.Fatal(err)
	}
	ok := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {})

	tests := []struct {
		name       string
		nets       bool
		remoteAddr string
		headers    map[string]string
		want       int
	}{
		{name: "allowed network", nets: true, remoteAddr: "10.0.4.20:5555", want: http.StatusOK},
		{name: "IPv4-mapped address", nets: true, remoteAddr: "[::ffff:10.0.4.20]:5555", want: http.StatusOK},
		{name: "allowed IPv6 network", nets: true, remoteAddr: "[fd7a:115c:a1e0::1]:5555", want: http.StatusOK},
		{name: "other network", nets: true, remoteAddr: "10.30.0.200:5555", want: http.StatusForbidden},
		{
			name:       "forwarded headers are ignored",
			nets:       true,
			remoteAddr: "10.30.0.200:5555",
			headers:    map[string]string{"X-Forwarded-For": "10.0.4.20", "CF-Connecting-IP": "10.0.4.20"},
			want:       http.StatusForbidden,
		},
		{name: "unparseable address", nets: true, remoteAddr: "garbage", want: http.StatusForbidden},
		{name: "no networks allows everything", nets: false, remoteAddr: "10.30.0.200:5555", want: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := httptest.NewRequest(http.MethodGet, "/", nil)
			r.RemoteAddr = tt.remoteAddr
			for k, v := range tt.headers {
				r.Header.Set(k, v)
			}
			w := httptest.NewRecorder()
			if tt.nets {
				RestrictTo(nets, ok).ServeHTTP(w, r)
			} else {
				RestrictTo(nil, ok).ServeHTTP(w, r)
			}
			if w.Code != tt.want {
				t.Errorf("got status %d, want %d", w.Code, tt.want)
			}
		})
	}
}
