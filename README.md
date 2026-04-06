# DEX Media Server (V0.9.0 Alpha)

DEX is a modern, lightweight, and self-hosted media server designed for seamless streaming of movies and TV shows directly in your web browser.

## 🚀 Features

- **Automated Scanning:** Automatically detects movies and TV shows from your library folders.
- **Rich Metadata:** Fetches posters, plot summaries, ratings, and genres from TMDB.
- **On-Demand Transcoding:** Uses FFmpeg to transcode media into HLS for maximum web compatibility.
- **Playback Persistence:** Tracks your progress and provides a "Continue Watching" section.
- **Global Search:** Find any movie, series, or episode instantly.
- **Subtitle Support:** Automatically detects and serves local .vtt and .srt subtitles.
- **File Management:** Built-in tool to rename files to ideal formats based on metadata.
- **Multi-User Ready:** JWT-based authentication system.

## 🛠️ Quick Start

### 1. Prerequisites
- Docker and Docker Compose installed.
- A TMDB API Key (Get one for free at [themoviedb.org](https://www.themoviedb.org/)).

### 2. Configuration
Create a `.env` file in the root directory:
```env
TMDB_API_KEY=your_api_key_here
PORT=8080
MEDIA_DIR=./media
DB_PATH=./data/dex.db
```

### 3. Launch
```bash
docker-compose up -d
```
The server will be available at `http://localhost:3000`.

## 📁 Library Organization

For the best experience, organize your media as follows:

- **Movies:** `/media/Movies/Movie Title (Year).mp4`
- **TV Shows:** `/media/TV/Series Name/Season 01/S01E01.mkv`

Subtitles should be named identically to the video file (e.g., `S01E01.en.srt`).

## 🛡️ Security Note
DEX Alpha currently uses a default secret for JWT signing. For production-like environments, ensure your `.env` is protected and consider running behind a reverse proxy like Nginx or Caddy with SSL.

## ⚖️ License
MIT
