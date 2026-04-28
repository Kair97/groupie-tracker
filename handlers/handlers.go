package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"groupie-tracker/api"
	"groupie-tracker/models"
	"html/template"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"
)

type IndexPageData struct {
	Query       string
	SearchType  string
	Artists     []ArtistCard
	Message     string
	Visuals     VisualizationData
	SearchTypes []SearchTypeOption
}

type ArtistCard struct {
	models.Artist
	LocationCount int
	DateCount     int
	MatchSummary  string
}

type SearchSuggestion struct {
	Value string `json:"value"`
	Type  string `json:"type"`
}

type SearchTypeOption struct {
	Value    string
	Label    string
	Selected bool
}

type SearchResult struct {
	Artist  models.Artist
	Matches []SearchSuggestion
}

type ConcertStop struct {
	Location  string
	Dates     []string
	DateCount int
	BarWidth  int
}

type ArtistPageData struct {
	Artist    models.Artist
	Locations []string
	Dates     []string
	Concerts  []ConcertStop
	MapStops  []MapStop
	Summary   ArtistSummary
}

type MapStop struct {
	Location        string   `json:"location"`
	DisplayLocation string   `json:"displayLocation"`
	Dates           []string `json:"dates"`
	Latitude        float64  `json:"latitude"`
	Longitude       float64  `json:"longitude"`
	Source          string   `json:"source"`
}

type VisualizationData struct {
	TotalArtists      int
	TotalLocations    int
	TotalConcertDates int
	AverageMembers    string
	EarliestCreation  int
	LatestCreation    int
	Decades           []DecadeBucket
	TopTouringArtists []ArtistMetric
}

type DecadeBucket struct {
	Label    string
	Count    int
	BarWidth int
}

type ArtistMetric struct {
	ID            int
	Name          string
	LocationCount int
	DateCount     int
	BarWidth      int
}

type ArtistSummary struct {
	LocationCount int
	DateCount     int
	ConcertCount  int
	PeakStop      ConcertStop
}

type ErrorPageData struct {
	StatusCode int
	Title      string
	Message    string
	ActionURL  string
	ActionText string
}

var templateFuncs = template.FuncMap{
	"formatDate":     formatDate,
	"formatLocation": formatLocation,
	"toJSON":         toJSON,
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.URL.Path != "/" {
		renderError(w, http.StatusNotFound, "Page not found", "The page you requested does not exist. Return to the search dashboard to keep exploring artists.", "/", "Back to dashboard")
		return
	}

	artists, locations, dates, err := loadIndexResources()
	if err != nil {
		http.Error(w, "Failed to fetch artists", http.StatusInternalServerError)
		return
	}

	err = renderIndex(w, IndexPageData{
		SearchType: "all",
		Artists:    buildArtistCards(artists, locations.Index, dates.Index),
		Visuals:    buildVisualizationData(artists, locations.Index, dates.Index),
	})
	if err != nil {
		http.Error(w, "Failed to render page", http.StatusInternalServerError)
	}
}

func ArtistHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Query().Get("id")
	if idStr == "" {
		http.Error(w, "Missing artist ID", http.StatusBadRequest)
		return
	}

	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "Invalid artist ID", http.StatusBadRequest)
		return
	}

	artist, err := api.GetArtist(id)
	if err != nil {
		if errors.Is(err, api.ErrArtistNotFound) {
			http.Error(w, "Artist not found", http.StatusNotFound)
			return
		}

		http.Error(w, "Failed to fetch artist", http.StatusInternalServerError)
		return
	}

	locations, dates, relations, err := loadArtistResources()
	if err != nil {
		http.Error(w, "Failed to fetch artist details", http.StatusInternalServerError)
		return
	}

	locationMap := buildLocationMap(locations.Index)
	dateMap := buildDateMap(dates.Index)

	data := ArtistPageData{
		Artist:    artist,
		Locations: locationMap[id],
		Dates:     dateMap[id],
		Concerts:  buildConcerts(id, relations.Index),
	}
	data.MapStops = buildMapStops(r.Context(), data.Concerts)
	data.Summary = buildArtistSummary(data.Locations, data.Dates, data.Concerts)

	err = renderTemplate(w, "templates/artist.html", data)
	if err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

func SearchHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	searchType := normalizeSearchType(r.URL.Query().Get("type"))

	artists, locations, dates, err := loadIndexResources()
	if err != nil {
		http.Error(w, "Failed to fetch artists", http.StatusInternalServerError)
		return
	}

	if query == "" {
		err = renderIndex(w, IndexPageData{
			SearchType: searchType,
			Artists:    buildArtistCards(artists, locations.Index, dates.Index),
			Message:    "Search by artist, member, location, first album date, or creation year.",
			Visuals:    buildVisualizationData(artists, locations.Index, dates.Index),
		})
		if err != nil {
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
		}
		return
	}

	data := IndexPageData{
		Query:      query,
		SearchType: searchType,
		Visuals:    buildVisualizationData(artists, locations.Index, dates.Index),
	}

	matches := searchArtists(artists, locations.Index, query, searchType)
	data.Artists = buildSearchResultCards(matches, locations.Index, dates.Index)
	if len(data.Artists) > 0 {
		data.Message = fmt.Sprintf("Showing %d %s result%s for \"%s\".", len(data.Artists), searchTypeLabel(searchType), plural(len(data.Artists)), query)
	} else {
		data.Message = fmt.Sprintf("No %s result matched \"%s\".", searchTypeLabel(searchType), query)
	}

	err = renderIndex(w, data)
	if err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

func SuggestHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	searchType := normalizeSearchType(r.URL.Query().Get("type"))
	artists, locations, _, err := loadIndexResources()
	if err != nil {
		http.Error(w, "Failed to fetch suggestions", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(buildSearchSuggestions(artists, locations.Index, query, searchType, 12)); err != nil {
		http.Error(w, "Failed to encode suggestions", http.StatusInternalServerError)
	}
}

func renderIndex(w http.ResponseWriter, data IndexPageData) error {
	data.SearchType = normalizeSearchType(data.SearchType)
	data.SearchTypes = buildSearchTypeOptions(data.SearchType)
	return renderTemplate(w, "templates/index.html", data)
}

func renderTemplate(w http.ResponseWriter, file string, data interface{}) error {
	tmpl, err := template.New(filepath.Base(file)).Funcs(templateFuncs).ParseFiles(file)
	if err != nil {
		return err
	}

	return tmpl.Execute(w, data)
}

func renderError(w http.ResponseWriter, statusCode int, title, message, actionURL, actionText string) {
	w.WriteHeader(statusCode)
	err := renderTemplate(w, "templates/error.html", ErrorPageData{
		StatusCode: statusCode,
		Title:      title,
		Message:    message,
		ActionURL:  actionURL,
		ActionText: actionText,
	})
	if err != nil {
		http.Error(w, http.StatusText(statusCode), statusCode)
	}
}

func loadIndexResources() ([]models.Artist, models.LocationIndex, models.DateIndex, error) {
	var (
		artists   []models.Artist
		locations models.LocationIndex
		dates     models.DateIndex
	)

	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	wg.Add(3)

	go func() {
		defer wg.Done()
		var err error
		artists, err = api.GetArtists()
		if err != nil {
			errCh <- err
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		locations, err = api.GetLocations()
		if err != nil {
			errCh <- err
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		dates, err = api.GetDates()
		if err != nil {
			errCh <- err
		}
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return nil, models.LocationIndex{}, models.DateIndex{}, err
		}
	}

	return artists, locations, dates, nil
}

func loadArtistResources() (models.LocationIndex, models.DateIndex, models.RelationIndex, error) {
	var (
		locations models.LocationIndex
		dates     models.DateIndex
		relations models.RelationIndex
	)

	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	wg.Add(3)

	go func() {
		defer wg.Done()
		var err error
		locations, err = api.GetLocations()
		if err != nil {
			errCh <- err
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		dates, err = api.GetDates()
		if err != nil {
			errCh <- err
		}
	}()

	go func() {
		defer wg.Done()
		var err error
		relations, err = api.GetRelations()
		if err != nil {
			errCh <- err
		}
	}()

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			return models.LocationIndex{}, models.DateIndex{}, models.RelationIndex{}, err
		}
	}

	return locations, dates, relations, nil
}

func buildArtistCards(artists []models.Artist, locations []models.Location, dates []models.Date) []ArtistCard {
	locationMap := buildLocationMap(locations)
	dateMap := buildDateMap(dates)

	cards := make([]ArtistCard, 0, len(artists))
	for _, artist := range artists {
		cards = append(cards, ArtistCard{
			Artist:        artist,
			LocationCount: len(locationMap[artist.ID]),
			DateCount:     len(dateMap[artist.ID]),
		})
	}

	return cards
}

func buildSearchResultCards(results []SearchResult, locations []models.Location, dates []models.Date) []ArtistCard {
	artists := make([]models.Artist, 0, len(results))
	matchSummaries := make(map[int]string, len(results))
	for _, result := range results {
		artists = append(artists, result.Artist)
		matchSummaries[result.Artist.ID] = describeMatches(result.Matches)
	}

	cards := buildArtistCards(artists, locations, dates)
	for i := range cards {
		cards[i].MatchSummary = matchSummaries[cards[i].ID]
	}

	return cards
}

func buildVisualizationData(artists []models.Artist, locations []models.Location, dates []models.Date) VisualizationData {
	locationMap := buildLocationMap(locations)
	dateMap := buildDateMap(dates)

	data := VisualizationData{
		TotalArtists:     len(artists),
		EarliestCreation: 0,
		LatestCreation:   0,
	}

	totalMembers := 0
	decadeCounts := make(map[int]int)
	for i, artist := range artists {
		totalMembers += len(artist.Members)
		if i == 0 || artist.CreationDate < data.EarliestCreation {
			data.EarliestCreation = artist.CreationDate
		}
		if artist.CreationDate > data.LatestCreation {
			data.LatestCreation = artist.CreationDate
		}
		if artist.CreationDate > 0 {
			decadeCounts[(artist.CreationDate/10)*10]++
		}
	}

	for _, location := range locations {
		data.TotalLocations += len(location.Locations)
	}
	for _, date := range dates {
		data.TotalConcertDates += len(date.Dates)
	}
	if len(artists) > 0 {
		data.AverageMembers = fmt.Sprintf("%.1f", float64(totalMembers)/float64(len(artists)))
	}

	data.Decades = buildDecadeBuckets(decadeCounts)
	data.TopTouringArtists = buildTopTouringArtists(artists, locationMap, dateMap)

	return data
}

func buildDecadeBuckets(decadeCounts map[int]int) []DecadeBucket {
	decades := make([]int, 0, len(decadeCounts))
	maxCount := 0
	for decade, count := range decadeCounts {
		decades = append(decades, decade)
		if count > maxCount {
			maxCount = count
		}
	}
	sort.Ints(decades)

	buckets := make([]DecadeBucket, 0, len(decades))
	for _, decade := range decades {
		count := decadeCounts[decade]
		buckets = append(buckets, DecadeBucket{
			Label:    fmt.Sprintf("%ds", decade),
			Count:    count,
			BarWidth: relativeWidth(count, maxCount),
		})
	}

	return buckets
}

func buildTopTouringArtists(artists []models.Artist, locationMap, dateMap map[int][]string) []ArtistMetric {
	metrics := make([]ArtistMetric, 0, len(artists))
	maxLocations := 0
	for _, artist := range artists {
		locationCount := len(locationMap[artist.ID])
		dateCount := len(dateMap[artist.ID])
		if locationCount > maxLocations {
			maxLocations = locationCount
		}
		metrics = append(metrics, ArtistMetric{
			ID:            artist.ID,
			Name:          artist.Name,
			LocationCount: locationCount,
			DateCount:     dateCount,
		})
	}

	sort.Slice(metrics, func(i, j int) bool {
		if metrics[i].LocationCount == metrics[j].LocationCount {
			if metrics[i].DateCount == metrics[j].DateCount {
				return metrics[i].Name < metrics[j].Name
			}
			return metrics[i].DateCount > metrics[j].DateCount
		}
		return metrics[i].LocationCount > metrics[j].LocationCount
	})

	limit := 5
	if len(metrics) < limit {
		limit = len(metrics)
	}
	metrics = metrics[:limit]
	for i := range metrics {
		metrics[i].BarWidth = relativeWidth(metrics[i].LocationCount, maxLocations)
	}

	return metrics
}

func buildLocationMap(locations []models.Location) map[int][]string {
	locationMap := make(map[int][]string, len(locations))

	for _, location := range locations {
		sortedLocations := append([]string(nil), location.Locations...)
		sort.Slice(sortedLocations, func(i, j int) bool {
			return formatLocation(sortedLocations[i]) < formatLocation(sortedLocations[j])
		})
		locationMap[location.ID] = sortedLocations
	}

	return locationMap
}

func buildDateMap(dates []models.Date) map[int][]string {
	dateMap := make(map[int][]string, len(dates))

	for _, date := range dates {
		sortedDates := append([]string(nil), date.Dates...)
		sortDates(sortedDates)
		dateMap[date.ID] = sortedDates
	}

	return dateMap
}

func buildConcerts(id int, relations []models.Relation) []ConcertStop {
	for _, rel := range relations {
		if rel.ID != id {
			continue
		}

		concerts := make([]ConcertStop, 0, len(rel.DatesLocations))
		for location, dates := range rel.DatesLocations {
			sortedDates := append([]string(nil), dates...)
			sortDates(sortedDates)
			concerts = append(concerts, ConcertStop{
				Location:  location,
				Dates:     sortedDates,
				DateCount: len(sortedDates),
			})
		}

		sort.Slice(concerts, func(i, j int) bool {
			return formatLocation(concerts[i].Location) < formatLocation(concerts[j].Location)
		})

		maxDates := 0
		for _, concert := range concerts {
			if concert.DateCount > maxDates {
				maxDates = concert.DateCount
			}
		}
		for i := range concerts {
			concerts[i].BarWidth = relativeWidth(concerts[i].DateCount, maxDates)
		}

		return concerts
	}

	return nil
}

func buildMapStops(ctx context.Context, concerts []ConcertStop) []MapStop {
	orderedConcerts := append([]ConcertStop(nil), concerts...)
	sort.SliceStable(orderedConcerts, func(i, j int) bool {
		left := firstConcertDate(orderedConcerts[i].Dates)
		right := firstConcertDate(orderedConcerts[j].Dates)
		if left.Equal(right) {
			return formatLocation(orderedConcerts[i].Location) < formatLocation(orderedConcerts[j].Location)
		}
		return left.Before(right)
	})

	stops := make([]MapStop, 0, len(orderedConcerts))
	for _, concert := range orderedConcerts {
		coordinate, source, err := api.GeocodeLocation(ctx, concert.Location)
		if err != nil {
			continue
		}

		stops = append(stops, MapStop{
			Location:        concert.Location,
			DisplayLocation: formatLocation(concert.Location),
			Dates:           concert.Dates,
			Latitude:        coordinate.Latitude,
			Longitude:       coordinate.Longitude,
			Source:          source,
		})
	}

	return stops
}

func firstConcertDate(dates []string) time.Time {
	for _, value := range dates {
		date, err := time.Parse("02-01-2006", value)
		if err == nil {
			return date
		}
	}

	return time.Time{}
}

func buildArtistSummary(locations, dates []string, concerts []ConcertStop) ArtistSummary {
	summary := ArtistSummary{
		LocationCount: len(locations),
		DateCount:     len(dates),
		ConcertCount:  len(concerts),
	}

	for _, concert := range concerts {
		if concert.DateCount > summary.PeakStop.DateCount {
			summary.PeakStop = concert
		}
	}

	return summary
}

func relativeWidth(value, maxValue int) int {
	if value <= 0 || maxValue <= 0 {
		return 0
	}

	width := value * 100 / maxValue
	if width < 8 {
		return 8
	}

	return width
}

func searchArtists(artists []models.Artist, locations []models.Location, query string, searchType string) []SearchResult {
	normalizedQuery := normalizeSearch(query)
	if normalizedQuery == "" {
		return nil
	}

	searchType = normalizeSearchType(searchType)
	locationMap := buildRawLocationMap(locations)
	results := make([]SearchResult, 0)
	for _, artist := range artists {
		matches := searchArtistMatches(artist, locationMap[artist.ID], normalizedQuery, searchType)
		if len(matches) > 0 {
			results = append(results, SearchResult{
				Artist:  artist,
				Matches: matches,
			})
		}
	}

	sort.SliceStable(results, func(i, j int) bool {
		left := bestMatchRank(results[i].Matches)
		right := bestMatchRank(results[j].Matches)
		if left == right {
			return results[i].Artist.Name < results[j].Artist.Name
		}
		return left < right
	})

	return results
}

func searchArtistMatches(artist models.Artist, locations []string, normalizedQuery string, searchType string) []SearchSuggestion {
	matches := make([]SearchSuggestion, 0)
	searchType = normalizeSearchType(searchType)

	if matchesType(searchType, "artist") && smartContains(artist.Name, normalizedQuery) {
		matches = append(matches, SearchSuggestion{Value: artist.Name, Type: "artist/band"})
	}

	if matchesType(searchType, "member") {
		for _, member := range artist.Members {
			if smartContains(member, normalizedQuery) {
				matches = append(matches, SearchSuggestion{Value: member, Type: "member"})
			}
		}
	}

	if matchesType(searchType, "location") {
		for _, location := range locations {
			if smartContains(location, normalizedQuery) || smartContains(formatLocation(location), normalizedQuery) {
				matches = append(matches, SearchSuggestion{Value: location, Type: "location"})
			}
		}
	}

	if matchesType(searchType, "album") && matchesAlbumDate(artist.FirstAlbum, normalizedQuery) {
		matches = append(matches, SearchSuggestion{Value: artist.FirstAlbum, Type: "first album date"})
	}

	if matchesType(searchType, "creation") && matchesCreationYear(artist.CreationDate, normalizedQuery) {
		matches = append(matches, SearchSuggestion{Value: strconv.Itoa(artist.CreationDate), Type: "creation year"})
	}

	return matches
}

func buildSearchSuggestions(artists []models.Artist, locations []models.Location, query string, searchType string, limit int) []SearchSuggestion {
	normalizedQuery := normalizeSearch(query)
	if normalizedQuery == "" || limit <= 0 {
		return nil
	}

	searchType = normalizeSearchType(searchType)
	locationMap := buildRawLocationMap(locations)
	seen := make(map[string]bool)
	suggestions := make([]SearchSuggestion, 0, limit)
	for _, artist := range artists {
		for _, candidate := range searchArtistMatches(artist, locationMap[artist.ID], normalizedQuery, searchType) {
			key := strings.ToLower(candidate.Type + "\x00" + candidate.Value)
			if seen[key] {
				continue
			}
			seen[key] = true
			suggestions = append(suggestions, candidate)
		}
	}

	sort.SliceStable(suggestions, func(i, j int) bool {
		left := suggestionRank(suggestions[i])
		right := suggestionRank(suggestions[j])
		if left == right {
			if suggestions[i].Value == suggestions[j].Value {
				return suggestions[i].Type < suggestions[j].Type
			}
			return suggestions[i].Value < suggestions[j].Value
		}
		return left < right
	})

	if len(suggestions) > limit {
		return suggestions[:limit]
	}

	return suggestions
}

func buildRawLocationMap(locations []models.Location) map[int][]string {
	locationMap := make(map[int][]string, len(locations))
	for _, location := range locations {
		locationMap[location.ID] = append([]string(nil), location.Locations...)
	}

	return locationMap
}

func describeMatches(matches []SearchSuggestion) string {
	if len(matches) == 0 {
		return ""
	}

	limit := 3
	if len(matches) < limit {
		limit = len(matches)
	}

	parts := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		parts = append(parts, fmt.Sprintf("%s -> %s", matches[i].Value, matches[i].Type))
	}
	if len(matches) > limit {
		parts = append(parts, fmt.Sprintf("+%d more", len(matches)-limit))
	}

	return strings.Join(parts, ", ")
}

func containsSearch(value, normalizedQuery string) bool {
	return strings.Contains(normalizeSearch(value), normalizedQuery)
}

func bestMatchRank(matches []SearchSuggestion) int {
	rank := 99
	for _, match := range matches {
		if current := suggestionRank(match); current < rank {
			rank = current
		}
	}

	return rank
}

func suggestionRank(suggestion SearchSuggestion) int {
	switch suggestion.Type {
	case "artist/band":
		return 0
	case "member":
		return 1
	case "location":
		return 2
	case "first album date":
		return 3
	case "creation year":
		return 4
	default:
		return 5
	}
}

func buildSearchTypeOptions(selected string) []SearchTypeOption {
	selected = normalizeSearchType(selected)
	options := []SearchTypeOption{
		{Value: "all", Label: "All fields"},
		{Value: "artist", Label: "Artist"},
		{Value: "member", Label: "Member"},
		{Value: "location", Label: "Location"},
		{Value: "creation", Label: "Creation year"},
		{Value: "album", Label: "First album date"},
	}

	for i := range options {
		options[i].Selected = options[i].Value == selected
	}

	return options
}

func normalizeSearchType(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "artist", "member", "location", "creation", "album":
		return strings.ToLower(strings.TrimSpace(value))
	default:
		return "all"
	}
}

func searchTypeLabel(searchType string) string {
	switch normalizeSearchType(searchType) {
	case "artist":
		return "artist"
	case "member":
		return "member"
	case "location":
		return "location"
	case "creation":
		return "creation year"
	case "album":
		return "first album date"
	default:
		return "search"
	}
}

func matchesType(selected, candidate string) bool {
	selected = normalizeSearchType(selected)
	return selected == "all" || selected == candidate
}

func smartContains(value, normalizedQuery string) bool {
	normalizedValue := normalizeSearch(value)
	if strings.Contains(normalizedValue, normalizedQuery) {
		return true
	}

	queryParts := strings.Fields(normalizedQuery)
	if len(queryParts) < 2 {
		return false
	}

	for _, part := range queryParts {
		if !strings.Contains(normalizedValue, part) {
			return false
		}
	}

	return true
}

func matchesCreationYear(creationYear int, normalizedQuery string) bool {
	return strconv.Itoa(creationYear) == strings.TrimSpace(normalizedQuery)
}

func matchesAlbumDate(firstAlbum, normalizedQuery string) bool {
	if containsSearch(firstAlbum, normalizedQuery) {
		return true
	}

	year := albumYear(firstAlbum)
	return len(normalizedQuery) == 4 && year == normalizedQuery
}

func albumYear(firstAlbum string) string {
	parts := strings.Split(firstAlbum, "-")
	if len(parts) != 3 {
		return ""
	}
	return parts[2]
}

func normalizeSearch(value string) string {
	value = strings.ReplaceAll(strings.ToLower(value), "_", " ")
	value = strings.ReplaceAll(value, "-", " ")
	return strings.Join(strings.Fields(value), " ")
}

func formatLocation(location string) string {
	if location == "" {
		return ""
	}

	segments := strings.Split(location, "-")
	formatted := make([]string, 0, len(segments))
	for _, segment := range segments {
		segment = strings.ReplaceAll(segment, "_", " ")
		words := strings.Fields(segment)
		for i, word := range words {
			words[i] = titleWord(word)
		}
		formatted = append(formatted, strings.Join(words, " "))
	}

	return strings.Join(formatted, ", ")
}

func titleWord(word string) string {
	if len(word) <= 3 {
		return strings.ToUpper(word)
	}

	runes := []rune(strings.ToLower(word))
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func formatDate(value string) string {
	date, err := time.Parse("02-01-2006", value)
	if err != nil {
		return value
	}

	return date.Format("02 Jan 2006")
}

func toJSON(value interface{}) template.JS {
	data, err := json.Marshal(value)
	if err != nil {
		return "[]"
	}

	return template.JS(data)
}

func sortDates(dates []string) {
	sort.Slice(dates, func(i, j int) bool {
		left, leftErr := time.Parse("02-01-2006", dates[i])
		right, rightErr := time.Parse("02-01-2006", dates[j])
		if leftErr == nil && rightErr == nil {
			return left.Before(right)
		}

		return dates[i] < dates[j]
	})
}

func plural(count int) string {
	if count == 1 {
		return ""
	}

	return "s"
}
