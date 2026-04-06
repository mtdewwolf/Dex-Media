package scraper

import (
	"dex/backend/db"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type TMDBResult struct {
	ID           int     `json:"id"`
	Title        string  `json:"title,omitempty"`
	Name         string  `json:"name,omitempty"`
	PosterPath   string  `json:"poster_path"`
	Overview     string  `json:"overview"`
	ReleaseDate  string  `json:"release_date,omitempty"`
	FirstAirDate string  `json:"first_air_date,omitempty"`
	VoteAverage  float64 `json:"vote_average"`
	GenreIDs     []int   `json:"genre_ids"`
}

type TMDBGenre struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type TMDBResponse struct {
	Results []TMDBResult `json:"results"`
}

var genreCache = make(map[string]map[int]string)

func getGenreNames(apiKey, mediaType string, ids []int) string {
	if genreCache[mediaType] == nil {
		apiURL := fmt.Sprintf("https://api.themoviedb.org/3/genre/%s/list?api_key=%s", mediaType, apiKey)
		resp, err := http.Get(apiURL)
		if err == nil {
			defer resp.Body.Close()
			var data struct {
				Genres []TMDBGenre `json:"genres"`
			}
			json.NewDecoder(resp.Body).Decode(&data)
			genreCache[mediaType] = make(map[int]string)
			for _, g := range data.Genres {
				genreCache[mediaType][g.ID] = g.Name
			}
		}
	}

	var names []string
	for _, id := range ids {
		if name, ok := genreCache[mediaType][id]; ok {
			names = append(names, name)
		}
	}
	return strings.Join(names, ", ")
}

func ScrapeMetadata() {
	apiKey := os.Getenv("TMDB_API_KEY")
	if apiKey == "" {
		log.Println("TMDB_API_KEY not set, skipping metadata scraping.")
		return
	}

	scrapeMovies(apiKey)
	scrapeSeries(apiKey)
}

func scrapeMovies(apiKey string) {
	rows, err := db.DB.Query("SELECT id, title FROM media WHERE type = 'movie' AND tmdb_id IS NULL")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			continue
		}

		result, err := searchTMDB(apiKey, "movie", title)
		if err == nil {
			posterURL := "https://image.tmdb.org/t/p/w500" + result.PosterPath
			genres := getGenreNames(apiKey, "movie", result.GenreIDs)
			db.DB.Exec("UPDATE media SET tmdb_id = ?, poster_url = ?, plot = ?, rating = ?, year = ?, genres = ? WHERE id = ?",
				fmt.Sprint(result.ID), posterURL, result.Overview, result.VoteAverage, extractYear(result.ReleaseDate), genres, id)
			log.Printf("Updated metadata for movie: %s\n", title)
		}
	}
}

func scrapeSeries(apiKey string) {
	rows, err := db.DB.Query("SELECT id, title FROM series WHERE tmdb_id IS NULL")
	if err != nil {
		return
	}
	defer rows.Close()

	for rows.Next() {
		var id int
		var title string
		if err := rows.Scan(&id, &title); err != nil {
			continue
		}

		result, err := searchTMDB(apiKey, "tv", title)
		if err == nil {
			posterURL := "https://image.tmdb.org/t/p/w500" + result.PosterPath
			genres := getGenreNames(apiKey, "tv", result.GenreIDs)
			db.DB.Exec("UPDATE series SET tmdb_id = ?, poster_url = ?, plot = ?, rating = ?, year = ?, genres = ? WHERE id = ?",
				fmt.Sprint(result.ID), posterURL, result.Overview, result.VoteAverage, extractYear(result.FirstAirDate), genres, id)
			log.Printf("Updated metadata for series: %s\n", title)
		}
	}
}

func searchTMDB(apiKey, mediaType, query string) (*TMDBResult, error) {
	safeQuery := url.QueryEscape(query)
	apiURL := fmt.Sprintf("https://api.themoviedb.org/3/search/%s?api_key=%s&query=%s", mediaType, apiKey, safeQuery)

	resp, err := http.Get(apiURL)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var data TMDBResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}

	if len(data.Results) > 0 {
		return &data.Results[0], nil
	}

	return nil, fmt.Errorf("no results found")
}

func extractYear(date string) int {
	if len(date) >= 4 {
		year := 0
		fmt.Sscanf(date[:4], "%d", &year)
		return year
	}
	return 0
}
