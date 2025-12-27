package bluesky

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-car/v2"
	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"github.com/chcolte/fediverse-archive-bot-go/models"
	"golang.org/x/net/websocket"
)

type BlueskyProvider struct {
	URL      string
	DownloadDir string
	ws       *websocket.Conn
	subscriptionID string
}

// 新しい BlueskyProvider を作成
func NewBlueskyProvider(url string, downloadDir string) *BlueskyProvider {
	return &BlueskyProvider{
		URL: url,
		DownloadDir: downloadDir,
	}
}

// Bluesky サーバーに WebSocket 接続
func (m *BlueskyProvider) Connect() error {
	wsURL, httpURL := urlAdjust(m.URL)
	ws, err := websocket.Dial(wsURL, "", httpURL)
	if err != nil {
		return err
	}
	m.ws = ws
	logger.Info("Connected to ", wsURL)
	return nil
}

// チャンネルに接続せずとも流れてくる
func (m *BlueskyProvider) ConnectChannel() error {
	return nil
}

// CBORメッセージヘッダー (Bluesky Firehose用)
type FirehoseHeader struct {
	Op int    `cbor:"op"`
	T  string `cbor:"t,omitempty"`
}

// CBORメッセージを受信
func (m *BlueskyProvider) ReceiveMessages(output chan<- models.DownloadItem) error {
	logger.Info("BlueskyProvider: Starting to receive messages")

	// ルートダウンロードディレクトリを作成
	if err := os.MkdirAll(m.DownloadDir, 0755); err != nil {
		logger.Errorf("Failed to create root download directory: %v", err)
		return err
	}

	for {
		var rawMsg []byte
		if err := websocket.Message.Receive(m.ws, &rawMsg); err != nil {
			logger.Errorf("BlueskyProvider: Receive error: %v", err)
			return err
		}

		// CBORデコーダーを作成
		decoder := cbor.NewDecoder(bytes.NewReader(rawMsg))

		// ヘッダーを汎用的なmapとしてデコード
		var headerMap map[string]interface{}
		if err := decoder.Decode(&headerMap); err != nil {
			logger.Debugf("BlueskyProvider: Failed to decode CBOR header: %v", err)
			continue
		}

		// opとtフィールドを抽出
		op, _ := headerMap["op"].(uint64)
		messageType, _ := headerMap["t"].(string)

		// エラーメッセージの場合 (op = -1)
		if op == 0 {
			// 符号付き整数としてデコードされた場合
			if opInt, ok := headerMap["op"].(int64); ok && opInt == -1 {
				logger.Infof("Received error from firehose: %+v", headerMap)
				continue
			}
		}

		// ペイロードを汎用mapとしてデコード
		var payloadMap map[string]interface{}
		if err := decoder.Decode(&payloadMap); err != nil {
			logger.Debugf("BlueskyProvider: Failed to decode payload: %v", err)
			continue
		}

		// 受信時刻と日付
		now := time.Now()
		dateStr := now.Format("2006-01-02")
		
		// 日毎ディレクトリを作成
		dailyDir := filepath.Join(m.DownloadDir, dateStr)
		cborDir := filepath.Join(dailyDir, "cbor")

		if err := os.MkdirAll(cborDir, 0755); err != nil {
			logger.Errorf("Failed to create cbor directory: %v", err)
			continue
		}

		// シーケンス番号を取得（すべてのメッセージタイプで共通）
		var seq uint64
		switch s := payloadMap["seq"].(type) {
		case uint64:
			seq = s
		case int64:
			seq = uint64(s)
		case float64:
			seq = uint64(s)
		default:
			// seqがない場合はタイムスタンプベースのファイル名
			seq = uint64(now.UnixNano())
		}

		// CBORバイナリを保存
		cborFileName := fmt.Sprintf("%d_%s.cbor", seq, strings.TrimPrefix(messageType, "#"))
		cborPath := filepath.Join(cborDir, cborFileName)
		if err := os.WriteFile(cborPath, rawMsg, 0644); err != nil {
			logger.Errorf("Failed to save CBOR file: %v", err)
		}

		// #commitの場合のみ詳細メタデータを作成
		if messageType == "#commit" {
			commit := parseCommitPayload(payloadMap)
			if commit == nil {
				logger.Debugf("BlueskyProvider: Failed to parse commit payload")
				continue
			}

			// 操作情報を変換
			ops := make([]OpInfo, len(commit.Ops))
			for i, op := range commit.Ops {
				ops[i] = OpInfo{
					Action: op.Action,
					Path:   op.Path,
					CID:    cidToString(op.CID),
				}
			}

			// メタデータを作成
			metadata := FirehoseMetadata{
				Seq:        commit.Seq,
				Time:       commit.Time,
				Type:       messageType,
				Repo:       commit.Repo,
				Rev:        commit.Rev,
				Ops:        ops,
				CBORFile:   cborFileName,
				ReceivedAt: now.Format(time.RFC3339),
			}

			// メタデータをJSONL形式で保存
			jsonlPath := filepath.Join(dailyDir, dateStr+".jsonl")
			metadataJSON, err := json.Marshal(metadata)
			if err != nil {
				logger.Errorf("Failed to marshal metadata: %v", err)
			} else {
				m.AppendToFile(string(metadataJSON), jsonlPath)
			}

			// ログ出力
			logger.Debugf("Saved commit seq=%d repo=%s ops=%d", commit.Seq, commit.Repo, len(commit.Ops))

			// CARブロックからメディアURLを抽出
			if len(commit.Blocks) > 0 {
				mediaURLs := m.extractMediaURLsFromCAR(commit.Repo, commit.Blocks)
				for _, url := range mediaURLs {
					logger.Infof("Found media URL: %s", url)
					output <- models.DownloadItem{
						URL:      url,
						Datetime: now,
					}
				}
			}
		} else {
			// #commit以外のメッセージ用の簡易メタデータ
			repo, _ := payloadMap["did"].(string)
			if repo == "" {
				repo, _ = payloadMap["repo"].(string)
			}

			metadata := FirehoseMetadata{
				Seq:        seq,
				Type:       messageType,
				Repo:       repo,
				CBORFile:   cborFileName,
				ReceivedAt: now.Format(time.RFC3339),
			}

			jsonlPath := filepath.Join(dailyDir, dateStr+".jsonl")
			metadataJSON, err := json.Marshal(metadata)
			if err != nil {
				logger.Errorf("Failed to marshal metadata: %v", err)
			} else {
				m.AppendToFile(string(metadataJSON), jsonlPath)
			}

			logger.Debugf("Saved %s message seq=%d", messageType, seq)
		}
	}
}

// マップからCommitPayloadを作成
func parseCommitPayload(m map[string]interface{}) *CommitPayload {
	commit := &CommitPayload{}
	
	// 文字列フィールド
	if repo, ok := m["repo"].(string); ok {
		commit.Repo = repo
	}
	if rev, ok := m["rev"].(string); ok {
		commit.Rev = rev
	}
	if since, ok := m["since"].(string); ok {
		commit.Since = since
	}
	if t, ok := m["time"].(string); ok {
		commit.Time = t
	}
	
	// シーケンス番号 (uint64またはint64)
	switch seq := m["seq"].(type) {
	case uint64:
		commit.Seq = seq
	case int64:
		commit.Seq = uint64(seq)
	case float64:
		commit.Seq = uint64(seq)
	}
	
	// tooBig
	if tooBig, ok := m["tooBig"].(bool); ok {
		commit.TooBig = tooBig
	}
	
	// blocks (バイト配列)
	if blocks, ok := m["blocks"].([]byte); ok {
		commit.Blocks = blocks
	}
	
	// ops配列
	if opsRaw, ok := m["ops"].([]interface{}); ok {
		for _, opRaw := range opsRaw {
			if opMap, ok := opRaw.(map[string]interface{}); ok {
				op := CommitOp{}
				if action, ok := opMap["action"].(string); ok {
					op.Action = action
				}
				if path, ok := opMap["path"].(string); ok {
					op.Path = path
				}
				op.CID = opMap["cid"]
				commit.Ops = append(commit.Ops, op)
			}
		}
	}
	
	return commit
}

// WebSocket接続を閉じる
func (m *BlueskyProvider) Close() error {
	if m.ws != nil {
		return m.ws.Close()
	}
	return nil
}

// マップ内のバイト配列を文字列形式に変換するヘルパー関数
func formatMapWithStrings(m map[string]interface{}) map[string]interface{} {
	result := make(map[string]interface{})
	for k, v := range m {
		switch val := v.(type) {
		case []byte:
			// 印刷可能な文字のみで構成されているか確認
			if isPrintable(val) {
				result[k] = string(val)
			} else if len(val) > 50 {
				result[k] = fmt.Sprintf("[%d bytes]", len(val))
			} else {
				result[k] = fmt.Sprintf("%x", val)
			}
		case map[string]interface{}:
			result[k] = formatMapWithStrings(val)
		default:
			result[k] = v
		}
	}
	return result
}

// バイト配列が印刷可能な文字のみで構成されているか確認
func isPrintable(data []byte) bool {
	for _, b := range data {
		if b < 32 || b > 126 {
			return false
		}
	}
	return len(data) > 0
}

// 受信した生JSONをJson line形式で書き出す
func (m *BlueskyProvider) AppendToFile(text string, filepath string) {
    file, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
    if err != nil {
        logger.Errorf("Failed to open file: %v", err)
    }
    defer file.Close()
    fmt.Fprintln(file, text) //書き込み
}


// URLを変換
func urlAdjust(url string) (ws string, http string) {
	if strings.HasPrefix(url, "https://") {
		return strings.Replace(url, "https://", "wss://", -1), url
	}
	if strings.HasPrefix(url, "http://") {
		return strings.Replace(url, "http://", "ws://", -1), url
	}
	if strings.HasPrefix(url, "wss://") {
		return url, strings.Replace(url, "wss://", "https://", -1)
	}
	if strings.HasPrefix(url, "ws://") {
		return strings.Replace(url, "ws://", "http://", -1), url
	}
	return "wss://" + url, "https://" + url
}

// CIDを文字列に変換
func cidToString(v interface{}) string {
	if v == nil {
		return ""
	}
	
	// CBORでデコードされたCIDはTagとして来ることがある
	switch c := v.(type) {
	case cbor.Tag:
		if bytes, ok := c.Content.([]byte); ok {
			// CIDバイト列をパース
			_, parsed, err := cid.CidFromBytes(bytes)
			if err == nil {
				return parsed.String()
			}
		}
	case []byte:
		_, parsed, err := cid.CidFromBytes(c)
		if err == nil {
			return parsed.String()
		}
		return fmt.Sprintf("%x", c)
	case string:
		return c
	}
	return fmt.Sprintf("%v", v)
}

// Blob URLを構築
func buildBlobURL(did, cidStr string) string {
	return fmt.Sprintf("https://bsky.social/xrpc/com.atproto.sync.getBlob?did=%s&cid=%s", did, cidStr)
}

// CARブロックからメディアURLを抽出
func (m *BlueskyProvider) extractMediaURLsFromCAR(repo string, blocks []byte) []string {
	var urls []string
	
	// CARリーダーを作成
	reader, err := car.NewBlockReader(bytes.NewReader(blocks))
	if err != nil {
		logger.Debugf("Failed to create CAR reader: %v", err)
		return urls
	}

	blockCount := 0
	// 各ブロックを読み込み
	for {
		block, err := reader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Debugf("Error reading CAR block: %v", err)
			break
		}
		blockCount++

		// ブロックデータをCBORとしてデコード
		var record map[string]interface{}
		if err := cbor.Unmarshal(block.RawData(), &record); err != nil {
			continue
		}

		// $typeを確認
		recordType, ok := record["$type"].(string)
		if !ok {
			continue
		}

		logger.Debugf("CAR block record type: %s", recordType)

		// 投稿またはメディア関連のレコードからblobを抽出
		switch recordType {
		case "app.bsky.feed.post":
			// embedフィールドの存在を確認（デバッグ用）
			if embed, ok := record["embed"]; ok {
				if embedMap := toStringMap(embed); embedMap != nil {
					logger.Debugf("Post has embed: %+v", formatMapWithStrings(embedMap))
				}
			}
			urls = append(urls, m.extractBlobsFromPost(repo, record)...)
		case "app.bsky.actor.profile":
			urls = append(urls, m.extractBlobsFromProfile(repo, record)...)
		}
	}

	return urls
}

// 投稿からblobを抽出
func (m *BlueskyProvider) extractBlobsFromPost(repo string, record map[string]interface{}) []string {
	var urls []string

	embed := toStringMap(record["embed"])
	if embed == nil {
		return urls
	}

	embedType, _ := embed["$type"].(string)

	switch embedType {
	case "app.bsky.embed.images":
		// 画像埋め込み
		images, ok := embed["images"].([]interface{})
		if ok {
			for _, img := range images {
				imgMap := toStringMap(img)
				if imgMap == nil {
					continue
				}
				blobRef := extractBlobRef(imgMap["image"])
				if blobRef != "" {
					urls = append(urls, buildBlobURL(repo, blobRef))
				}
			}
		}
	case "app.bsky.embed.video":
		// 動画埋め込み
		blobRef := extractBlobRef(embed["video"])
		if blobRef != "" {
			urls = append(urls, buildBlobURL(repo, blobRef))
		}
	case "app.bsky.embed.external":
		// 外部リンク埋め込み（サムネイル）
		external := toStringMap(embed["external"])
		if external != nil {
			blobRef := extractBlobRef(external["thumb"])
			if blobRef != "" {
				urls = append(urls, buildBlobURL(repo, blobRef))
			}
		}
	case "app.bsky.embed.recordWithMedia":
		// メディア付き引用
		media := toStringMap(embed["media"])
		if media != nil {
			mediaType, _ := media["$type"].(string)
			if mediaType == "app.bsky.embed.images" {
				images, ok := media["images"].([]interface{})
				if ok {
					for _, img := range images {
						imgMap := toStringMap(img)
						if imgMap == nil {
							continue
						}
						blobRef := extractBlobRef(imgMap["image"])
						if blobRef != "" {
							urls = append(urls, buildBlobURL(repo, blobRef))
						}
					}
				}
			}
		}
	}

	return urls
}

// プロフィールからblobを抽出
func (m *BlueskyProvider) extractBlobsFromProfile(repo string, record map[string]interface{}) []string {
	var urls []string

	// アバター
	blobRef := extractBlobRef(record["avatar"])
	if blobRef != "" {
		urls = append(urls, buildBlobURL(repo, blobRef))
	}

	// バナー
	blobRef = extractBlobRef(record["banner"])
	if blobRef != "" {
		urls = append(urls, buildBlobURL(repo, blobRef))
	}

	return urls
}

// Blob参照からCID文字列を抽出
func extractBlobRef(v interface{}) string {
	if v == nil {
		return ""
	}

	blob := toStringMap(v)
	if blob == nil {
		return ""
	}

	// refフィールドを取得
	refVal := blob["ref"]
	if refVal == nil {
		return ""
	}

	// CBORタグ42（CIDリンク）として来る場合
	if refTag, ok := refVal.(cbor.Tag); ok {
		logger.Debugf("Found CID tag: Number=%d", refTag.Number)
		if refTag.Number == 42 { // Tag 42 = CID
			if content, ok := refTag.Content.([]byte); ok {
				// CIDバイト列の先頭バイト(0x00)をスキップ
				if len(content) > 1 && content[0] == 0x00 {
					content = content[1:]
				}
				parsed, err := cid.Decode(string(content))
				if err != nil {
					// バイナリCIDとしてパース
					_, parsed, err = cid.CidFromBytes(content)
				}
				if err == nil {
					return parsed.String()
				}
				logger.Debugf("Failed to parse CID from tag content: %v", err)
			}
		}
	}

	// ref.$linkを取得（マップの場合）
	ref := toStringMap(refVal)
	if ref != nil {
		if link, ok := ref["$link"].(string); ok {
			return link
		}
		// $linkがbyte配列の場合
		if linkBytes, ok := ref["$link"].([]byte); ok {
			_, parsed, err := cid.CidFromBytes(linkBytes)
			if err == nil {
				return parsed.String()
			}
		}
		// $linkがCBORタグの場合
		if linkTag, ok := ref["$link"].(cbor.Tag); ok {
			if content, ok := linkTag.Content.([]byte); ok {
				_, parsed, err := cid.CidFromBytes(content)
				if err == nil {
					return parsed.String()
				}
			}
		}
	}

	return ""
}

// map[interface{}]interface{}をmap[string]interface{}に変換するヘルパー関数
func toStringMap(v interface{}) map[string]interface{} {
	if v == nil {
		return nil
	}

	// すでにmap[string]interface{}の場合
	if m, ok := v.(map[string]interface{}); ok {
		return m
	}

	// map[interface{}]interface{}の場合（CBORのデフォルト）
	if m, ok := v.(map[interface{}]interface{}); ok {
		result := make(map[string]interface{})
		for k, val := range m {
			if ks, ok := k.(string); ok {
				result[ks] = val
			}
		}
		return result
	}

	return nil
}

