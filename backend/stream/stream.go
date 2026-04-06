package stream

import (
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func GetStreamDir(id string) string {
	return filepath.Join("data", "streams", id)
}

func GetPlaylistPath(id string) string {
	return filepath.Join(GetStreamDir(id), "index.m3u8")
}

func StartCleanupTask() {
	go func() {
		for {
			time.Sleep(1 * time.Hour)
			cleanupOldStreams()
		}
	}()
}

func cleanupOldStreams() {
	streamRoot := "data/streams"
	entries, err := os.ReadDir(streamRoot)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() {
			dirPath := filepath.Join(streamRoot, entry.Name())
			info, err := os.Stat(dirPath)
			if err == nil {
				// Delete streams older than 24 hours
				if time.Since(info.ModTime()) > 24*time.Hour {
					os.RemoveAll(dirPath)
					log.Printf("Cleaned up old stream: %s\n", dirPath)
				}
			}
		}
	}
}

func TranscodeIfNeeded(mediaPath string, id string) error {
	outDir := GetStreamDir(id)
	playlistPath := GetPlaylistPath(id)

	if _, err := os.Stat(playlistPath); err == nil {
		return nil // Already transcoded
	}

	err := os.MkdirAll(outDir, 0755)
	if err != nil {
		return err
	}

	// FFmpeg command for HLS
	cmd := exec.Command("ffmpeg",
		"-i", mediaPath,
		"-codec:v", "libx264",
		"-preset", "veryfast",
		"-codec:a", "aac",
		"-b:a", "128k",
		"-map", "0",
		"-f", "hls",
		"-hls_time", "10",
		"-hls_list_size", "0",
		"-hls_segment_filename", filepath.Join(outDir, "seg%d.ts"),
		playlistPath,
	)

	// Run in background
	go func() {
		log.Printf("Starting transcoding for media %s to %s\n", id, outDir)
		output, err := cmd.CombinedOutput()
		if err != nil {
			log.Printf("FFmpeg error for %s: %v\nOutput: %s\n", id, err, string(output))
		} else {
			log.Printf("Finished transcoding for %s\n", id)
		}
	}()

	return nil
}

func ServeHLS(w http.ResponseWriter, r *http.Request, id string) {
	streamDir := GetStreamDir(id)
	
	playlistPath := GetPlaylistPath(id)
	if _, err := os.Stat(playlistPath); os.IsNotExist(err) {
		http.Error(w, "Transcoding in progress, please wait", http.StatusAccepted)
		return
	}

	fs := http.FileServer(http.Dir(streamDir))
	fs.ServeHTTP(w, r)
}
