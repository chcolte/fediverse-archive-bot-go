package crawlManager

import (
	"sync"
	"time"
	"errors"
	"path/filepath"
	"os"
	"fmt"

	"github.com/chcolte/fediverse-archive-bot-go/providers"
	"github.com/chcolte/fediverse-archive-bot-go/models"
	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"github.com/chcolte/fediverse-archive-bot-go/providers/bluesky"
	"github.com/chcolte/fediverse-archive-bot-go/providers/mastodon"
	"github.com/chcolte/fediverse-archive-bot-go/providers/misskey"
	"github.com/chcolte/fediverse-archive-bot-go/providers/nostr"
	"github.com/chcolte/fediverse-archive-bot-go/media-downloader"
)

// todo: 名前CrawlerManagerのほうがいいか？
// 実際に稼働中のサーバーの管理情報　（ServerInfoの上位存在，正常に稼働開始しているサーバーならば追加要素も埋められるはず）
type ServerManager struct {
	models.ServerTLInfo

	provider providers.PlatformProvider // Provider instance <-Goだと必須フィールドの強制はできないらしい
	wg *sync.WaitGroup // WaitGroup
}

// 任意のTLでの新規サーバー検索役のクローラーの管理情報
type ExplorerManager struct {
	ServerManager
	serverqueue chan models.ServerInfo // Server queue
}

// 任意のTLでのデータ保存役のクローラーの管理情報
type ArchiverManager struct {
	ServerManager
	dlqueue chan models.DownloadItem // Download queue
}

/*
[オブジェクトの階層]

- TLを保存するRoleの接続インスタンス or 新規サーバーを探索するRoleの接続インスタンス -> ArchiverManager, ExplorerManager
↑
- 監視対象のストリームへの接続インスタンス -> ServerManager
↑
- うち，監視対象の(ポスト)ストリーム情報 -> ServerTLInfo
↑
- 接続サーバーはどこか -> ServerInfo
*/


type CrawlManager struct {
	NewServerReceiver chan models.ServerInfo // 新規サーバー受付チャネル
	ServerRegistry map[string]ServerManager // ServerRegistry
	ServerRegistryLock sync.RWMutex // ServerRegistryを操作する際の排他ロック

	DownloadDir string
	Mode string // live, past
	Media bool
	ParallelDownload int
}

func NewCrawlManager(downloadDir string, mode string, media bool, parallelDownload int) *CrawlManager {
	return &CrawlManager{
		NewServerReceiver: make(chan models.ServerInfo, 10),
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

func (c *CrawlManager) removeServerFromRegistry(server models.ServerTLInfo) {
	c.ServerRegistryLock.Lock()
	delete(c.ServerRegistry, server.URL)
	c.ServerRegistryLock.Unlock()
}

func (c *CrawlManager) GetServerFromRegistry(server models.ServerTLInfo) ServerManager {
	c.ServerRegistryLock.RLock()
	defer c.ServerRegistryLock.RUnlock()
	return c.ServerRegistry[server.URL]
}

func (c *CrawlManager) ServerExistsInRegistry(server models.ServerTLInfo) bool {
	c.ServerRegistryLock.RLock()
	defer c.ServerRegistryLock.RUnlock()
	_, exists := c.ServerRegistry[server.URL]
	return exists
}

func (c *CrawlManager) Start() {
	for serverInfo := range c.NewServerReceiver {
		logger.Debug("Received new server: ", serverInfo)
		
		// validation
		var serverTLInfo models.ServerTLInfo
		serverTLInfo.ServerInfo = serverInfo
		serverTLInfo.Timeline = "localTimeline" // FIXME: for misskey only now
		if c.ServerExistsInRegistry(serverTLInfo) {
			continue
		}
		logger.Debug("Adding server to registry: ", serverInfo)

		// initialize
		var serverManager ServerManager
		serverManager.ServerTLInfo = serverTLInfo
		serverManager.wg = &sync.WaitGroup{}
		var err error
		serverManager.provider, err = c.getProvider(serverTLInfo)
		if err != nil {// TODO: ServerInfoとProviderで重複して情報保持しているところを修正する
			logger.Error("Failed to get provider: ", err)
			continue
		}

		// add server to registry
		c.addServerToRegistry(serverManager)
		// update server list
		ServerListPath := filepath.Join(c.DownloadDir, serverInfo.Type, "server_list.txt")
		c.AppendToFile(serverInfo.URL, ServerListPath)

		// start archiver
		archiveManager := ArchiverManager{
			ServerManager: serverManager,
			dlqueue: make(chan models.DownloadItem, 100),
		}
		c.startTLArchiver(archiveManager)

		// ------------explorer--------------
		explorer_serverTLInfo := models.ServerTLInfo{
			ServerInfo: serverInfo,
			Timeline: "globalTimeline", // FIXME: for misskey only now
		}
		
		explorer_serverManager := ServerManager{
			ServerTLInfo: explorer_serverTLInfo,
			wg: &sync.WaitGroup{},
		}
		explorer_serverManager.provider, err = c.getProvider(explorer_serverTLInfo)
		if err != nil {// TODO: ServerInfoとProviderで重複して情報保持しているところを修正する
			logger.Error("Failed to get provider: ", err)
			continue
		}

		explorerManager := ExplorerManager{
			ServerManager: explorer_serverManager,
			serverqueue: c.NewServerReceiver,
		}
		// start
		c.startTLExplorer(explorerManager)


		logger.Info("Server added to registry: ", serverManager)
	}
}

func (c *CrawlManager) Stop() {
	// TODO: implement
}

func (c *CrawlManager) getProvider(server models.ServerTLInfo) (providers.PlatformProvider, error) {
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
		return nil, errors.New("unsupported system specified: " + server.Type)
	}
}

// サーバーのTLの保存を開始する
func (c *CrawlManager) startTLArchiver(serverManager ArchiverManager) {
	// 接続
	if err := serverManager.provider.Connect(); err != nil {
		logger.Error("Failed to connect:", err)
		close(serverManager.dlqueue)
		return
	}

	if err := serverManager.provider.ConnectChannel(); err != nil {
		logger.Error("Failed to connect channel:", err)
		close(serverManager.dlqueue)
		return
	}

	// 受信
	serverManager.wg.Add(1)
	go func() {
		defer serverManager.wg.Done()
		defer serverManager.provider.Close()
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

			}else{
				break
			}
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

}

// 上と重複するが一旦放置
func (c *CrawlManager) startTLExplorer(serverManager ExplorerManager) {
	// 接続
	if err := serverManager.provider.Connect(); err != nil {
		logger.Error("Failed to connect:", err)
		return
	}

	if err := serverManager.provider.ConnectChannel(); err != nil {
		logger.Error("Failed to connect channel:", err)
		return
	}

	// 受信
	serverManager.wg.Add(1)
	go func() {
		defer serverManager.wg.Done()
		defer serverManager.provider.Close()
		for {
			if err := serverManager.provider.CrawlNewServer(serverManager.serverqueue); err != nil {
				logger.Errorf("CrawlNewServer error: %v. Reconnecting in 5 seconds...", err)
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

			}else{
				break
			}
		}
	}()

}

func (c *CrawlManager) AppendToFile(text string, filepath string) {
    file, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
    if err != nil {
        logger.Errorf("Failed to open file: %v", err)
    }
    defer file.Close()
    fmt.Fprintln(file, text) //書き込み
}