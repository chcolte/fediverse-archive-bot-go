package mastodon

import (
	"encoding/json"
	"strings"
	"os"
	"path/filepath"
	"fmt"
	"time"
	"net/url"

	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"github.com/chcolte/fediverse-archive-bot-go/models"
	"github.com/chcolte/fediverse-archive-bot-go/nodeinfo"
	"github.com/chcolte/fediverse-archive-bot-go/providers"
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
// WebSocketのHTTP HeaderとResponseが取りたいところだが，今のパッケージだと無理
func (m *MastodonProvider) Connect() (string, error) {
	wsURL, httpURL := urlAdjust(m.URL)

	tl := m.convertTimeline()

	streamURL := wsURL + "/api/v1/streaming/?stream=" + tl

	// まず認証なしで接続を試みる
	ws, err := m.tryConnect(streamURL, httpURL, "")
	if err == nil {
		m.ws = ws
		logger.Info("Connected to ", streamURL, " (no auth)")
		return streamURL, nil
	}
	
	logger.Debugf("Connection without auth failed for %s: %v, trying with token...", m.URL, err)

	// 認証なしで失敗した場合、トークン付きでリトライ
	accessToken, tokenErr := GetAccessToken(m.URL)
	if tokenErr != nil {
		logger.Errorf("Failed to get access token for %s: %v", m.URL, tokenErr)
		return streamURL, err // 元のエラーを返す
	}

	ws, err = m.tryConnect(streamURL+"&access_token="+accessToken, httpURL, accessToken)
	if err != nil {
		return streamURL, fmt.Errorf("connection failed both with and without auth: %w", err)
	}

	m.ws = ws
	logger.Info("Connected to ", streamURL, " (with token)")
	return streamURL, nil
}

// tryConnect attempts to establish a WebSocket connection
func (m *MastodonProvider) tryConnect(streamURL, origin, token string) (*websocket.Conn, error) {
	config, err := websocket.NewConfig(streamURL, origin)
	if err != nil {
		return nil, err
	}

	// User-Agentを設定
	config.Header.Set("User-Agent", "Mozilla/5.0 (compatible; FediverseArchiveBot/1.0)")
	
	// トークンがある場合はヘッダーにも設定
	if token != "" {
		config.Header.Set("Authorization", "Bearer "+token)
	}

	return websocket.DialConfig(config)
}

// 指定されたタイムラインチャンネルに接続 (WebSocket接続時にすでに接続済み)
func (m *MastodonProvider) ConnectChannel() ([]byte, error) {
	logger.Info("Already connected to channel: ", m.Timeline)
	return nil, nil
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
		var payload Payload
		msg := m.parseStreamingMessage(rawMsg)
		if(msg.Event == "update"){
			payload = m.getPayloadFromStreamingMessage(msg)
			logger.Debug("Received message: ", msg.Event)
			logger.Debug("Received message: ", payload.ID)
			logger.Debug("Received message: ", payload.CreatedAt)
			logger.Debug("Received message: ", payload.URL)
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
		JSONSavePath := filepath.Join(dailyDir, dateStr+"_"+m.Timeline+".jsonl")
		providers.AppendToFile(rawMsg, JSONSavePath)

		// URLを抽出
		urls := m.extractMediaURLsFromPayload(payload)
		

		// URLをDLキューに送信
		for _, url := range urls {
			output <- models.DownloadItem{
				URL:      url,
				Datetime: payload.CreatedAt,
			}
		}
	}
}


func (m *MastodonProvider) CrawlNewServer(server chan <- models.Server) error {
	logger.Info("MastodonProvider: Starting to crawl new servers [", m.URL, "]")
	for {
		// メッセージを受信
		var rawMsg string
		if err := websocket.Message.Receive(m.ws, &rawMsg); err != nil {
			logger.Errorf("MastodonProvider: Receive error: %v", err)
			return err
		}

		// メッセージをパース
		msg := m.parseStreamingMessage(rawMsg)
		if(msg.Event != "update") {continue}
		payload := m.getPayloadFromStreamingMessage(msg)

		// サーバーを通知
		u, err := url.Parse(payload.URL)
		if err != nil {
			logger.Errorf("Failed to parse URL: %v", err)
			continue
		}
		
		if u.Host != m.URL {
			softwareName, err := nodeinfo.GetSoftwareName(u.Host)
			if err != nil {
				logger.Errorf("Failed to get software name: %v", err)
				continue
			}
			server <- models.Server{
				Type: softwareName,
				URL: u.Host,
			}
		}
	}
	return nil
}


// WebSocket 接続を閉じる
func (m *MastodonProvider) Close() error {
	if m.ws != nil {
		return m.ws.Close()
	}
	return nil
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

func (m *MastodonProvider) extractMediaURLsFromPayload(payload Payload) []string {
	var urls []string

	for _, attachment := range payload.MediaAttachments {
		urls = append(urls, attachment.URL)
		urls = append(urls, attachment.PreviewURL) //?
		urls = append(urls, attachment.RemoteURL) // ?
		urls = append(urls, attachment.TextURL) //?
	}
	for _, emoji := range payload.Emojis {
		urls = append(urls, emoji.URL)
		urls = append(urls, emoji.StaticURL)
	}
	for _, emoji := range payload.Account.Emojis {
		urls = append(urls, emoji.URL)
		urls = append(urls, emoji.StaticURL)
	}
	urls = append(urls, payload.Account.Avatar)
	urls = append(urls, payload.Account.AvatarStatic)
	urls = append(urls, payload.Account.Header)
	urls = append(urls, payload.Account.HeaderStatic)
	for _, emoji := range payload.Poll.Emojis {
		urls = append(urls, emoji.URL)
		urls = append(urls, emoji.StaticURL)
	}
	return urls
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

// convertTimeline は共通タイムライン名をMastodon用に変換する
func (m *MastodonProvider) convertTimeline() string {
	switch m.Timeline {
	case models.TimelineLocal, "localTimeline": // 後方互換性のため旧名も対応
		return "public:local"
	case models.TimelineGlobal, "globalTimeline":
		return "public"
	default:
		// デフォルトは連合TL
		return "public"
	}
}
