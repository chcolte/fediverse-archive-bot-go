package crawlManager

import (
	"sync"
	"time"
	"errors"
	"path/filepath"

	"github.com/chcolte/fediverse-archive-bot-go/providers"
	"github.com/chcolte/fediverse-archive-bot-go/models"
	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"github.com/chcolte/fediverse-archive-bot-go/providers/bluesky"
	"github.com/chcolte/fediverse-archive-bot-go/providers/mastodon"
	"github.com/chcolte/fediverse-archive-bot-go/providers/misskey"
	"github.com/chcolte/fediverse-archive-bot-go/providers/nostr"
	"github.com/chcolte/fediverse-archive-bot-go/media-downloader"
)

// サーバーの基本情報
type ServerInfo struct {
	Type string // bluesky, mastodon, misskey, nostr
	URL string // baseURL
	Timeline string // timeline name
}

// 実際に稼働中のサーバーの管理情報　（ServerInfoの上位存在，正常に稼働開始しているサーバーならば追加要素も埋められるはず）
type ServerManager struct {
	ServerInfo

	provider providers.PlatformProvider // Provider instance
	dlqueue chan models.DownloadItem // Download queue
	wg *sync.WaitGroup // WaitGroup
}

type CrawlManager struct {
	NewServerReceiver chan ServerInfo // 新規サーバー受付チャネル
	ServerRegistry map[string]ServerManager // ServerRegistry
	ServerRegistryLock sync.RWMutex // ServerRegistryを操作する際の排他ロック

	DownloadDir string
	Mode string // live, past
	Media bool
	ParallelDownload int
}

func NewCrawlManager(downloadDir string, mode string, media bool, parallelDownload int) *CrawlManager {
	return &CrawlManager{
		NewServerReceiver: make(chan ServerInfo, 10),
		ServerRegistry: make(map[string]ServerManager),
		ServerRegistryLock: sync.RWMutex{},
		DownloadDir: downloadDir,
		Mode: mode,
		Media: media,
		ParallelDownload: parallelDownload,
	}
}

func (c *CrawlManager) addServerToRegistry(server ServerManager) {
	c.ServerRegistryLock.Lock()
	c.ServerRegistry[server.URL] = server
	c.ServerRegistryLock.Unlock()
}

func (c *CrawlManager) removeServerFromRegistry(server ServerInfo) {
	c.ServerRegistryLock.Lock()
	delete(c.ServerRegistry, server.URL)
	c.ServerRegistryLock.Unlock()
}

func (c *CrawlManager) GetServerFromRegistry(server ServerInfo) ServerManager {
	c.ServerRegistryLock.RLock()
	defer c.ServerRegistryLock.RUnlock()
	return c.ServerRegistry[server.URL]
}

func (c *CrawlManager) ServerExistsInRegistry(server ServerInfo) bool {
	c.ServerRegistryLock.RLock()
	defer c.ServerRegistryLock.RUnlock()
	_, exists := c.ServerRegistry[server.URL]
	return exists
}

func (c *CrawlManager) Start() {
	for serverInfo := range c.NewServerReceiver {
		// validation
		if c.ServerExistsInRegistry(serverInfo) {
			continue
		}
		logger.Info("Adding server to registry: ", serverInfo)

		// initialize
		serverManager := new(ServerManager)
		serverManager.Type = serverInfo.Type
		serverManager.URL = serverInfo.URL
		serverManager.Timeline = serverInfo.Timeline

		serverManager.dlqueue = make(chan models.DownloadItem, 100)
		serverManager.wg = &sync.WaitGroup{}
		var err error
		serverManager.provider, err = c.getProvider(serverInfo)
		if err != nil {// TODO: ServerInfoとProviderで重複して情報保持しているところを修正する
			logger.Error("Failed to get provider: ", err)
			close(serverManager.dlqueue)
			continue
		}

		// 接続
		if err := serverManager.provider.Connect(); err != nil {
			logger.Error("Failed to connect:", err)
			close(serverManager.dlqueue)
			continue
		}
		defer serverManager.provider.Close()
	
		if err := serverManager.provider.ConnectChannel(); err != nil {
			logger.Error("Failed to connect channel:", err)
			close(serverManager.dlqueue)
			continue
		}

		// 受信
		serverManager.wg.Add(1)
		go func() {
			defer serverManager.wg.Done()
			for {
				if err := serverManager.provider.ReceiveMessages(serverManager.dlqueue); err != nil {
					logger.Errorf("ReceiveMessages error: %v. Reconnecting in 5 seconds...", err)
					time.Sleep(5 * time.Second)
					
					// 再接続
					serverManager.provider.Close()
					if err := serverManager.provider.Connect(); err != nil {
						logger.Errorf("Reconnect failed: %v. Retrying...", err)
						continue
					}
					if err := serverManager.provider.ConnectChannel(); err != nil {
						logger.Errorf("ReconnectChannel failed: %v. Retrying...", err)
						continue
					}
					logger.Info("Reconnected successfully")

					// TODO: ダウンタイムの間のポストをREST APIで取得する処理
					
					continue
				}
				break
			}
		}()

		// ダウンローダーを開始
		if (c.Media) {
			for i := 0; i < c.ParallelDownload; i++ {
				serverManager.wg.Add(1)
				go mediaDownloader.MediaDownloader(serverManager.dlqueue, serverManager.wg, c.DownloadDir)
			}
		}else{
			go func(){
				defer serverManager.wg.Done()
				for item := range serverManager.dlqueue {
					logger.Debug("Discarding item:", item)
				}
			}()
		}

		c.addServerToRegistry(*serverManager)
		logger.Info("Server added to registry: ", serverManager)
	}
}

func (c *CrawlManager) Stop() {
	// TODO: implement
}

func (c *CrawlManager) getProvider(server ServerInfo) (providers.PlatformProvider, error) {
	switch server.Type {
	case "misskey":
		return misskey.NewMisskeyProvider(server.URL, server.Timeline,  filepath.Join(c.DownloadDir, "misskey", server.URL)), nil
	case "nostr":
		return nostr.NewNostrProvider(server.URL, filepath.Join(c.DownloadDir, "nostr", server.URL)), nil
	case "bluesky":
		return bluesky.NewBlueskyProvider(server.URL, filepath.Join(c.DownloadDir, "bluesky", server.URL)), nil
	case "mastodon":
		return mastodon.NewMastodonProvider(server.URL, server.Timeline, filepath.Join(c.DownloadDir, "mastodon", server.URL)), nil
	default:
		logger.Error("Invalid system specified")
		return nil, errors.New("Invalid system specified")
	}
}