package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestGetArtistReturnsNotFoundOnEmptyResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/artists/99999":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	previousBaseURL := BaseURL
	BaseURL = server.URL + "/api"
	t.Cleanup(func() {
		BaseURL = previousBaseURL
	})

	_, err := GetArtist(99999)
	if !errors.Is(err, ErrArtistNotFound) {
		t.Fatalf("expected ErrArtistNotFound, got %v", err)
	}
}

func TestGetArtistReturnsArtistData(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/artists/1":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"id":1,"name":"Queen","image":"queen.jpg","members":["Freddie Mercury"],"creationDate":1970,"firstAlbum":"13-07-1973"}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	previousBaseURL := BaseURL
	BaseURL = server.URL + "/api"
	t.Cleanup(func() {
		BaseURL = previousBaseURL
	})

	artist, err := GetArtist(1)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if artist.ID != 1 || artist.Name != "Queen" {
		t.Fatalf("expected artist Queen with ID 1, got %+v", artist)
	}
}
