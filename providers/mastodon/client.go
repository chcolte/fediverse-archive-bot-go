package mastodon

import (
	"encoding/json"
	"strings"
	"os"
	"path/filepath"
	"fmt"
	"time"

	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"github.com/chcolte/fediverse-archive-bot-go/models"
	"golang.org/x/net/websocket"
)

type MastodonProvider struct {
	URL      string
	Timeline string
	DownloadDir string
	ws       *websocket.Conn
}

// 新しい MastodonProvider を作成
func NewMastodonProvider(url, timeline, downloadDir string) *MastodonProvider {
	return &MastodonProvider{
		URL:      url,
		Timeline: timeline,
		DownloadDir: downloadDir,
	}
}

// Mastodon サーバーに WebSocket 接続
func (m *MastodonProvider) Connect() error {
	wsURL, httpURL := urlAdjust(m.URL)

	tl := "public"
	if (m.Timeline == "localTimeline") {
		tl = "public:local"
	}

	streamURL := wsURL + "/api/v1/streaming/?stream=" + tl

	config, err := websocket.NewConfig(streamURL, httpURL)
	if err != nil {
		return err
	}

	// User-Agentを設定（設定しないとなんかブロックされる）
	config.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FediverseArchiveBot/1.0)")

	ws, err := websocket.DialConfig(config)
	if err != nil {
		return err
	}
	m.ws = ws
	logger.Info("Connected to ", streamURL)
	return nil
}

// 指定されたタイムラインチャンネルに接続 (WebSocket接続時にすでに接続済み)
func (m *MastodonProvider) ConnectChannel() error {
	logger.Info("Already connected to channel: ", m.Timeline)
	return nil
}

// メッセージを受信し、メディアURLを output チャンネルに送信
func (m *MastodonProvider) ReceiveMessages(output chan<- models.DownloadItem) error {
	logger.Info("MastodonProvider: Starting to receive messages")
	
	// ルートダウンロードディレクトリを作成
	if err := os.MkdirAll(m.DownloadDir, 0755); err != nil {
		logger.Errorf("Failed to create root download directory: %v", err)
		return err
	}
	
	for {
		// メッセージを受信
		var rawMsg string
		if err := websocket.Message.Receive(m.ws, &rawMsg); err != nil {
			logger.Errorf("MastodonProvider: Receive error: %v", err)
			return err
		}

		// メッセージをパース
		dateStr := ""
		msg := m.parseStreamingMessage(rawMsg)
		if(msg.Event == "update"){
			payload := m.getPayloadFromStreamingMessage(msg)
			logger.Info("Received message: ", msg.Event)
			logger.Info("Received message: ", payload.ID)
			logger.Info("Received message: ", payload.CreatedAt)
			logger.Info("Received message: ", payload.URL)
			dateStr = payload.CreatedAt.Format("2006-01-02")
			
		}else if(msg.Event == "delete"){
			dateStr = time.Now().Format("2006-01-02")
		}
		
		// 日毎ディレクトリを作成(なければ)
		
		dailyDir := filepath.Join(m.DownloadDir, dateStr)
		assetsDir := filepath.Join(dailyDir, "data")

		if err := os.MkdirAll(assetsDir, 0755); err != nil {
			logger.Errorf("Failed to create dailydownload directory: %v", err)
			continue
		}

		//受信した生メッセージを保存
		JSONSavePath := filepath.Join(dailyDir, dateStr+".jsonl")
		m.AppendToFile(rawMsg, JSONSavePath)

		// URLを抽出
		

		// URLをDLキューに送信
		// for _, url := range urls {
		// 	output <- models.DownloadItem{
		// 		URL:      url,
		// 		Datetime: note.CreatedAt,
		// 	}
		// }
	}
}


func (m *MastodonProvider) CrawlNewServer(server chan <- models.ServerInfo) error {
	logger.Info("MastodonProvider: Starting to crawl new servers")
	
	return nil
	// for {
	// TODO: implement
	// }
}


// WebSocket 接続を閉じる
func (m *MastodonProvider) Close() error {
	if m.ws != nil {
		return m.ws.Close()
	}
	return nil
}

// 受信した生JSONをJson line形式で書き出す
func (m *MastodonProvider) AppendToFile(text string, filepath string) {
    file, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
    if err != nil {
        logger.Errorf("Failed to open file: %v", err)
    }
    defer file.Close()
    fmt.Fprintln(file, text) //書き込み
}

func (m *MastodonProvider) parseStreamingMessage(raw string) StreamingMessage {
	var msg StreamingMessage
	if err := json.Unmarshal([]byte(raw), &msg); err != nil {
		logger.Errorf("Failed to parse message: %v", err)
		return StreamingMessage{}
	}

	return msg
}

func (m *MastodonProvider) getPayloadFromStreamingMessage(msg StreamingMessage) Payload {
	var payload Payload
	if err := json.Unmarshal([]byte(msg.Payload), &payload); err != nil {
		logger.Errorf("Failed to parse payload: %v", err)
		return Payload{}
	}
	return payload
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
