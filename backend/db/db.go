package db

import (
	"database/sql"
	"log"

	_ "github.com/mattn/go-sqlite3"
)

var DB *sql.DB

func InitDB(filepath string) {
	var err error
	DB, err = sql.Open("sqlite3", filepath)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	createTables()
}

func createTables() {
	userTable := `
	CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password TEXT NOT NULL,
		role TEXT DEFAULT 'user'
	);`

	seriesTable := `
	CREATE TABLE IF NOT EXISTS series (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL UNIQUE,
		plot TEXT,
		poster_url TEXT,
		year INTEGER,
		tmdb_id TEXT,
		rating REAL,
		genres TEXT
	);`

	mediaTable := `
	CREATE TABLE IF NOT EXISTS media (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		title TEXT NOT NULL,
		filepath TEXT UNIQUE NOT NULL,
		format TEXT NOT NULL,
		poster_url TEXT,
		plot TEXT,
		year INTEGER,
		duration INTEGER,
		current_position INTEGER DEFAULT 0,
		type TEXT DEFAULT 'movie', -- 'movie' or 'episode'
		series_id INTEGER,
		season_num INTEGER,
		episode_num INTEGER,
		tmdb_id TEXT,
		rating REAL,
		genres TEXT,
		last_watched DATETIME,
		FOREIGN KEY (series_id) REFERENCES series(id)
	);`

	librariesTable := `
	CREATE TABLE IF NOT EXISTS libraries (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		path TEXT UNIQUE NOT NULL,
		type TEXT NOT NULL, -- 'movie' or 'tv'
		last_scanned DATETIME,
		scanning BOOLEAN DEFAULT 0,
		last_error TEXT
	);`

	_, err := DB.Exec(userTable)
	if err != nil {
		log.Fatal("Failed to create users table:", err)
	}

	_, err = DB.Exec(seriesTable)
	if err != nil {
		log.Fatal("Failed to create series table:", err)
	}

	_, err = DB.Exec(mediaTable)
	if err != nil {
		log.Fatal("Failed to create media table:", err)
	}

	// Migrations
	_, _ = DB.Exec("ALTER TABLE media ADD COLUMN current_position INTEGER DEFAULT 0")
	_, _ = DB.Exec("ALTER TABLE media ADD COLUMN genres TEXT")
	_, _ = DB.Exec("ALTER TABLE series ADD COLUMN genres TEXT")
	_, _ = DB.Exec("ALTER TABLE libraries ADD COLUMN scanning BOOLEAN DEFAULT 0")
	_, _ = DB.Exec("ALTER TABLE libraries ADD COLUMN last_error TEXT")

	_, err = DB.Exec(librariesTable)
	if err != nil {
		log.Fatal("Failed to create libraries table:", err)
	}
}
