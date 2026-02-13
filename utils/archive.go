package utils

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"github.com/chcolte/fediverse-archive-bot-go/models"
	"github.com/google/uuid"
)

const ToolVersion = "0.3.1-beta"

// プロセス起動時に1回生成されるセッションID
var ServerSessionID = uuid.New().String()

// レコードタイプ定数
const (
	RecordTypeArchiveInfo = "archive_info"
	RecordTypeResponse = "response"
	RecordTypeRequest  = "request"
	RecordTypeMetadata = "metadata"
)

// saveRecord: JSONL形式で保存するベース関数。
// エンベロープ（必須フィールド）のみを管理し、dataの中身は呼び出し側が構築する。
//
// 必須フィールド（自動付与）:
//   - record_id:       レコードごとのUUID
//   - server_session:  プロセスセッションID
//   - saved_at:        保存時刻（RFC3339）
//   - record_type:     レコード種別
func saveRecord(recordType string, data interface{}, savePath string) error {
	envelope := struct {
		RecordID      string      `json:"record_id"`
		ServerSession string      `json:"server_session"`
		SavedAt       string      `json:"saved_at"`
		RecordType    string      `json:"record_type"`
		Data          interface{} `json:"data"`
	}{
		RecordID:      uuid.New().String(),
		ServerSession: ServerSessionID,
		SavedAt:       time.Now().Format(time.RFC3339),
		RecordType:    recordType,
		Data:          data,
	}

	line, err := json.Marshal(envelope)
	if err != nil {
		return fmt.Errorf("failed to marshal record: %w", err)
	}

	// ディレクトリを自動作成
	dir := filepath.Dir(savePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	// JSONL形式でappend
	f, err := os.OpenFile(savePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("failed to open file: %w", err)
	}
	defer f.Close()

	if _, err := f.WriteString(string(line) + "\n"); err != nil {
		return fmt.Errorf("failed to write: %w", err)
	}

	logger.Debugf("Saved %s record to %s", recordType, savePath)
	return nil
}

// SaveArchiveInfo はサーバー起動時のメタデータを保存する。
func SaveArchiveInfo(savePath string, mode string, timelines []string, scope string, seedServers []models.Server) error {
	urls := make([]string, len(seedServers))
	for i, s := range seedServers {
		urls[i] = s.URL
	}
	data := struct {
		Software    string `json:"software"`
		// hostname //コメントはWarcに存在するフィールド
		// ip
		ServerSessionID string `json:"server_session_id"`
		// description
		// operator
		// http-header-user-agent


		Mode        string `json:"mode"`
		Timelines   string `json:"timelines"`
		Scope       string `json:"scope"`
		SeedServers string `json:"seed_servers"`
	}{
		Software:    "fediverse-archive-bot-go/" + ToolVersion,
		ServerSessionID: ServerSessionID,
		Mode:        mode,
		Timelines:   strings.Join(timelines, ","),
		Scope:       scope,
		SeedServers: strings.Join(urls, ","),
	}
	return saveRecord(RecordTypeArchiveInfo, data, savePath)
}

// computeDigest は生データのSHA256ダイジェストを計算する。
func computeDigest(rawJSON []byte) string {
	hash := sha256.Sum256(rawJSON)
	return "sha256:" + hex.EncodeToString(hash[:])
}

// buildDataWithContent はメタデータ + content(生データ) + data_digest を含むdata構造を構築する。
func buildDataWithContent(rawJSON []byte, meta map[string]interface{}) map[string]interface{} {
	if meta == nil {
		meta = make(map[string]interface{})
	}
	meta["raw_sha256"] = computeDigest(rawJSON)
	meta["raw"] = json.RawMessage(rawJSON)
	return meta
}

// SaveResponse は受信データ（WebSocketメッセージ等）を保存する。
func SaveResponse(rawJSON []byte, sourceURL string, crawlSessionID string, savePath string) error {
	meta := map[string]interface{}{
		"source_url":       sourceURL,
		"crawl_session_id": crawlSessionID,
	}
	return saveRecord(RecordTypeResponse, buildDataWithContent(rawJSON, meta), savePath)
}

// SaveRequest は送信リクエスト（WebSocket接続、チャンネル購読等）を保存する。
func SaveRequest(rawJSON []byte, targetURL string, crawlSessionID string, savePath string) error {
	meta := map[string]interface{}{
		"target_url":       targetURL,
		"crawl_session_id": crawlSessionID,
	}
	return saveRecord(RecordTypeRequest, buildDataWithContent(rawJSON, meta), savePath)
}

// SaveMetadata は補足情報（NodeInfo、クロールセッション等）を保存する。
// 用途が多様なため、metaは自由形式のmap。
func SaveMetadata(rawJSON []byte, crawlSessionID string, savePath string, meta map[string]string) error {
	data := make(map[string]interface{})
	for k, v := range meta {
		data[k] = v
	}
	data["crawl_session_id"] = crawlSessionID
	return saveRecord(RecordTypeMetadata, buildDataWithContent(rawJSON, data), savePath)
}
