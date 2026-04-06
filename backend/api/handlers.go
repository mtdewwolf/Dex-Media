package api

import (
	"database/sql"
	"dex/backend/auth"
	"dex/backend/db"
	"dex/backend/renamer"
	"dex/backend/scanner"
	"dex/backend/stream"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

type Credentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func SetupRoutes(r chi.Router) {
	r.Post("/api/login", Login)
	r.Post("/api/register", Register)

	r.Group(func(r chi.Router) {
		r.Use(AuthMiddleware)
		r.Get("/api/media", GetMedia)
		r.Get("/api/series", GetSeries)
		r.Get("/api/series/{id}/episodes", GetEpisodesBySeries)
		r.Get("/api/search", SearchMedia)
		
		// Playback progress
		r.Post("/api/media/{id}/progress", UpdateProgress)
		r.Get("/api/continue-watching", GetContinueWatching)

		// Streaming endpoints
		r.Get("/api/stream/{id}/start", StartHLS)
		r.Get("/api/stream/{id}/*", ServeHLSFile)
		r.Get("/api/media/{id}/subtitles", GetSubtitles)
		r.Get("/api/media/{id}/subtitles/{filename}", ServeSubtitleFile)
		
		// Renamer endpoints
		r.Get("/api/renamer/preview", GetRenamePreviews)
		r.Post("/api/renamer/apply", ApplyRenames)

		// Library endpoints
		r.Get("/api/libraries", GetLibraries)
		r.Post("/api/libraries", AddLibrary)
		r.Delete("/api/libraries/{id}", RemoveLibrary)
		r.Post("/api/libraries/{id}/scan", ScanLibrary)
		
		// Legacy direct play (keep as fallback)
		r.Get("/stream/{id}", StreamMedia)
	})
}

func Login(w http.ResponseWriter, r *http.Request) {
	var creds Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	var storedHash string
	var role string
	err := db.DB.QueryRow("SELECT password, role FROM users WHERE username = ?", creds.Username).Scan(&storedHash, &role)
	if err != nil || !auth.CheckPasswordHash(creds.Password, storedHash) {
		http.Error(w, "Invalid credentials", http.StatusUnauthorized)
		return
	}

	token, err := auth.GenerateToken(creds.Username, role)
	if err != nil {
		http.Error(w, "Error generating token", http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"token": token})
}

func Register(w http.ResponseWriter, r *http.Request) {
	var creds Credentials
	if err := json.NewDecoder(r.Body).Decode(&creds); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	hash, err := auth.HashPassword(creds.Password)
	if err != nil {
		http.Error(w, "Internal error", http.StatusInternalServerError)
		return
	}

	_, err = db.DB.Exec("INSERT INTO users (username, password) VALUES (?, ?)", creds.Username, hash)
	if err != nil {
		http.Error(w, "Username already taken", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func AuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenString := r.Header.Get("Authorization")
		if len(tokenString) > 7 && tokenString[:7] == "Bearer " {
			tokenString = tokenString[7:]
		}

		claims, err := auth.ValidateToken(tokenString)
		if err != nil {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		_ = claims
		next.ServeHTTP(w, r)
	})
}

func GetMedia(w http.ResponseWriter, r *http.Request) {
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "id DESC"
	} else if sortBy == "newest" {
		sortBy = "id DESC"
	} else if sortBy == "rating" {
		sortBy = "rating DESC"
	} else if sortBy == "year" {
		sortBy = "year DESC"
	}

	query := fmt.Sprintf(`
		SELECT id, title, format, poster_url, plot, year, rating, type, series_id, season_num, episode_num, duration, current_position, genres 
		FROM media WHERE type = 'movie' ORDER BY %s`, sortBy)

	rows, err := db.DB.Query(query)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var title, format, mediaType string
		var posterURL, plot, genres sql.NullString
		var year, season, episode, seriesID, duration, currentPosition sql.NullInt64
		var rating sql.NullFloat64

		if err := rows.Scan(&id, &title, &format, &posterURL, &plot, &year, &rating, &mediaType, &seriesID, &season, &episode, &duration, &currentPosition, &genres); err != nil {
			continue
		}

		media := map[string]interface{}{
			"id":               id,
			"title":            title,
			"format":           format,
			"poster_url":       posterURL.String,
			"plot":             plot.String,
			"year":             year.Int64,
			"rating":           rating.Float64,
			"type":             mediaType,
			"duration":         duration.Int64,
			"current_position": currentPosition.Int64,
			"genres":           genres.String,
		}
		results = append(results, media)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func GetSeries(w http.ResponseWriter, r *http.Request) {
	sortBy := r.URL.Query().Get("sort")
	if sortBy == "" {
		sortBy = "id DESC"
	} else if sortBy == "newest" {
		sortBy = "id DESC"
	} else if sortBy == "rating" {
		sortBy = "rating DESC"
	} else if sortBy == "year" {
		sortBy = "year DESC"
	}

	query := fmt.Sprintf("SELECT id, title, plot, poster_url, year, rating, genres FROM series ORDER BY %s", sortBy)
	rows, err := db.DB.Query(query)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var title string
		var plot, posterURL, genres sql.NullString
		var year sql.NullInt64
		var rating sql.NullFloat64

		if err := rows.Scan(&id, &title, &plot, &posterURL, &year, &rating, &genres); err != nil {
			continue
		}

		results = append(results, map[string]interface{}{
			"id":         id,
			"title":      title,
			"plot":       plot.String,
			"poster_url": posterURL.String,
			"year":       year.Int64,
			"rating":     rating.Float64,
			"genres":     genres.String,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func GetEpisodesBySeries(w http.ResponseWriter, r *http.Request) {
	seriesID := chi.URLParam(r, "id")
	rows, err := db.DB.Query(`
		SELECT id, title, format, poster_url, plot, year, rating, season_num, episode_num, duration, current_position 
		FROM media WHERE series_id = ? ORDER BY season_num, episode_num`, seriesID)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var title, format string
		var posterURL, plot sql.NullString
		var year, season, episode, duration, currentPosition sql.NullInt64
		var rating sql.NullFloat64

		if err := rows.Scan(&id, &title, &format, &posterURL, &plot, &year, &rating, &season, &episode, &duration, &currentPosition); err != nil {
			continue
		}

		results = append(results, map[string]interface{}{
			"id":               id,
			"title":            title,
			"format":           format,
			"poster_url":       posterURL.String,
			"plot":             plot.String,
			"year":             year.Int64,
			"rating":           rating.Float64,
			"season_num":       season.Int64,
			"episode_num":      episode.Int64,
			"duration":         duration.Int64,
			"current_position": currentPosition.Int64,
		})
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func StartHLS(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var filepath string
	err := db.DB.QueryRow("SELECT filepath FROM media WHERE id = ?", id).Scan(&filepath)
	if err != nil {
		http.Error(w, "Media not found", http.StatusNotFound)
		return
	}

	err = stream.TranscodeIfNeeded(filepath, id)
	if err != nil {
		http.Error(w, "Failed to start transcoder", http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(map[string]string{"playlist": fmt.Sprintf("/api/stream/%s/index.m3u8", id)})
}

func ServeHLSFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	subPath := chi.URLParam(r, "*")
	
	streamDir := stream.GetStreamDir(id)
	fullPath := filepath.Join(streamDir, subPath)

	// Security: prevent path traversal
	if !strings.HasPrefix(fullPath, "data/streams") {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	// Wait for index.m3u8 if it's the playlist
	if subPath == "index.m3u8" {
		// Wait a few seconds for FFmpeg to create the file
		for i := 0; i < 10; i++ {
			if _, err := os.Stat(fullPath); err == nil {
				break
			}
			http.Error(w, "Still transcoding...", http.StatusAccepted)
			return
		}
	}

	http.ServeFile(w, r, fullPath)
}

func GetRenamePreviews(w http.ResponseWriter, r *http.Request) {
	previews, err := renamer.GetRenamePreviews()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(previews)
}

func ApplyRenames(w http.ResponseWriter, r *http.Request) {
	var request []struct {
		ID      int    `json:"id"`
		NewPath string `json:"new_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	for _, req := range request {
		// Validation: check if source exists and is within expected paths
		var oldPath string
		err := db.DB.QueryRow("SELECT filepath FROM media WHERE id = ?", req.ID).Scan(&oldPath)
		if err != nil {
			continue
		}

		if _, err := os.Stat(oldPath); os.IsNotExist(err) {
			log.Printf("Skip rename: source file not found: %s\n", oldPath)
			continue
		}

		// Validation: ensure target directory exists
		targetDir := filepath.Dir(req.NewPath)
		if err := os.MkdirAll(targetDir, 0755); err != nil {
			log.Printf("Failed to create target directory %s: %v\n", targetDir, err)
			continue
		}

		if err := renamer.ApplyRename(req.ID, req.NewPath); err != nil {
			log.Printf("Failed to rename %d: %v\n", req.ID, err)
		}
	}

	w.WriteHeader(http.StatusOK)
}

func GetLibraries(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query("SELECT id, path, type, last_scanned, scanning, last_error FROM libraries")
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var path, libType string
		var lastScanned sql.NullTime
		var scanning bool
		var lastError sql.NullString
		if err := rows.Scan(&id, &path, &libType, &lastScanned, &scanning, &lastError); err != nil {
			continue
		}
		results = append(results, map[string]interface{}{
			"id":           id,
			"path":         path,
			"type":         libType,
			"last_scanned": lastScanned.Time,
			"scanning":     scanning,
			"last_error":   lastError.String,
		})
	}
	json.NewEncoder(w).Encode(results)
}

func AddLibrary(w http.ResponseWriter, r *http.Request) {
	var lib struct {
		Path string `json:"path"`
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&lib); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	_, err := db.DB.Exec("INSERT INTO libraries (path, type) VALUES (?, ?)", lib.Path, lib.Type)
	if err != nil {
		http.Error(w, "Library already exists or invalid path", http.StatusConflict)
		return
	}

	w.WriteHeader(http.StatusCreated)
}

func RemoveLibrary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	_, err := db.DB.Exec("DELETE FROM libraries WHERE id = ?", id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func ScanLibrary(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var path, libType string
	err := db.DB.QueryRow("SELECT path, type FROM libraries WHERE id = ?", id).Scan(&path, &libType)
	if err != nil {
		http.Error(w, "Library not found", http.StatusNotFound)
		return
	}

	go scanner.ScanDirectory(path, libType)
	w.WriteHeader(http.StatusAccepted)
}

func GetSubtitles(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var mediaPath string
	err := db.DB.QueryRow("SELECT filepath FROM media WHERE id = ?", id).Scan(&mediaPath)
	if err != nil {
		http.Error(w, "Media not found", http.StatusNotFound)
		return
	}

	dir := filepath.Dir(mediaPath)
	base := strings.TrimSuffix(filepath.Base(mediaPath), filepath.Ext(mediaPath))
	
	files, _ := os.ReadDir(dir)
	var subs []string
	for _, f := range files {
		if !f.IsDir() && strings.HasPrefix(f.Name(), base) {
			ext := strings.ToLower(filepath.Ext(f.Name()))
			if ext == ".vtt" || ext == ".srt" {
				subs = append(subs, f.Name())
			}
		}
	}

	json.NewEncoder(w).Encode(subs)
}

func ServeSubtitleFile(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	filename := chi.URLParam(r, "filename")
	
	var mediaPath string
	err := db.DB.QueryRow("SELECT filepath FROM media WHERE id = ?", id).Scan(&mediaPath)
	if err != nil {
		http.Error(w, "Media not found", http.StatusNotFound)
		return
	}

	dir := filepath.Dir(mediaPath)
	fullPath := filepath.Join(dir, filename)

	if filepath.Dir(fullPath) != dir {
		http.Error(w, "Access denied", http.StatusForbidden)
		return
	}

	http.ServeFile(w, r, fullPath)
}

func StreamMedia(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var filepath string
	err := db.DB.QueryRow("SELECT filepath FROM media WHERE id = ?", id).Scan(&filepath)
	if err != nil {
		http.Error(w, "Media not found", http.StatusNotFound)
		return
	}
	http.ServeFile(w, r, filepath)
}

func UpdateProgress(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var body struct {
		CurrentPosition int `json:"current_position"`
		Duration        int `json:"duration"`
	}
	if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	_, err := db.DB.Exec("UPDATE media SET current_position = ?, duration = ?, last_watched = ? WHERE id = ?",
		body.CurrentPosition, body.Duration, time.Now(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func GetContinueWatching(w http.ResponseWriter, r *http.Request) {
	rows, err := db.DB.Query(`
		SELECT id, title, format, poster_url, plot, year, rating, type, series_id, season_num, episode_num, duration, current_position 
		FROM media 
		WHERE last_watched IS NOT NULL 
		AND (current_position < duration * 0.95 OR duration = 0)
		ORDER BY last_watched DESC 
		LIMIT 10`)
	if err != nil {
		http.Error(w, "Database error", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var results []map[string]interface{}
	for rows.Next() {
		var id int
		var title, format, mediaType string
		var posterURL, plot sql.NullString
		var year, season, episode, seriesID, duration, currentPosition sql.NullInt64
		var rating sql.NullFloat64

		if err := rows.Scan(&id, &title, &format, &posterURL, &plot, &year, &rating, &mediaType, &seriesID, &season, &episode, &duration, &currentPosition); err != nil {
			continue
		}

		results = append(results, map[string]interface{}{
			"id":               id,
			"title":            title,
			"format":           format,
			"poster_url":       posterURL.String,
			"plot":             plot.String,
			"year":             year.Int64,
			"rating":           rating.Float64,
			"type":             mediaType,
			"duration":         duration.Int64,
			"current_position": currentPosition.Int64,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

func SearchMedia(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query().Get("q")
	if query == "" {
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	searchPattern := "%" + query + "%"
	var results []map[string]interface{}

	// Search Movies
	movieRows, err := db.DB.Query(`
		SELECT id, title, format, poster_url, plot, year, rating, type, genres, duration, current_position 
		FROM media WHERE type = 'movie' AND (title LIKE ? OR plot LIKE ? OR genres LIKE ?)`, searchPattern, searchPattern, searchPattern)
	if err == nil {
		defer movieRows.Close()
		for movieRows.Next() {
			var id int
			var title, format, mediaType string
			var posterURL, plot, genres sql.NullString
			var year, duration, currentPosition sql.NullInt64
			var rating sql.NullFloat64

			if err := movieRows.Scan(&id, &title, &format, &posterURL, &plot, &year, &rating, &mediaType, &genres, &duration, &currentPosition); err != nil {
				continue
			}
			results = append(results, map[string]interface{}{
				"id":               id,
				"title":            title,
				"format":           format,
				"poster_url":       posterURL.String,
				"plot":             plot.String,
				"year":             year.Int64,
				"rating":           rating.Float64,
				"type":             "movie",
				"genres":           genres.String,
				"duration":         duration.Int64,
				"current_position": currentPosition.Int64,
			})
		}
	}

	// Search Series
	seriesRows, err := db.DB.Query(`
		SELECT id, title, plot, poster_url, year, rating, genres 
		FROM series WHERE title LIKE ? OR plot LIKE ? OR genres LIKE ?`, searchPattern, searchPattern, searchPattern)
	if err == nil {
		defer seriesRows.Close()
		for seriesRows.Next() {
			var id int
			var title string
			var plot, posterURL, genres sql.NullString
			var year sql.NullInt64
			var rating sql.NullFloat64

			if err := seriesRows.Scan(&id, &title, &plot, &posterURL, &year, &rating, &genres); err != nil {
				continue
			}
			results = append(results, map[string]interface{}{
				"id":         id,
				"title":      title,
				"plot":       plot.String,
				"poster_url": posterURL.String,
				"year":       year.Int64,
				"rating":     rating.Float64,
				"type":       "series",
				"genres":     genres.String,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}
