package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

type Coordinate struct {
	Latitude  float64
	Longitude float64
}

var GeocodeBaseURL = "https://nominatim.openstreetmap.org/search"

type geocodeResult struct {
	Lat string `json:"lat"`
	Lon string `json:"lon"`
}

func GeocodeLocation(ctx context.Context, location string) (Coordinate, string, error) {
	if fallback, ok := fallbackCoordinates[normalizeAddress(location)]; ok {
		return fallback, "local fallback", nil
	}

	address := geocodeQuery(location)
	coordinate, err := geocodeWithNominatim(ctx, address)
	if err == nil {
		return coordinate, "geocoded", nil
	}

	return Coordinate{}, "", err
}

func geocodeWithNominatim(ctx context.Context, address string) (Coordinate, error) {
	endpoint, err := url.Parse(GeocodeBaseURL)
	if err != nil {
		return Coordinate{}, fmt.Errorf("invalid geocode endpoint: %w", err)
	}

	query := endpoint.Query()
	query.Set("format", "json")
	query.Set("limit", "1")
	query.Set("q", address)
	endpoint.RawQuery = query.Encode()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint.String(), nil)
	if err != nil {
		return Coordinate{}, err
	}
	req.Header.Set("User-Agent", "groupie-tracker-geolocalization/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return Coordinate{}, fmt.Errorf("failed to geocode %q: %w", address, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return Coordinate{}, fmt.Errorf("bad status from geocoder for %q: %s", address, resp.Status)
	}

	var results []geocodeResult
	if err := json.NewDecoder(resp.Body).Decode(&results); err != nil {
		return Coordinate{}, fmt.Errorf("failed to decode geocoder response for %q: %w", address, err)
	}
	if len(results) == 0 {
		return Coordinate{}, fmt.Errorf("no geocoding result for %q", address)
	}

	lat, err := strconv.ParseFloat(results[0].Lat, 64)
	if err != nil {
		return Coordinate{}, fmt.Errorf("invalid latitude for %q: %w", address, err)
	}
	lon, err := strconv.ParseFloat(results[0].Lon, 64)
	if err != nil {
		return Coordinate{}, fmt.Errorf("invalid longitude for %q: %w", address, err)
	}

	return Coordinate{Latitude: lat, Longitude: lon}, nil
}

func geocodeQuery(location string) string {
	parts := strings.Fields(strings.ReplaceAll(strings.ReplaceAll(location, "_", " "), "-", " "))
	for i, part := range parts {
		switch strings.ToLower(part) {
		case "uk":
			parts[i] = "United Kingdom"
		case "usa":
			parts[i] = "United States"
		case "italia":
			parts[i] = "Italy"
		}
	}

	return strings.Join(parts, " ")
}

func normalizeAddress(location string) string {
	return strings.Join(strings.Fields(strings.ToLower(strings.ReplaceAll(strings.ReplaceAll(location, "_", " "), "-", " "))), " ")
}

var fallbackCoordinates = map[string]Coordinate{
	"aarhus denmark":        {Latitude: 56.1629, Longitude: 10.2039},
	"alabama usa":           {Latitude: 32.3182, Longitude: -86.9023},
	"athens greece":         {Latitude: 37.9838, Longitude: 23.7275},
	"berlin germany":        {Latitude: 52.52, Longitude: 13.405},
	"california usa":        {Latitude: 36.7783, Longitude: -119.4179},
	"canton usa":            {Latitude: 40.7989, Longitude: -81.3784},
	"charlotte usa":         {Latitude: 35.2271, Longitude: -80.8431},
	"chicago usa":           {Latitude: 41.8781, Longitude: -87.6298},
	"columbia usa":          {Latitude: 34.0007, Longitude: -81.0348},
	"del mar usa":           {Latitude: 32.9595, Longitude: -117.2653},
	"dunedin new zealand":   {Latitude: -45.8788, Longitude: 170.5028},
	"dusseldorf germany":    {Latitude: 51.2277, Longitude: 6.7735},
	"florence italia":       {Latitude: 43.7696, Longitude: 11.2558},
	"georgia usa":           {Latitude: 32.1656, Longitude: -82.9001},
	"grand rapids usa":      {Latitude: 42.9634, Longitude: -85.6681},
	"hershey usa":           {Latitude: 40.2859, Longitude: -76.6502},
	"houston usa":           {Latitude: 29.7604, Longitude: -95.3698},
	"indianapolis usa":      {Latitude: 39.7684, Longitude: -86.1581},
	"inglewood usa":         {Latitude: 33.9617, Longitude: -118.3531},
	"kansas city usa":       {Latitude: 39.0997, Longitude: -94.5786},
	"landgraaf netherlands": {Latitude: 50.9058, Longitude: 6.0215},
	"las vegas usa":         {Latitude: 36.1716, Longitude: -115.1391},
	"lisbon portugal":       {Latitude: 38.7223, Longitude: -9.1393},
	"los angeles usa":       {Latitude: 34.0522, Longitude: -118.2437},
	"madrid spain":          {Latitude: 40.4168, Longitude: -3.7038},
	"manchester uk":         {Latitude: 53.4808, Longitude: -2.2426},
	"massachusetts usa":     {Latitude: 42.4072, Longitude: -71.3824},
	"mexico city mexico":    {Latitude: 19.4326, Longitude: -99.1332},
	"montreal canada":       {Latitude: 45.5019, Longitude: -73.5674},
	"montreal usa":          {Latitude: 45.5019, Longitude: -73.5674},
	"monterrey mexico":      {Latitude: 25.6866, Longitude: -100.3161},
	"nagoya japan":          {Latitude: 35.1815, Longitude: 136.9066},
	"new york usa":          {Latitude: 40.7128, Longitude: -74.006},
	"newark usa":            {Latitude: 40.7357, Longitude: -74.1724},
	"north carolina usa":    {Latitude: 35.7596, Longitude: -79.0193},
	"oakland usa":           {Latitude: 37.8044, Longitude: -122.2712},
	"omaha usa":             {Latitude: 41.2565, Longitude: -95.9345},
	"osaka japan":           {Latitude: 34.6937, Longitude: 135.5023},
	"penrose new zealand":   {Latitude: -36.9093, Longitude: 174.8159},
	"philadelphia usa":      {Latitude: 39.9526, Longitude: -75.1652},
	"pittsburgh usa":        {Latitude: 40.4406, Longitude: -79.9959},
	"quebec canada":         {Latitude: 46.8139, Longitude: -71.208},
	"rio de janeiro brazil": {Latitude: -22.9068, Longitude: -43.1729},
	"riyadh saudi arabia":   {Latitude: 24.7136, Longitude: 46.6753},
	"rosemont usa":          {Latitude: 41.9953, Longitude: -87.8845},
	"saitama japan":         {Latitude: 35.8617, Longitude: 139.6455},
	"st louis usa":          {Latitude: 38.627, Longitude: -90.1994},
	"toronto canada":        {Latitude: 43.6532, Longitude: -79.3832},
	"toronto usa":           {Latitude: 43.6532, Longitude: -79.3832},
	"uniondale usa":         {Latitude: 40.7004, Longitude: -73.5929},
	"washington usa":        {Latitude: 38.9072, Longitude: -77.0369},
}
