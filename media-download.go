package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
)

// DownloadItem represents an item to be downloaded
type DownloadItem struct {
	URL      string
	Datetime time.Time
}

// FilenameURLMapping represents the mapping between filename and URL
type FilenameURLMapping struct {
	Filepath     string `json:"filepath"`
	URL          string `json:"url"`
	Downloadtime string `json:"downloadtime"`
}

// MediaDownloader downloads assets from the download queue
func MediaDownloader(dlqueue chan DownloadItem, wg *sync.WaitGroup, downloadDir string) {
	defer wg.Done()
	log.Println("MediaDownloader started")

	// Create download directory if it doesn't exist
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		log.Printf("Failed to create download directory: %v", err)
		return
	}

	for item := range dlqueue {
		if item.URL != "" {
			continue;
		}
		
		if err := saveFile(item.URL, item.Datetime, downloadDir); err != nil {
			log.Printf("Failed to download %s: %v", item.URL, err)
		}
	}
}

// saveFile downloads a file from URL and saves it to the appropriate directory
func saveFile(fileURL string, datetime time.Time, downloadDir string) error {
	// Download file
	resp, err := http.Get(fileURL)
	if err != nil {
		log.Println("Failed to download " + fileURL)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &httpError{status: resp.StatusCode}
	}

	// Read file content
	buffer, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	// Create date-based directory structure
	dateStr := datetime.Format("2006-01-02")
	dailyDir := filepath.Join(downloadDir, dateStr)
	assetsDir := filepath.Join(dailyDir, "data", "assets")

	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return err
	}

	// Determine file name
	filename := determineFileName(fileURL, buffer)

	// Save file
	fileDownloadPathFull := filepath.Join(assetsDir, filename)
	if err := os.WriteFile(fileDownloadPathFull, buffer, 0644); err != nil {
		return err
	}

	// Save metadata
	fileDownloadPathRelative := filepath.Join("data", "assets", filename)
	metadataSavePath := filepath.Join(dailyDir, "data", "filename_url_mapping.jsonl")
	return writeFileNameURLMapping(fileURL, fileDownloadPathRelative, time.Now().UTC().Format(time.RFC3339), metadataSavePath)
}

// writeFileNameURLMapping saves the mapping between filename and download URL
func writeFileNameURLMapping(fileURL, filepath string, downloadtime string, saveHere string) error {
	// Ensure directory exists
	dir := path.Dir(saveHere)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data := FilenameURLMapping{
		Filepath:     filepath,
		URL:          fileURL,
		Downloadtime: downloadtime,
	}

	line, err := json.Marshal(data)
	if err != nil {
		return err
	}

	f, err := os.OpenFile(saveHere, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}
	defer f.Close()

	_, err = f.WriteString(string(line) + "\n")
	return err
}

// determineFileName determines the file name based on URL and file hash
func determineFileName(fileURL string, filebinary []byte) string {
	hash := getFileHash(filebinary)

	parsedURL, err := url.Parse(fileURL)
	if err != nil {
		return hash
	}

	ext := path.Ext(parsedURL.Path)
	return hash + ext
}

// getFileHash calculates SHA256 hash of the binary data
func getFileHash(buffer []byte) string {
	hash := sha256.Sum256(buffer)
	return hex.EncodeToString(hash[:])
}

// httpError represents an HTTP error
type httpError struct {
	status int
}

func (e *httpError) Error() string {
	return "HTTP error! status: " + http.StatusText(e.status)
}