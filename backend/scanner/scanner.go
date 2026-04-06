package scanner

import (
	"database/sql"
	"dex/backend/db"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
)

// Regex patterns for TV shows: S01E01, 1x01, Season 1 Episode 1
var tvPatterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)s(\d+)e(\d+)`),
	regexp.MustCompile(`(?i)(\d+)x(\d+)`),
	regexp.MustCompile(`(?i)season\s+(\d+)\s+episode\s+(\d+)`),
}

func ScanDirectory(path, libType string) {
	// Set scanning status
	db.DB.Exec("UPDATE libraries SET scanning = 1, last_error = NULL WHERE path = ?", path)

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(filePath))
		if ext != ".mp4" && ext != ".mkv" && ext != ".avi" && ext != ".mov" {
			return nil
		}

		if libType == "movie" {
			processMovie(filePath)
		} else {
			processTVShow(filePath)
		}
		return nil
	})

	status := 0
	errorMsg := ""
	if err != nil {
		log.Printf("Error scanning directory %s: %v\n", path, err)
		errorMsg = err.Error()
	}

	db.DB.Exec("UPDATE libraries SET scanning = ?, last_scanned = ?, last_error = ? WHERE path = ?",
		status, time.Now(), errorMsg, path)
}

func processMovie(path string) {
	filename := filepath.Base(path)
	title := strings.TrimSuffix(filename, filepath.Ext(filename))
	format := strings.TrimPrefix(filepath.Ext(path), ".")

	_, err := db.DB.Exec(`INSERT OR IGNORE INTO media (title, filepath, format, type) VALUES (?, ?, ?, 'movie')`, title, path, format)
	if err != nil {
		log.Printf("Failed to insert movie %s: %v\n", title, err)
	}
}

func processTVShow(path string) {
	filename := filepath.Base(path)
	format := strings.TrimPrefix(filepath.Ext(path), ".")

	var season, episode int
	found := false

	for _, p := range tvPatterns {
		matches := p.FindStringSubmatch(filename)
		if len(matches) == 3 {
			season, _ = strconv.Atoi(matches[1])
			episode, _ = strconv.Atoi(matches[2])
			found = true
			break
		}
	}

	if !found {
		return // Skip files that don't match TV patterns in a TV library
	}

	// Try to find series title (part before S01E01)
	seriesTitle := filename
	for _, p := range tvPatterns {
		loc := p.FindStringIndex(filename)
		if loc != nil {
			seriesTitle = strings.TrimSpace(filename[:loc[0]])
			// Clean up separators like ., _, -
			seriesTitle = strings.Trim(seriesTitle, " ._-")
			break
		}
	}

	var seriesID int
	err := db.DB.QueryRow("SELECT id FROM series WHERE title = ?", seriesTitle).Scan(&seriesID)
	if err == sql.ErrNoRows {
		res, err := db.DB.Exec("INSERT INTO series (title) VALUES (?)", seriesTitle)
		if err != nil {
			log.Printf("Failed to insert series %s: %v\n", seriesTitle, err)
			return
		}
		id, _ := res.LastInsertId()
		seriesID = int(id)
	}

	title := fmt.Sprintf("%s - S%02dE%02d", seriesTitle, season, episode)
	_, err = db.DB.Exec(`
		INSERT OR IGNORE INTO media (title, filepath, format, type, series_id, season_num, episode_num) 
		VALUES (?, ?, ?, 'episode', ?, ?, ?)`, 
		title, path, format, seriesID, season, episode)
	
	if err != nil {
		log.Printf("Failed to insert episode %s: %v\n", title, err)
	}
}
