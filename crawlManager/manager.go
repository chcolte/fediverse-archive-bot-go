package crawlManager

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"github.com/chcolte/fediverse-archive-bot-go/media-downloader"
	"github.com/chcolte/fediverse-archive-bot-go/models"
	"github.com/chcolte/fediverse-archive-bot-go/nodeinfo"
	"github.com/chcolte/fediverse-archive-bot-go/providers"
	"github.com/chcolte/fediverse-archive-bot-go/providers/bluesky"
	"github.com/chcolte/fediverse-archive-bot-go/providers/mastodon"
	"github.com/chcolte/fediverse-archive-bot-go/providers/misskey"
	"github.com/chcolte/fediverse-archive-bot-go/providers/nostr"
	"github.com/chcolte/fediverse-archive-bot-go/utils"
	"github.com/google/uuid"
)

/*
[オブジェクトの階層]
Server
    ↑ has
Connection (接続)
   ↑ use  ↑ use
Archiver   Explorer
*/

// Connection: サーバーへの接続を抽象化
type Connection struct {
	Target   models.Target
	Provider providers.PlatformProvider
}

// Archiver: TLの保存
type Archiver struct {
	Conn    *Connection
	DLQueue chan models.DownloadItem
	WG      *sync.WaitGroup
	CrawlSessionID string
}

// Explorer: 新規サーバー探索
type Explorer struct {
	Conn        *Connection
	ServerQueue chan models.Server
	WG          *sync.WaitGroup
	CrawlSessionID string
}

// CrawlManager: 全体管理
type CrawlManager struct {
	NewServerReceiver chan models.Server // 新規サーバー受付チャネル
	ArchiverRegistry  map[string]*Archiver
	ExplorerRegistry  map[string]*Explorer
	KnownServers      map[string]models.Server
	RegistryLock      sync.RWMutex

	DownloadDir      string
	Mode             string // live, past
	Media            bool
	ParallelDownload int
	Scope            string
	Timelines        []string // 共通タイムライン名のリスト (local, global)
}

func NewCrawlManager(downloadDir string, mode string, media bool, parallelDownload int, scope string, timelines []string) *CrawlManager {
	return &CrawlManager{
		NewServerReceiver: make(chan models.Server, 100),
		ArchiverRegistry:  make(map[string]*Archiver),
		ExplorerRegistry:  make(map[string]*Explorer),
		KnownServers:      make(map[string]models.Server),
		RegistryLock:      sync.RWMutex{},
		DownloadDir:       downloadDir,
		Mode:              mode,
		Media:             media,
		ParallelDownload:  parallelDownload,
		Scope:             scope,
		Timelines:         timelines,
	}
}

func makeRegistryKey(target models.Target) string {
	return target.Server.URL + ":" + target.Timeline
}

// MEMO: ここの中で重複排除するべき？同じサイトでArchiverを更新したくなることはある？
func (c *CrawlManager) registerArchiver(archiver *Archiver) {
	c.RegistryLock.Lock()
	defer c.RegistryLock.Unlock()
	c.ArchiverRegistry[makeRegistryKey(archiver.Conn.Target)] = archiver
	logger.Debugf("Added archiver for %s (%s)", archiver.Conn.Target.Server.URL, archiver.Conn.Target.Timeline)
}

func (c *CrawlManager) registerExplorer(explorer *Explorer) {
	c.RegistryLock.Lock()
	defer c.RegistryLock.Unlock()
	c.ExplorerRegistry[makeRegistryKey(explorer.Conn.Target)] = explorer
	logger.Debugf("Added explorer for %s (%s)", explorer.Conn.Target.Server.URL, explorer.Conn.Target.Timeline)
}

func (c *CrawlManager) archiverExists(target models.Target) bool {
	c.RegistryLock.RLock()
	defer c.RegistryLock.RUnlock()
	_, exists := c.ArchiverRegistry[makeRegistryKey(target)]
	return exists
}

func (c *CrawlManager) explorerExists(target models.Target) bool {
	c.RegistryLock.RLock()
	defer c.RegistryLock.RUnlock()
	_, exists := c.ExplorerRegistry[makeRegistryKey(target)]
	return exists
}

func (c *CrawlManager) isObservedServer(server models.Server) bool {
	// 全てのタイムラインに対してチェック
	for _, tl := range c.Timelines {
		if c.archiverExists(models.Target{Server: server, Timeline: tl}) {
			return true
		}
	}
	return c.explorerExists(models.Target{Server: server, Timeline: models.TimelineGlobal})
}

func (c *CrawlManager) isKnownServer(server models.Server) bool {
	c.RegistryLock.RLock()
	defer c.RegistryLock.RUnlock()
	_, exists := c.KnownServers[server.URL]
	return exists
}

func (c *CrawlManager) addKnownServer(server models.Server) {
	c.RegistryLock.Lock()
	c.KnownServers[server.URL] = server
	c.RegistryLock.Unlock()

	serverListPath := filepath.Join(c.DownloadDir, server.Type, "server_list.txt")
	c.AppendToFile(server.URL+" "+server.Type, serverListPath)

	// NodeInfoを取得して保存（Nostr, Bluesky等は失敗してもスキップ）
	c.saveNodeInfo(server)
}

func (c *CrawlManager) saveNodeInfo(server models.Server) {
	info, err := nodeinfo.GetNodeInfo(server.URL)
	if err != nil {
		logger.Debugf("NodeInfo not available for %s: %v", server.URL, err)
		return
	}

	savePath := filepath.Join(c.DownloadDir, server.Type, server.URL, "nodeinfo.jsonl")

	extraMeta := map[string]string{
		"source_url":     fmt.Sprintf("https://%s/.well-known/nodeinfo", server.URL),
		"server_url":     server.URL,
		"server_type":    server.Type,
		"schema_version": info.Version,
	}

	if err := utils.SaveMetadata([]byte(info.RawJSON), "", savePath, extraMeta); err != nil {
		logger.Errorf("Failed to save nodeinfo for %s: %v", server.URL, err)
	}
}

// saveCrawlSession はクロールセッションのメタデータを保存する
func (c *CrawlManager) saveCrawlSession(crawlSessionID string, target models.Target, role string) {
	savePath := filepath.Join(c.DownloadDir, target.Server.Type, target.Server.URL, "crawl_sessions.jsonl")
	meta := map[string]string{
		"server_url":  target.Server.URL,
		"server_type": target.Server.Type,
		"timeline":    target.Timeline,
		"role":        role,
	}
	if err := utils.SaveMetadata(nil, crawlSessionID, savePath, meta); err != nil {
		logger.Errorf("Failed to save crawl session: %v", err)
	}
}


func (c *CrawlManager) Start() {
	for server := range c.NewServerReceiver {
		// サーバーリストを更新
		if c.isKnownServer(server) {
			continue
		}else{
			c.addKnownServer(server)
		}

		// Scope: system名の除外処理だけココでやろうとしてるから，変なことになってる。
		if (c.Scope != "unbounded" && c.Scope != "server" && c.Scope != server.Type) {
			continue
		}

		logger.Debug("Received new server: ", server)

		// ------------Archiver--------------
		// 各タイムラインに対してArchiverを作成
		for _, timeline := range c.Timelines {
			archiverTarget := models.Target{
				Server:   server,
				Timeline: timeline,
			}

			// 重複チェック
			if c.archiverExists(archiverTarget) {
				continue
			}

			archiverConn, err := c.createConnection(archiverTarget)
			if err != nil {
				logger.Errorf("Failed to create archiver connection for %s (%s): %v", server.URL, timeline, err)
				continue
			}

			archiver := &Archiver{
				Conn:    archiverConn,
				DLQueue: make(chan models.DownloadItem, 100),
				WG:      &sync.WaitGroup{},
				CrawlSessionID: uuid.New().String(),
			}
			c.registerArchiver(archiver)

			// Archiverを開始
			c.startArchiver(archiver)
		}

		// ------------Explorer--------------
		var explore = true;
		if c.Scope == "server"{
			explore = false;
		}

		if explore {
			explorerTarget := models.Target{
				Server:   server,
				Timeline: models.TimelineGlobal,
			}

			// 重複チェック
			if c.explorerExists(explorerTarget) {
				continue
			}
		
			explorerConn, err := c.createConnection(explorerTarget)
			if err != nil {
				logger.Error("Failed to create explorer connection: ", err)
				continue
			}

			explorer := &Explorer{
				Conn:        explorerConn,
				ServerQueue: c.NewServerReceiver,
				WG:          &sync.WaitGroup{},
				CrawlSessionID: uuid.New().String(),
			}
			c.registerExplorer(explorer)

			// Explorerを開始
			c.startExplorer(explorer)
		}
	}
}

func (c *CrawlManager) Stop() {
	// TODO: implement
}

func (c *CrawlManager) createConnection(target models.Target) (*Connection, error) {
	provider, err := c.getProvider(target)
	if err != nil {
		return nil, err
	}
	return &Connection{
		Target:   target,
		Provider: provider,
	}, nil
}

func (c *CrawlManager) getProvider(target models.Target) (providers.PlatformProvider, error) {
	server := target.Server
	downloadPath := filepath.Join(c.DownloadDir, server.Type, server.URL)

	switch server.Type {
	case "misskey":
		return misskey.NewMisskeyProvider(server.URL, target.Timeline, downloadPath), nil
	case "nostr":
		return nostr.NewNostrProvider(server.URL, downloadPath), nil
	case "bluesky":
		return bluesky.NewBlueskyProvider(server.URL, downloadPath), nil
	case "mastodon":
		return mastodon.NewMastodonProvider(server.URL, target.Timeline, downloadPath), nil
	default:
		return nil, errors.New("unsupported system specified: " + server.Type)
	}
}

// Archiverを開始
func (c *CrawlManager) startArchiver(archiver *Archiver) {
	conn := archiver.Conn

	// クロールセッション情報を記録
	c.saveCrawlSession(archiver.CrawlSessionID, conn.Target, "archiver")

	// 接続
	targetURL, err := conn.Provider.Connect()
	if err != nil {
		logger.Error("Failed to connect:", err)
		close(archiver.DLQueue)
		return
	}
	savePath := filepath.Join(c.DownloadDir, conn.Target.Server.Type, conn.Target.Server.URL, "crawl_sessions.jsonl")
	utils.SaveRequest(nil, targetURL, archiver.CrawlSessionID, savePath)

	sentMsg, err := conn.Provider.ConnectChannel()
	if err != nil {
		logger.Error("Failed to connect channel:", err)
		close(archiver.DLQueue)
		return
	}
	if sentMsg != nil {
		utils.SaveRequest(sentMsg, targetURL, archiver.CrawlSessionID, savePath)
	}

	// 受信
	archiver.WG.Add(1)
	go func() {
		defer archiver.WG.Done()
		defer conn.Provider.Close()
		for {
			if err := conn.Provider.ReceiveMessages(archiver.DLQueue); err != nil {
				logger.Errorf("ReceiveMessages error: %v. Reconnecting in 5 seconds... [%s]", err, conn.Target.Server.URL)
				time.Sleep(5 * time.Second)

				// 再接続
				conn.Provider.Close()

				archiver.CrawlSessionID = uuid.New().String()
				c.saveCrawlSession(archiver.CrawlSessionID, conn.Target, "archiver")
				
				if targetURL, err := conn.Provider.Connect(); err != nil {
					logger.Errorf("Reconnect failed: %v. Retrying... [%s]", err, conn.Target.Server.URL)
					continue
				} else {
					utils.SaveRequest(nil, targetURL, archiver.CrawlSessionID, savePath)
				}
				if sentMsg, err := conn.Provider.ConnectChannel(); err != nil {
					logger.Errorf("ReconnectChannel failed: %v. Retrying... [%s]", err, conn.Target.Server.URL)
					continue
				} else if sentMsg != nil {
					utils.SaveRequest(sentMsg, targetURL, archiver.CrawlSessionID, savePath)
				}
				logger.Infof("Reconnected successfully [%s]", conn.Target.Server.URL)

				// TODO: ダウンタイムの間のポストをREST APIで取得する処理

			} else {
				break
			}
		}
	}()

	// ダウンローダーを開始
	if c.Media {
		for i := 0; i < c.ParallelDownload; i++ {
			archiver.WG.Add(1)
			go mediaDownloader.MediaDownloader(archiver.DLQueue, archiver.WG, c.DownloadDir)
		}
	} else {
		go func() {
			defer archiver.WG.Done()
			for item := range archiver.DLQueue {
				logger.Debug("Discarding item:", item)
			}
		}()
	}
}

// Explorerを開始
func (c *CrawlManager) startExplorer(explorer *Explorer) {
	conn := explorer.Conn

	// クロールセッション情報を記録
	c.saveCrawlSession(explorer.CrawlSessionID, conn.Target, "explorer")

	// 接続
	targetURL, err := conn.Provider.Connect()
	if err != nil {
		logger.Error("Failed to connect:", err)
		return
	}
	savePath := filepath.Join(c.DownloadDir, conn.Target.Server.Type, conn.Target.Server.URL, "crawl_sessions.jsonl")
	utils.SaveRequest(nil, targetURL, explorer.CrawlSessionID, savePath)

	sentMsg, err := conn.Provider.ConnectChannel()
	if err != nil {
		logger.Error("Failed to connect channel:", err)
		return
	}
	if sentMsg != nil {
		utils.SaveRequest(sentMsg, targetURL, explorer.CrawlSessionID, savePath)
	}

	// 受信
	explorer.WG.Add(1)
	go func() {
		defer explorer.WG.Done()
		defer conn.Provider.Close()
		for {
			if err := conn.Provider.CrawlNewServer(explorer.ServerQueue); err != nil {
				logger.Errorf("CrawlNewServer error: %v. Reconnecting in 5 seconds...", err)
				time.Sleep(5 * time.Second)

				// 再接続
				conn.Provider.Close()

				explorer.CrawlSessionID = uuid.New().String()
				c.saveCrawlSession(explorer.CrawlSessionID, conn.Target, "explorer")

				if targetURL, err := conn.Provider.Connect(); err != nil {
					logger.Errorf("Reconnect failed: %v. Retrying...", err)
					continue
				} else {
					utils.SaveRequest(nil, targetURL, explorer.CrawlSessionID, savePath)
				}
				if sentMsg, err := conn.Provider.ConnectChannel(); err != nil {
					logger.Errorf("ReconnectChannel failed: %v. Retrying...", err)
					continue
				} else if sentMsg != nil {
					utils.SaveRequest(sentMsg, targetURL, explorer.CrawlSessionID, savePath)
				}
				logger.Info("Reconnected successfully")

				// TODO: ダウンタイムの間のポストをREST APIで取得する処理

			} else {
				break
			}
		}
	}()
}

func (c *CrawlManager) AppendToFile(text string, filePath string) {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Errorf("Failed to create directory: %v", err)
		return
	}

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		logger.Errorf("Failed to open file: %v", err)
		return
	}
	defer file.Close()
	fmt.Fprintln(file, text)
}