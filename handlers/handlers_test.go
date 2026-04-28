package handlers

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"groupie-tracker/api"
	"groupie-tracker/models"
)

func TestBuildArtistCardsUsesLocationAndDateCounts(t *testing.T) {
	artists := []models.Artist{
		{ID: 1, Name: "Queen"},
		{ID: 2, Name: "Gorillaz"},
	}

	locations := []models.Location{
		{ID: 1, Locations: []string{"london-uk", "paris-france"}},
		{ID: 2, Locations: []string{"tokyo-japan"}},
	}

	dates := []models.Date{
		{ID: 1, Dates: []string{"02-03-2001", "01-03-2001", "05-03-2001"}},
		{ID: 2, Dates: []string{"10-04-2002"}},
	}

	cards := buildArtistCards(artists, locations, dates)
	if len(cards) != 2 {
		t.Fatalf("expected 2 cards, got %d", len(cards))
	}

	if cards[0].LocationCount != 2 || cards[0].DateCount != 3 {
		t.Fatalf("expected first artist counts 2 locations and 3 dates, got %+v", cards[0])
	}

	if cards[1].LocationCount != 1 || cards[1].DateCount != 1 {
		t.Fatalf("expected second artist counts 1 location and 1 date, got %+v", cards[1])
	}
}

func TestBuildVisualizationDataSummarizesArtistsAndCharts(t *testing.T) {
	artists := []models.Artist{
		{ID: 1, Name: "Queen", Members: []string{"Freddie Mercury", "Brian May"}, CreationDate: 1970},
		{ID: 2, Name: "Daft Punk", Members: []string{"Thomas Bangalter", "Guy-Manuel de Homem-Christo"}, CreationDate: 1993},
		{ID: 3, Name: "Gorillaz", Members: []string{"Damon Albarn"}, CreationDate: 1998},
	}

	locations := []models.Location{
		{ID: 1, Locations: []string{"london-uk", "paris-france", "tokyo-japan"}},
		{ID: 2, Locations: []string{"paris-france"}},
		{ID: 3, Locations: []string{"new_york-usa", "berlin-germany"}},
	}

	dates := []models.Date{
		{ID: 1, Dates: []string{"02-03-2001", "01-03-2001"}},
		{ID: 2, Dates: []string{"10-04-2002"}},
		{ID: 3, Dates: []string{"11-04-2002", "12-04-2002", "13-04-2002"}},
	}

	data := buildVisualizationData(artists, locations, dates)

	if data.TotalArtists != 3 || data.TotalLocations != 6 || data.TotalConcertDates != 6 {
		t.Fatalf("unexpected totals: %+v", data)
	}

	if data.AverageMembers != "1.7" {
		t.Fatalf("expected average members 1.7, got %q", data.AverageMembers)
	}

	if data.EarliestCreation != 1970 || data.LatestCreation != 1998 {
		t.Fatalf("unexpected era range: %+v", data)
	}

	if len(data.Decades) != 2 || data.Decades[0].Label != "1970s" || data.Decades[1].Count != 2 {
		t.Fatalf("unexpected decade buckets: %+v", data.Decades)
	}

	if len(data.TopTouringArtists) != 3 || data.TopTouringArtists[0].Name != "Queen" || data.TopTouringArtists[0].BarWidth != 100 {
		t.Fatalf("unexpected top touring artists: %+v", data.TopTouringArtists)
	}
}

func TestBuildConcertsAddsActivityMetrics(t *testing.T) {
	relations := []models.Relation{
		{
			ID: 1,
			DatesLocations: map[string][]string{
				"paris-france": []string{"02-03-2001", "01-03-2001"},
				"london-uk":    []string{"10-04-2002"},
			},
		},
	}

	concerts := buildConcerts(1, relations)
	if len(concerts) != 2 {
		t.Fatalf("expected 2 concerts, got %d", len(concerts))
	}

	if concerts[0].Location != "london-uk" || concerts[0].DateCount != 1 {
		t.Fatalf("expected concerts sorted by formatted location, got %+v", concerts)
	}

	if concerts[1].BarWidth != 100 {
		t.Fatalf("expected largest activity bar to be 100, got %+v", concerts[1])
	}
}

func TestBuildMapStopsOrdersConcertsHistorically(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Query().Get("q") {
		case "berlin germany":
			_, _ = w.Write([]byte(`[{"lat":"52.5200","lon":"13.4050"}]`))
		case "los angeles United States":
			_, _ = w.Write([]byte(`[{"lat":"34.0522","lon":"-118.2437"}]`))
		default:
			_, _ = w.Write([]byte(`[]`))
		}
	}))
	defer server.Close()

	previousURL := api.GeocodeBaseURL
	api.GeocodeBaseURL = server.URL
	t.Cleanup(func() {
		api.GeocodeBaseURL = previousURL
	})

	stops := buildMapStops(context.Background(), []ConcertStop{
		{Location: "los_angeles-usa", Dates: []string{"12-08-2021"}},
		{Location: "berlin-germany", Dates: []string{"02-01-2020"}},
	})

	if len(stops) != 2 {
		t.Fatalf("expected two mapped stops, got %+v", stops)
	}
	if stops[0].Location != "berlin-germany" || stops[1].Location != "los_angeles-usa" {
		t.Fatalf("expected historical route order, got %+v", stops)
	}
	if stops[0].Latitude != 52.52 || stops[1].Longitude != -118.2437 {
		t.Fatalf("unexpected mapped coordinates: %+v", stops)
	}
}

func TestSearchArtistsMatchesRequiredFields(t *testing.T) {
	artists := []models.Artist{
		{ID: 1, Name: "Scorpions", Members: []string{"Klaus Meine"}, CreationDate: 1965, FirstAlbum: "09-02-1972"},
		{ID: 2, Name: "The Jimi Hendrix Experience", Members: []string{"Jimi Hendrix"}, CreationDate: 1966, FirstAlbum: "12-05-1967"},
		{ID: 3, Name: "Pink Floyd", Members: []string{"David Gilmour"}, CreationDate: 1965, FirstAlbum: "05-08-1967"},
		{ID: 4, Name: "ACDC", Members: []string{"Angus Young"}, CreationDate: 1973, FirstAlbum: "01-01-1975"},
		{ID: 5, Name: "Queen", Members: []string{"Freddie Mercury"}, CreationDate: 1970, FirstAlbum: "13-07-1973"},
		{ID: 6, Name: "Queensland", Members: []string{"Example Member"}, CreationDate: 2001, FirstAlbum: "01-01-2002"},
	}
	locations := []models.Location{
		{ID: 3, Locations: []string{"london-uk"}},
	}

	memberResults := searchArtists(artists, locations, "Jimi Hendrix", "member")
	if len(memberResults) != 1 || memberResults[0].Artist.Name != "The Jimi Hendrix Experience" {
		t.Fatalf("expected member search to return The Jimi Hendrix Experience, got %+v", memberResults)
	}

	locationResults := searchArtists(artists, locations, "london-uk", "location")
	if len(locationResults) != 1 || locationResults[0].Artist.Name != "Pink Floyd" {
		t.Fatalf("expected location search to return Pink Floyd, got %+v", locationResults)
	}

	albumResults := searchArtists(artists, locations, "05-08-1967", "album")
	if len(albumResults) != 1 || albumResults[0].Artist.Name != "Pink Floyd" {
		t.Fatalf("expected first album search to return Pink Floyd, got %+v", albumResults)
	}

	creationResults := searchArtists(artists, locations, "1965", "creation")
	if len(creationResults) != 2 || creationResults[0].Artist.Name != "Pink Floyd" || creationResults[1].Artist.Name != "Scorpions" {
		t.Fatalf("expected creation date search to return Pink Floyd and Scorpions, got %+v", creationResults)
	}

	queenResults := searchArtists(artists, locations, "queen", "artist")
	if len(queenResults) != 2 || queenResults[0].Artist.Name != "Queen" || queenResults[1].Artist.Name != "Queensland" {
		t.Fatalf("expected case-insensitive queen search to return Queen and Queensland, got %+v", queenResults)
	}
}

func TestSearchArtistsSeparatesCreationAndAlbumDates(t *testing.T) {
	artists := []models.Artist{
		{ID: 1, Name: "Creation Match", CreationDate: 1973, FirstAlbum: "01-01-1980"},
		{ID: 2, Name: "Album Match", CreationDate: 1980, FirstAlbum: "13-07-1973"},
	}

	creationResults := searchArtists(artists, nil, "1973", "creation")
	if len(creationResults) != 1 || creationResults[0].Artist.Name != "Creation Match" {
		t.Fatalf("expected only the creation-year artist, got %+v", creationResults)
	}

	albumResults := searchArtists(artists, nil, "1973", "album")
	if len(albumResults) != 1 || albumResults[0].Artist.Name != "Album Match" {
		t.Fatalf("expected only the first-album-date artist, got %+v", albumResults)
	}
}

func TestBuildSearchSuggestionsLabelsTypes(t *testing.T) {
	artists := []models.Artist{
		{ID: 1, Name: "Green Day", Members: []string{"Billie Joe Armstrong"}, CreationDate: 1987, FirstAlbum: "10-04-1990"},
		{ID: 2, Name: "Queen", Members: []string{"Freddie Mercury"}, CreationDate: 1970, FirstAlbum: "13-07-1973"},
	}
	locations := []models.Location{
		{ID: 1, Locations: []string{"saitama-japan", "osaka-japan", "nagoya-japan"}},
	}

	memberSuggestions := buildSearchSuggestions(artists, locations, "Billie Joe", "member", 12)
	if len(memberSuggestions) == 0 || memberSuggestions[0].Value != "Billie Joe Armstrong" || memberSuggestions[0].Type != "member" {
		t.Fatalf("expected Billie Joe Armstrong member suggestion, got %+v", memberSuggestions)
	}

	locationSuggestions := buildSearchSuggestions(artists, locations, "Japan", "location", 12)
	values := make(map[string]string)
	for _, suggestion := range locationSuggestions {
		values[suggestion.Value] = suggestion.Type
	}
	for _, location := range []string{"saitama-japan", "osaka-japan", "nagoya-japan"} {
		if values[location] != "location" {
			t.Fatalf("expected %s location suggestion, got %+v", location, locationSuggestions)
		}
	}
}

func TestIndexHandlerRendersStyled404ForUnknownRoute(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	root := filepath.Dir(wd)
	if err := os.Chdir(root); err != nil {
		t.Fatalf("failed to enter project root: %v", err)
	}
	t.Cleanup(func() {
		if err := os.Chdir(wd); err != nil {
			t.Fatalf("failed to restore working directory: %v", err)
		}
	})

	req := httptest.NewRequest(http.MethodGet, "/missing-page", nil)
	rec := httptest.NewRecorder()

	IndexHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}

	if body := rec.Body.String(); !strings.Contains(body, "Page not found") || !strings.Contains(body, "Back to dashboard") {
		t.Fatalf("expected styled 404 page, got %q", body)
	}
}

func TestArtistHandlerReturns404ForUnknownArtist(t *testing.T) {
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

	previousBaseURL := api.BaseURL
	api.BaseURL = server.URL + "/api"
	t.Cleanup(func() {
		api.BaseURL = previousBaseURL
	})

	req := httptest.NewRequest(http.MethodGet, "/artist?id=99999", nil)
	rec := httptest.NewRecorder()

	ArtistHandler(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestTemplatesRenderWithEndpointData(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}

	root := filepath.Dir(wd)

	indexRecorder := httptest.NewRecorder()
	indexData := IndexPageData{
		Query: "queen",
		Artists: []ArtistCard{
			{
				Artist: models.Artist{
					ID:           1,
					Name:         "Queen",
					Image:        "queen.jpg",
					Members:      []string{"Freddie Mercury", "Brian May"},
					CreationDate: 1970,
					FirstAlbum:   "13-07-1973",
				},
				LocationCount: 2,
				DateCount:     3,
			},
		},
	}

	if err := renderTemplate(indexRecorder, filepath.Join(root, "templates", "index.html"), indexData); err != nil {
		t.Fatalf("expected index template to render, got %v", err)
	}

	artistRecorder := httptest.NewRecorder()
	artistData := ArtistPageData{
		Artist: models.Artist{
			ID:           1,
			Name:         "Queen",
			Image:        "queen.jpg",
			Members:      []string{"Freddie Mercury", "Brian May"},
			CreationDate: 1970,
			FirstAlbum:   "13-07-1973",
		},
		Locations: []string{"london-uk", "paris-france"},
		Dates:     []string{"01-03-2001", "02-03-2001"},
		Concerts: []ConcertStop{
			{
				Location: "london-uk",
				Dates:    []string{"01-03-2001"},
			},
		},
	}

	if err := renderTemplate(artistRecorder, filepath.Join(root, "templates", "artist.html"), artistData); err != nil {
		t.Fatalf("expected artist template to render, got %v", err)
	}
}
