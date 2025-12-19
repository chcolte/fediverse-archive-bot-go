package nostr

import (
	//"encoding/json"
	"strings"
	"os"
	"fmt"
	//"time"
	"path/filepath"
	"encoding/json"

	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"github.com/chcolte/fediverse-archive-bot-go/models"
	"golang.org/x/net/websocket"
	"github.com/google/uuid"
)

type NostrProvider struct {
	URL      string
	DownloadDir string
	ws       *websocket.Conn
}

// 新しい NostrProvider を作成
func NewNostrProvider(url string, downloadDir string) *NostrProvider {
	return &NostrProvider{
		URL: url,
		DownloadDir: downloadDir,
	}
}

// Nostr サーバーに WebSocket 接続
func (m *NostrProvider) Connect() error {
	wsURL, httpURL := urlAdjust(m.URL)
	ws, err := websocket.Dial(wsURL, "", httpURL)
	if err != nil {
		return err
	}
	m.ws = ws
	logger.Info("Connected to ", wsURL)
	return nil
}

// チャンネルに接続
func (m *NostrProvider) ConnectChannel() error {
	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	msg := `[
		"REQ",
		"` + id.String() + `",
		{ }
	]`
	logger.Debug("Send message: ", msg)

	if err := websocket.Message.Send(m.ws, msg); err != nil {
		return err
	}

	logger.Info("Connected to Timeline.")
	return nil
}

// メッセージを受信
func (m *NostrProvider) ReceiveMessages(output chan<- models.DownloadItem) {
	logger.Info("NostrProvider: Starting to receive messages")

	// ルートダウンロードディレクトリを作成
	if err := os.MkdirAll(m.DownloadDir, 0755); err != nil {
		logger.Errorf("Failed to create root download directory: %v", err)
		return
	}

	for {
		var rawMsg string
		if err := websocket.Message.Receive(m.ws, &rawMsg); err != nil {
			logger.Errorf("NostrProvider: Receive error: %v", err)
			continue
		}
		logger.Debug("Received message: ", rawMsg)

		// //メッセージパース
		// msg, _ := unmarshalJSON([]byte(rawMsg))

		// // 日毎ディレクトリを作成(なければ)
		// dateStr := time.Unix(msg.Event.CreatedAt, 0).Format("2006-01-02")
		// dailyDir := filepath.Join(m.DownloadDir, dateStr)
		// assetsDir := filepath.Join(dailyDir, "data")

		// if err := os.MkdirAll(assetsDir, 0755); err != nil {
		// 	logger.Errorf("Failed to create dailydownload directory: %v", err)
		// 	continue
		// }

		// 受信した生メッセージを保存
		JSONSavePath := filepath.Join(m.DownloadDir, "download.jsonl")
		m.AppendToFile(rawMsg, JSONSavePath)

		//URL抽出
	}
}

// WebSocket接続を閉じる
func (m *NostrProvider) Close() error {
	if m.ws != nil {
		return m.ws.Close()
	}
	return nil
}

// 受信した生JSONをJson line形式で書き出す
func (m *NostrProvider) AppendToFile(text string, filepath string) {
    file, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
    if err != nil {
        logger.Errorf("Failed to open file: %v", err)
    }
    defer file.Close()
    fmt.Fprintln(file, text) //書き込み
}


// func (m *NostrProvider) parseStreamingMessage(raw string) StreamingMessage {
// 	var msg StreamingMessage
// 	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
// 		logger.Errorf("Failed to parse message: %v", err)
// 		return StreamingMessage{}
// 	}
// 	return msg
// }

// func (m *NostrProvider) getNoteFromStreamingMessage(msg StreamingMessage) Note {
// 	return msg.Body.Body
// }


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

// 生メッセージを構造体に変換(動かない)
func unmarshalJSON(data []byte) (NostrMessage, error) {
	var raw []json.RawMessage
	var NostrMessage NostrMessage

	if err := json.Unmarshal(data, &raw); err != nil {
		return NostrMessage, err
	}

	if len(raw) < 1 {
		return NostrMessage, fmt.Errorf("empty message")
	}

	// First element: message type
	if err := json.Unmarshal(raw[0], &NostrMessage.Type); err != nil {
		return NostrMessage, err
	}

	switch NostrMessage.Type {
	case "EVENT":
		if len(raw) >= 3 {
			if err := json.Unmarshal(raw[1], &NostrMessage.SubscriptionID); err != nil {
				return NostrMessage, err
			}
			NostrMessage.Event = &NostrEvent{}
			if err := json.Unmarshal(raw[2], NostrMessage.Event); err != nil {
				return NostrMessage, err
			}
		}
	case "NOTICE":
		if len(raw) >= 2 {
			if err := json.Unmarshal(raw[1], &NostrMessage.Message); err != nil {
				return NostrMessage, err
			}
		}
	case "EOSE":
		if len(raw) >= 2 {
			if err := json.Unmarshal(raw[1], &NostrMessage.SubscriptionID); err != nil {
				return NostrMessage, err
			}
		}
	case "OK":
		// OK messages: ["OK", "event_id", true/false, "message"]
		// Handle as needed
	}

	return NostrMessage, nil
}

