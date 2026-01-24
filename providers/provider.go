package providers

import "github.com/chcolte/fediverse-archive-bot-go/models"

// 各プラットフォーム（Misskey, Mastodon等）が実装すべきインターフェース
type PlatformProvider interface {
	Connect() error
	ConnectChannel() error
	ReceiveMessages(output chan<- models.DownloadItem) error
	CrawlNewServer(server chan <- models.Server) error
	Close() error
}
