package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestGeocodeLocationUsesGeocoderAPI(t *testing.T) {
	var requestedQuery string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestedQuery = r.URL.Query().Get("q")
		if r.Header.Get("User-Agent") == "" {
			t.Fatalf("expected geocoder user agent")
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`[{"lat":"49.5938","lon":"8.15052"}]`))
	}))
	defer server.Close()

	previousURL := GeocodeBaseURL
	GeocodeBaseURL = server.URL
	t.Cleanup(func() {
		GeocodeBaseURL = previousURL
	})

	coordinate, source, err := GeocodeLocation(context.Background(), "mainz-germany")
	if err != nil {
		t.Fatalf("expected geocoding to succeed, got %v", err)
	}

	if source != "geocoded" {
		t.Fatalf("expected geocoded source, got %q", source)
	}
	if requestedQuery != "mainz germany" {
		t.Fatalf("expected normalized geocode query, got %q", requestedQuery)
	}
	if coordinate.Latitude != 49.5938 || coordinate.Longitude != 8.15052 {
		t.Fatalf("unexpected coordinate: %+v", coordinate)
	}
}

func TestGeocodeLocationFallsBackForKnownDatasetLocation(t *testing.T) {
	previousURL := GeocodeBaseURL
	GeocodeBaseURL = "http://127.0.0.1:1/search"
	t.Cleanup(func() {
		GeocodeBaseURL = previousURL
	})

	coordinate, source, err := GeocodeLocation(context.Background(), "north_carolina-usa")
	if err != nil {
		t.Fatalf("expected fallback coordinate, got %v", err)
	}
	if source != "local fallback" {
		t.Fatalf("expected local fallback source, got %q", source)
	}
	if coordinate.Latitude == 0 || coordinate.Longitude == 0 {
		t.Fatalf("expected non-zero fallback coordinate, got %+v", coordinate)
	}
}

func TestGeocodeQueryExpandsCountryAliases(t *testing.T) {
	query := geocodeQuery("manchester-uk")
	if !strings.Contains(query, "United Kingdom") {
		t.Fatalf("expected UK alias expansion, got %q", query)
	}
}
