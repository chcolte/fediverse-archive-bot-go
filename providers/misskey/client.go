package misskey

import (
	"encoding/json"
	"strings"

	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"github.com/chcolte/fediverse-archive-bot-go/models"
	"github.com/google/uuid"
	"golang.org/x/net/websocket"
)

type MisskeyProvider struct {
	URL      string
	Timeline string
	ws       *websocket.Conn
}

// 新しい MisskeyProvider を作成
func NewMisskeyProvider(url, timeline string) *MisskeyProvider {
	return &MisskeyProvider{
		URL:      url,
		Timeline: timeline,
	}
}

// Misskey サーバーに WebSocket 接続
func (m *MisskeyProvider) Connect() error {
	wsURL, httpURL := urlAdjust(m.URL)
	ws, err := websocket.Dial(wsURL+"/streaming", "", httpURL)
	if err != nil {
		return err
	}
	m.ws = ws
	logger.Info("Connected to", wsURL)
	return nil
}

// 指定されたタイムラインチャンネルに接続
func (m *MisskeyProvider) ConnectChannel() error {
	id, err := uuid.NewRandom()
	if err != nil {
		return err
	}

	msg := `{
		"type": "connect",
		"body": {
			"channel": "` + m.Timeline + `",
			"id": "` + id.String() + `"
		}
	}`
	logger.Debug("Send message: ", msg)

	if err := websocket.Message.Send(m.ws, msg); err != nil {
		return err
	}

	logger.Info("Connected to channel:", m.Timeline)
	return nil
}

// メッセージを受信し、メディアURLを output チャンネルに送信
func (m *MisskeyProvider) ReceiveMessages(output chan<- models.DownloadItem) {
	logger.Info("MisskeyProvider: Starting to receive messages")

	for {
		var rawMsg string
		if err := websocket.Message.Receive(m.ws, &rawMsg); err != nil {
			logger.Errorf("MisskeyProvider: Receive error: %v", err)
			continue
		}

		// メッセージをパース
		msg := m.parseStreamingMessage(rawMsg)
		note := m.getNoteFromStreamingMessage(msg)
		
		// URLを抽出
		urls := SafeExtractURL(note)

		for _, url := range urls {
			output <- models.DownloadItem{
				URL:      url,
				Datetime: note.CreatedAt,
			}
		}
	}
}

// WebSocket 接続を閉じる
func (m *MisskeyProvider) Close() error {
	if m.ws != nil {
		return m.ws.Close()
	}
	return nil
}

func (m *MisskeyProvider) parseStreamingMessage(raw string) StreamingMessage {
	var msg StreamingMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		logger.Errorf("Failed to parse message: %v", err)
		return StreamingMessage{}
	}
	return msg
}

func (m *MisskeyProvider) getNoteFromStreamingMessage(msg StreamingMessage) Note {
	return msg.Body.Body
}


// urlAdjust は URL を WebSocket 用に変換する
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
