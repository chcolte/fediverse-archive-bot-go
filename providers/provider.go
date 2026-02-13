package providers

import (
	"github.com/chcolte/fediverse-archive-bot-go/models"
	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"time"
	"os"
	"fmt"
	"path"
	"strings"
)

// 各プラットフォーム（Misskey, Mastodon等）が実装すべきインターフェース
type PlatformProvider interface {
	Connect() (string, error)
	ConnectChannel() ([]byte, error)
	ReceiveMessages(output chan<- models.DownloadItem) error
	CrawlNewServer(server chan <- models.Server) error
	Close() error
}

// 受信した生JSONをJson line形式で書き出す
func AppendToFile(text string, filepath string) {
	
	ext := path.Ext(filepath)
	base := strings.TrimSuffix(filepath, ext)

	filepath = base + "_" + time.Now().Format("20060102") + ext
    file, err := os.OpenFile(filepath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0665)
    if err != nil {
        logger.Errorf("Failed to open file: %v", err)
    }
    defer file.Close()
    fmt.Fprintln(file, text) //書き込み
}




