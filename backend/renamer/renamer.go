package renamer

import (
	"dex/backend/db"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

type RenamePreview struct {
	ID        int    `json:"id"`
	OldPath   string `json:"old_path"`
	NewPath   string `json:"new_path"`
	NewName   string `json:"new_name"`
	Type      string `json:"type"`
	NeedsRename bool `json:"needs_rename"`
}

func sanitizeFilename(name string) string {
	re := regexp.MustCompile(`[\\/:*?"<>|]`)
	return re.ReplaceAllString(name, " ")
}

func GenerateIdealName(m map[string]interface{}) string {
	mediaType := m["type"].(string)
	title := m["title"].(string)
	ext := m["format"].(string)
	year := m["year"].(int64)

	if mediaType == "movie" {
		if year > 0 {
			return fmt.Sprintf("%s (%d)%s", sanitizeFilename(title), year, ext)
		}
		return sanitizeFilename(title) + ext
	} else if mediaType == "episode" {
		seriesName := m["series_title"].(string)
		season := m["season_num"].(int64)
		episode := m["episode_num"].(int64)
		// Series Name - S01E01.ext
		return fmt.Sprintf("%s - S%02dE%02d%s", sanitizeFilename(seriesName), season, episode, ext)
	}

	return title + ext
}

func GetRenamePreviews() ([]RenamePreview, error) {
	query := `
		SELECT m.id, m.title, m.filepath, m.format, m.type, m.year, m.season_num, m.episode_num, COALESCE(s.title, '') as series_title
		FROM media m
		LEFT JOIN series s ON m.series_id = s.id
		WHERE m.tmdb_id IS NOT NULL OR (m.type = 'episode' AND s.title IS NOT NULL)
	`
	rows, err := db.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var previews []RenamePreview
	for rows.Next() {
		var id int
		var title, currentPath, format, mediaType, seriesTitle string
		var year, season, episode int64

		if err := rows.Scan(&id, &title, &currentPath, &format, &mediaType, &year, &season, &episode, &seriesTitle); err != nil {
			continue
		}

		mediaMap := map[string]interface{}{
			"id":           id,
			"title":        title,
			"format":       format,
			"type":         mediaType,
			"year":         year,
			"season_num":   season,
			"episode_num":  episode,
			"series_title": seriesTitle,
		}

		idealName := GenerateIdealName(mediaMap)
		dir := filepath.Dir(currentPath)
		newPath := filepath.Join(dir, idealName)

		// Compare actual filename with ideal name
		currentName := filepath.Base(currentPath)
		needsRename := currentName != idealName

		previews = append(previews, RenamePreview{
			ID:          id,
			OldPath:     currentPath,
			NewPath:     newPath,
			NewName:     idealName,
			Type:        mediaType,
			NeedsRename: needsRename,
		})
	}

	return previews, nil
}

func ApplyRename(id int, newPath string) error {
	var oldPath string
	err := db.DB.QueryRow("SELECT filepath FROM media WHERE id = ?", id).Scan(&oldPath)
	if err != nil {
		return err
	}

	if oldPath == newPath {
		return nil
	}

	// Ensure destination doesn't exist
	if _, err := os.Stat(newPath); err == nil {
		return fmt.Errorf("file already exists: %s", newPath)
	}

	// Perform actual rename on disk
	err = os.Rename(oldPath, newPath)
	if err != nil {
		return err
	}

	// Update DB
	newTitle := strings.TrimSuffix(filepath.Base(newPath), filepath.Ext(newPath))
	_, err = db.DB.Exec("UPDATE media SET filepath = ?, title = ? WHERE id = ?", newPath, newTitle, id)
	if err != nil {
		// ROLLBACK RENAME if DB update fails?
		os.Rename(newPath, oldPath)
		return err
	}

	log.Printf("Renamed file: %s -> %s\n", oldPath, newPath)
	return nil
}
