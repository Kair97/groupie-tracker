# Groupie Tracker Search Bar

Groupie Tracker Search Bar is a Go web application that fetches music data from the public Groupie Tracker API and lets users search the full data system from one responsive interface.

The search supports artist/band names, members, concert locations, first album dates, and creation dates. Input is case-insensitive, and the search field provides typed suggestions that identify each suggestion type, such as `Billie Joe Armstrong -> member` or `saitama-japan -> location`.

## Features

- Backend written in Go using only standard Go packages
- Search by artist/band name
- Search by band member
- Search by concert location
- Search by first album date
- Search by creation date
- Case-insensitive matching
- Live typing suggestions from a JSON endpoint
- Suggestion labels for `artist/band`, `member`, `location`, `first album`, and `creation date`
- Artist cards showing the matched field when a search is active
- Artist detail pages with locations, dates, relation timeline, and geolocalized map route
- Styled error pages and HTTP method/status handling
- Unit tests for search behavior, suggestions, API handling, templates, and route errors

## Tech Stack

- Go
- HTML templates
- CSS
- Vanilla JavaScript
- Groupie Tracker public API
- Nominatim/OpenStreetMap geocoding API
- Leaflet map display

## Project Structure

```text
.
|-- api/
|   |-- api.go
|   |-- api_test.go
|   |-- geocode.go
|   `-- geocode_test.go
|-- handlers/
|   |-- handlers.go
|   `-- handlers_test.go
|-- models/
|   `-- models.go
|-- static/
|   |-- map.js
|   |-- search.js
|   |-- style.css
|   `-- ui.js
|-- templates/
|   |-- artist.html
|   |-- error.html
|   `-- index.html
|-- go.mod
|-- main.go
`-- README.md
```

## Run

```bash
go run .
```

Open:

```text
http://localhost:8080
```

Try searches such as:

- `Billie Joe`
- `Japan`
- `Scorpions`
- `Jimi Hendrix`
- `Phil Collins`
- `london-uk`
- `queen`
- `05-08-1967`
- `1973`
- `1965`

## Test

```bash
go test ./...
```

PowerShell with a local Go cache:

```powershell
$env:GOCACHE = Join-Path (Get-Location) ".gocache"; go test ./...
```

## Routes

- `/` shows the search dashboard and all artists
- `/search?q=<text>` searches across artist names, members, locations, first album dates, and creation dates
- `/suggest?q=<text>` returns JSON suggestions for the active search text
- `/artist?id=<id>` shows one artist with detailed data and a map
- Unknown routes return a styled `404 Not Found` page

## Authors

- Kairzhan
- Asylbek
