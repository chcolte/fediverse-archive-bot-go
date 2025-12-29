package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sync"
	"time"
	"strings"

	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"github.com/chcolte/fediverse-archive-bot-go/models"
)

// MediaDownloader downloads assets from the download queue
func MediaDownloader(dlqueue chan models.DownloadItem, wg *sync.WaitGroup, downloadDir string) {
	defer wg.Done()
	logger.Info("MediaDownloader started")

	// Create download directory if it doesn't exist
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		logger.Errorf("Failed to create download directory: %v", err)
		return
	}

	for item := range dlqueue {
		if item.URL == "" {
			continue
		}

		if err := saveFile(item.URL, item.Datetime, downloadDir); err != nil {
			logger.Errorf("Failed to download %s: %v", item.URL, err)
			// dlqueue <- item //リキューするなら，回数上限を設ける仕組みがないとスタック
		}
	}
}

// saveFile downloads a file from URL and saves it to the appropriate directory
func saveFile(fileURL string, datetime time.Time, downloadDir string) error {
	// Download file
	resp, err := http.Get(fileURL)
	if err != nil {
		logger.Debugf("Failed to http Get: %s", fileURL)
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return &httpError{status: resp.StatusCode}
	}

	// Get Content-Type header
	contentType := resp.Header.Get("Content-Type")

	// Read file content
	buffer, err := io.ReadAll(resp.Body)
	if err != nil {
		logger.Debugf("Failed to io.ReadAll: %s", fileURL)
		return err
	}

	// Create date-based directory structure
	dateStr := datetime.Format("2006-01-02")
	dailyDir := filepath.Join(downloadDir, dateStr)
	assetsDir := filepath.Join(dailyDir, "assets")

	if err := os.MkdirAll(assetsDir, 0755); err != nil {
		return err
	}

	// Determine file name
	filename := determineFileName(fileURL, buffer, contentType)

	// Save file
	fileDownloadPathFull := filepath.Join(assetsDir, filename)
	if err := os.WriteFile(fileDownloadPathFull, buffer, 0644); err != nil {
		return err
	}

	// Save metadata (今は決め打ち)
	fileDownloadPathRelative := filepath.Join(assetsDir, filename)
	metadataSavePath := filepath.Join(dailyDir, "filename_url_mapping.jsonl")
	return writeFileNameURLMapping(fileURL, fileDownloadPathRelative, time.Now().UTC().Format(time.RFC3339), metadataSavePath)
}

// writeFileNameURLMapping saves the mapping between filename and download URL
func writeFileNameURLMapping(fileURL, filepath string, downloadtime string, saveHere string) error {
	// Ensure directory exists
	dir := path.Dir(saveHere)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data := models.FilenameURLMapping{
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

// determineFileName determines the file name based on Content-Type and URL
func determineFileName(fileURL string, filebinary []byte, contentType string) string {
	hash := getFileHash(filebinary)

	// まずContent-Typeから拡張子を決定
	ext := extFromContentType(contentType)
	
	// Content-Typeから取れない場合はURLから推測
	if ext == "" {
		parsedURL, err := url.Parse(fileURL)
		if err == nil {
			// URLのクエリパラメータにつく"?"が%3fとエンコードされてしまっているURLを渡された際に，
			// 拡張子が，適切でなくなることを事前に弾く
			urlPath := parsedURL.Path
			if idx := strings.Index(urlPath, "?"); idx != -1 {
				urlPath = urlPath[:idx]
			}
			ext = path.Ext(urlPath)
		}
	}
	
	return hash + ext
}

// extFromContentType returns file extension from Content-Type header
func extFromContentType(contentType string) string {
	// Content-Type: image/jpeg; charset=utf-8 のような形式を処理
	if idx := strings.Index(contentType, ";"); idx != -1 {
		contentType = contentType[:idx]
	}
	contentType = strings.TrimSpace(contentType)

	switch contentType {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/gif":
		return ".gif"
	case "image/webp":
		return ".webp"
	case "image/avif":
		return ".avif"
	case "image/svg+xml":
		return ".svg"
	case "video/mp4":
		return ".mp4"
	case "video/webm":
		return ".webm"
	case "video/quicktime":
		return ".mov"
	case "audio/mpeg":
		return ".mp3"
	case "audio/ogg":
		return ".ogg"
	case "audio/wav":
		return ".wav"
	case "application/pdf":
		return ".pdf"
	default:
		return ""
	}
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