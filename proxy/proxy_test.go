package proxy

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func request(remoteAddr string, headers map[string]string) *http.Request {
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.RemoteAddr = remoteAddr
	for k, v := range headers {
		r.Header.Set(k, v)
	}
	return r
}

func TestExtractClientIP(t *testing.T) {
	tests := []struct {
		name       string
		remoteAddr string
		headers    map[string]string
		want       string
	}{
		{
			name:       "CF header takes precedence",
			remoteAddr: "10.1.2.3:1234",
			headers: map[string]string{
				"CF-Connecting-IP": "198.51.100.1",
				"X-Forwarded-For":  "203.0.113.7",
			},
			want: "198.51.100.1",
		},
		{
			name:       "XFF uses rightmost hop",
			remoteAddr: "10.1.2.3:1234",
			headers:    map[string]string{"X-Forwarded-For": "6.6.6.6, 198.51.100.1"},
			want:       "198.51.100.1",
		},
		{
			name:       "X-Real-IP is used when others are absent",
			remoteAddr: "10.1.2.3:1234",
			headers:    map[string]string{"X-Real-IP": "198.51.100.1"},
			want:       "198.51.100.1",
		},
		{
			name:       "no headers falls back to peer address",
			remoteAddr: "10.1.2.3:1234",
			want:       "10.1.2.3",
		},
		{
			name:       "invalid header value falls through to peer",
			remoteAddr: "10.1.2.3:1234",
			headers:    map[string]string{"CF-Connecting-IP": "not-an-ip"},
			want:       "10.1.2.3",
		},
		{
			name:       "IPv4-mapped IPv6 is canonicalized",
			remoteAddr: "[::ffff:203.0.113.7]:1234",
			want:       "203.0.113.7",
		},
		{
			name:       "header value is canonicalized",
			remoteAddr: "10.1.2.3:1234",
			headers:    map[string]string{"X-Real-IP": "::ffff:198.51.100.1"},
			want:       "198.51.100.1",
		},
		{
			name:       "unparsable remote addr is rejected",
			remoteAddr: "not-an-ip",
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractClientIP(request(tt.remoteAddr, tt.headers))
			if got != tt.want {
				t.Errorf("extractClientIP() = %q, want %q", got, tt.want)
			}
		})
	}
}
