package allowlist

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRecordAttempt(t *testing.T) {
	// Mock ipinfo.io
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{
			"city": "Mountain View", 
			"region": "California", 
			"country": "US", 
			"org": "AS15169 Google LLC"
		}`))
	}))
	defer server.Close()

	al := New()
	al.ipInfoCache.SetBaseURL(server.URL + "/")

	ip := "8.8.8.8"
	al.RecordAttempt(ip, false)

	attempts := al.GetRecentAttempts()
	if len(attempts) != 1 {
		t.Fatalf("Expected 1 attempt, got %d", len(attempts))
	}

	if attempts[0].IP != ip {
		t.Errorf("Expected IP %s, got %s", ip, attempts[0].IP)
	}

	if attempts[0].Info == nil {
		t.Fatal("Expected Info to be populated on first attempt")
	}

	if attempts[0].Info.City != "Mountain View" {
		t.Errorf("Expected City Mountain View, got %s", attempts[0].Info.City)
	}
}
