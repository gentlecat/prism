package ipinfo

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLookup(t *testing.T) {
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

	retriever := NewRetriever()
	retriever.SetBaseURL(server.URL + "/")
	ip := "8.8.8.8"

	// Should be synchronous now
	info := retriever.Lookup(ip)
	if info == nil {
		t.Fatal("Expected non-nil on first lookup")
	}

	if info.City != "Mountain View" {
		t.Errorf("Expected City to be Mountain View, got %s", info.City)
	}
}
