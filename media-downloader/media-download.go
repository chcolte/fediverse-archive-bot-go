package mediaDownloader

import (
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"github.com/chcolte/fediverse-archive-bot-go/models"
	"github.com/chcolte/fediverse-archive-bot-go/utils"
	"github.com/patrickmn/go-cache"
)

// TTL: 24 hours, cleanup interval: 30 minutes.
var urlCache = cache.New(24*time.Hour, 30*time.Minute)

// MediaDownloader downloads assets from the download queue
func MediaDownloader(dlqueue chan models.DownloadItem, wg *sync.WaitGroup, downloadDir string, media_fetch_only bool) {
	defer wg.Done()
	logger.Info("MediaDownloader started")

	// Create download directory if it doesn't exist
	if err := os.MkdirAll(downloadDir, 0755); err != nil {
		logger.Errorf("Failed to create download directory: %v", err)
		return
	}

	for item := range dlqueue {
		if len(dlqueue) > 90 { // MagicNumber: dlqueueのバッファが100なので90を指定。
			logger.Errorf("Reaching media download queue limit! Currently %d . Stopped all archive system.", len(dlqueue))
		}

		if item.URL == "" || isRecentlyFetched(item.URL) {
			logger.Debug("Skiped fetch Media: "+item.URL)
			continue
		}

		resp, err := fetchFile(item.URL)
		if err != nil {
			logger.Errorf("Failed to fetch %s : %v", item.URL, err)
			continue
		}
		
		if media_fetch_only {
			io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			
		}else{
			err := saveFile(resp, item.URL, item.Datetime, downloadDir)
			resp.Body.Close()
			if err != nil {
				logger.Errorf("Failed to download %s: %v", item.URL, err)
				// dlqueue <- item //TODO: リキューするなら，回数上限を設ける仕組みがないとスタック
			}
			
			logger.Debug("Downloaded Media: "+item.URL)
		}
		markAsFetched(item.URL)
	}
}

func isRecentlyFetched(fileURL string) bool {
	// Skip if URL was recently fetched
	if _, found := urlCache.Get(fileURL); found {
		logger.Debugf("Skipping recently downloaded URL: %s", fileURL)
		return true
	}
	return false
}

func markAsFetched(fileURL string) {
	// Mark URL as downloaded in cache
	urlCache.SetDefault(fileURL, true)
}

func fetchFile(fileURL string) (*http.Response, error) {
	// Skip TLS Verify (for Pywb proxy)
	tr := http.DefaultTransport.(*http.Transport).Clone()
	tr.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	client := &http.Client{Transport: tr}

	// Fetch file
	resp, err := client.Get(fileURL)
	if err != nil {
		logger.Debugf("Failed to http Get: %s", fileURL)
		return nil, err
	}
	
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, &httpError{status: resp.StatusCode}
	}
	return resp, nil
}

// saveFile downloads a file from URL and saves it to the appropriate directory
func saveFile(resp *http.Response, fileURL string, datetime time.Time, downloadDir string) error {

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
	data := map[string]interface{}{
		"filepath":     filepath,
		"url":          fileURL,
		"downloadtime": downloadtime,
	}
	return utils.SaveRecord(utils.RecordTypeMediaMapping, data, saveHere)
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