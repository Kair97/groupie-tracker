package api

import (
	"encoding/json"
	"fmt"
	"groupie-tracker/models"
	"net/http"
	"time"
)

const URL = "https://groupietrackers.herokuapp.com/api"

var client = &http.Client{
	Timeout: 10 * time.Second,
}

func fetch(url string, target interface{}) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("failed to fetch %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("bad status from %s: %s", url, resp.Status)
	}

	err = json.NewDecoder(resp.Body).Decode(target)
	if err != nil {
		return fmt.Errorf("failed to decode response from %s: %w", url, err)
	}

	return nil

}

func GetArtists() ([]models.Artist, error) {
	var artists []models.Artist

	err := fetch(URL+"/artists", &artists)
	if err != nil {
		return nil, err
	}

	return artists, nil
}

func GetArtist(id int) (models.Artist, error) {
	var artist models.Artist

	err := fetch(fmt.Sprintf("%s/artists/%d", URL, id), &artist)
	if err != nil {
		return models.Artist{}, err
	}

	return artist, nil
}

func GetLocations() (models.LocationIndex, error) {
	var locations models.LocationIndex

	err := fetch(URL+"/locations", &locations)
	if err != nil {
		return models.LocationIndex{}, err
	}

	return locations, nil

}

func GetDates() (models.DateIndex, error) {
	var dates models.DateIndex

	err := fetch(URL+"/dates", &dates)
	if err != nil {
		return models.DateIndex{}, err
	}

	return dates, nil
}

func GetRelations() (models.RelationIndex, error) {
	var relations models.RelationIndex

	err := fetch(URL+"/relation", &relations)
	if err != nil {
		return models.RelationIndex{}, err
	}

	return relations, nil
}
