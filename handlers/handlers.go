package handlers

import (
	"fmt"
	"groupie-tracker/api"
	"groupie-tracker/models"
	"html/template"
	"net/http"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
	"unicode"
)

type IndexPageData struct {
	Query   string
	Artists []models.Artist
	Message string
}

type ConcertStop struct {
	Location string
	Dates    []string
}

type ArtistPageData struct {
	Artist   models.Artist
	Concerts []ConcertStop
}

var templateFuncs = template.FuncMap{
	"formatDate":     formatDate,
	"formatLocation": formatLocation,
}

func IndexHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if r.URL.Path != "/" {
		http.Error(w, "404 - Page Not Found", http.StatusNotFound)
		return
	}

	artists, err := api.GetArtists()
	if err != nil {
		http.Error(w, "Failed to fetch artists", http.StatusInternalServerError)
		return
	}

	err = renderIndex(w, IndexPageData{
		Artists: artists,
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
		http.Error(w, "Artist not found", http.StatusNotFound)
		return
	}

	relations, err := api.GetRelations()
	if err != nil {
		http.Error(w, "Failed to fetch relations", http.StatusInternalServerError)
		return
	}

	data := ArtistPageData{
		Artist:   artist,
		Concerts: buildConcerts(id, relations.Index),
	}

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

	artists, err := api.GetArtists()
	if err != nil {
		http.Error(w, "Failed to fetch artists", http.StatusInternalServerError)
		return
	}

	if query == "" {
		err = renderIndex(w, IndexPageData{
			Artists: artists,
			Message: "Type an artist name to search the collection.",
		})
		if err != nil {
			http.Error(w, "Failed to render template", http.StatusInternalServerError)
		}
		return
	}

	data := IndexPageData{
		Query: query,
	}

	data.Artists = findArtists(artists, query)
	if len(data.Artists) > 0 {
		data.Message = fmt.Sprintf("Showing %d result%s for \"%s\".", len(data.Artists), plural(len(data.Artists)), query)
	} else {
		data.Artists = suggestArtists(artists, query)
		if len(data.Artists) > 0 {
			data.Message = fmt.Sprintf("No exact match for \"%s\". Similar artists you may mean:", query)
		} else {
			data.Message = fmt.Sprintf("No artist found for \"%s\".", query)
		}
	}

	err = renderIndex(w, data)
	if err != nil {
		http.Error(w, "Failed to render template", http.StatusInternalServerError)
	}
}

func renderIndex(w http.ResponseWriter, data IndexPageData) error {
	return renderTemplate(w, "templates/index.html", data)
}

func renderTemplate(w http.ResponseWriter, file string, data interface{}) error {
	tmpl, err := template.New(filepath.Base(file)).Funcs(templateFuncs).ParseFiles(file)
	if err != nil {
		return err
	}

	return tmpl.Execute(w, data)
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
				Location: location,
				Dates:    sortedDates,
			})
		}

		sort.Slice(concerts, func(i, j int) bool {
			return formatLocation(concerts[i].Location) < formatLocation(concerts[j].Location)
		})

		return concerts
	}

	return nil
}

func findArtists(artists []models.Artist, query string) []models.Artist {
	normalizedQuery := normalize(query)
	if normalizedQuery == "" {
		return nil
	}

	var matches []models.Artist
	for _, artist := range artists {
		if strings.Contains(normalize(artist.Name), normalizedQuery) {
			matches = append(matches, artist)
		}
	}

	return matches
}

func suggestArtists(artists []models.Artist, query string) []models.Artist {
	normalizedQuery := normalize(query)
	queryTokens := uniqueTokens(query)

	type scoredArtist struct {
		artist models.Artist
		score  int
	}

	var scored []scoredArtist
	for _, artist := range artists {
		name := normalize(artist.Name)
		score := tokenMatchScore(queryTokens, uniqueTokens(artist.Name))

		if distance := levenshtein(normalizedQuery, name); distance <= 2 {
			score += 3
		}

		if score >= suggestionThreshold(len(queryTokens)) {
			scored = append(scored, scoredArtist{
				artist: artist,
				score:  score,
			})
		}
	}

	sort.Slice(scored, func(i, j int) bool {
		if scored[i].score == scored[j].score {
			return scored[i].artist.Name < scored[j].artist.Name
		}
		return scored[i].score > scored[j].score
	})

	limit := 6
	if len(scored) < limit {
		limit = len(scored)
	}

	suggestions := make([]models.Artist, 0, limit)
	for i := 0; i < limit; i++ {
		suggestions = append(suggestions, scored[i].artist)
	}

	return suggestions
}

func tokenMatchScore(queryTokens, nameTokens []string) int {
	score := 0
	for _, queryToken := range queryTokens {
		for _, nameToken := range nameTokens {
			switch {
			case queryToken == nameToken:
				score += 3
				goto nextToken
			case strings.Contains(nameToken, queryToken), strings.Contains(queryToken, nameToken):
				score += 2
				goto nextToken
			}
		}
	nextToken:
	}

	return score
}

func suggestionThreshold(tokenCount int) int {
	switch {
	case tokenCount >= 3:
		return 4
	case tokenCount == 2:
		return 3
	default:
		return 2
	}
}

func uniqueTokens(value string) []string {
	parts := strings.FieldsFunc(strings.ToLower(value), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})

	seen := make(map[string]bool)
	tokens := make([]string, 0, len(parts))
	for _, part := range parts {
		if part == "" || seen[part] {
			continue
		}

		seen[part] = true
		tokens = append(tokens, part)
	}

	return tokens
}

func normalize(value string) string {
	return strings.Join(uniqueTokens(value), " ")
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

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}

	if a == "" {
		return len([]rune(b))
	}

	if b == "" {
		return len([]rune(a))
	}

	aRunes := []rune(a)
	bRunes := []rune(b)

	previous := make([]int, len(bRunes)+1)
	for j := range previous {
		previous[j] = j
	}

	for i, aRune := range aRunes {
		current := make([]int, len(bRunes)+1)
		current[0] = i + 1

		for j, bRune := range bRunes {
			cost := 0
			if aRune != bRune {
				cost = 1
			}

			current[j+1] = min3(
				current[j]+1,
				previous[j+1]+1,
				previous[j]+cost,
			)
		}

		previous = current
	}

	return previous[len(bRunes)]
}

func min3(a, b, c int) int {
	if a < b && a < c {
		return a
	}

	if b < c {
		return b
	}

	return c
}
