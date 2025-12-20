package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"github.com/chcolte/fediverse-archive-bot-go/models"
	"github.com/chcolte/fediverse-archive-bot-go/providers"
	"github.com/chcolte/fediverse-archive-bot-go/providers/misskey"
	"github.com/chcolte/fediverse-archive-bot-go/providers/nostr"

	// for debug 
	// "net/http"
	// _ "net/http/pprof"
)

func main() {

	// for debug
	// go func() {
	// 	log.Println("pprof server listening on :6060")
	// 	log.Println(http.ListenAndServe("localhost:6060", nil))
	// }()
	

	system, mode, url, timeline, downloadDir, verbose, media := readFlags()
	startMessage(system, mode, url, timeline, downloadDir, media)

	logger.SetVerbose(verbose)

	// ダウンロードキューを準備
	dlqueue := make(chan models.DownloadItem, 100)
	//loadPendingURLs(dlqueue) // なぜか動かすと，スタックする

	var wg sync.WaitGroup

	// システムの選択
	var provider providers.PlatformProvider
	switch system {
		case "misskey":
			provider = misskey.NewMisskeyProvider(url, timeline, downloadDir)

		case "nostr":
			provider = nostr.NewNostrProvider(url, downloadDir)

		default:
			logger.Error("Invalid system specified")
			os.Exit(1)
	}

	// WebSocket接続
	if err := provider.Connect(); err != nil {
		logger.Error("Failed to connect:", err)
	}
	defer provider.Close()
	
	// チャネル接続
	if err := provider.ConnectChannel(); err != nil {
		logger.Error("Failed to connect channel:", err)
	}

	// メッセージ受信を開始
	wg.Add(1)
	go func() {
		defer wg.Done()
		provider.ReceiveMessages(dlqueue)
	}()
	
	// ダウンローダーを開始
	wg.Add(1)
	if (media) {
		go MediaDownloader(dlqueue, &wg, downloadDir)
	}else{
		go func(){
			defer wg.Done()
			for item := range dlqueue {
				logger.Debug("Discarding item:", item)
			}
		}()
	}

	// シグナルハンドリング
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	<-quit // シグナル待ち
	logger.Info("Shutting down...")
	

	// 未処理のURLを保存する
	savePendingURLs(dlqueue)
	logger.Info("Saved pending URLs to file")

	// チャネルを閉じてワーカーを停止させる
	close(dlqueue)
	logger.Info("Closed download queue")
}

func startMessage(system string, mode string, url string, timeline string, downloadDir string, media bool) {
	logger.SetFlags(0)
	logger.Info("---------------------------------------------------")
	logger.Info("Fediverse Archive Bot v1.0.0")
	logger.Info("https://github.com/chcolte/fediverse-archive-bot-go")
	logger.Info("- Target System:      ", system)
	logger.Info("- Mode:               ", mode)
	logger.Info("- URL:                ", url)
	logger.Info("- Timeline:           ", timeline)
	logger.Info("- Download media:     ", media)
	logger.Info("- Download Directory: ", downloadDir)
	logger.Info("---------------------------------------------------")
}

func readFlags() (string, string, string, string, string, bool, bool) {
	var (
		s = flag.String("s", "misskey", "target system. (e.g misskey, nostr)")
		m = flag.String("m", "live", "archive mode.(currently live only)")
		u = flag.String("u", "", "server URL. (e.g. https://misskey.io)")
		t = flag.String("t", "localTimeline", "(Misskey only) timeline (e.g localTimeline, globalTimeline)")
		d = flag.String("d", "downloads", "download directory")
		v = flag.Bool("V", false, "verbose output")
		M = flag.Bool("media", false, "download media files")
	)
	flag.Parse()
	return *s, *m, *u, *t, *d, *v, *M
}

func loadPendingURLs(dlqueue chan models.DownloadItem) {
	file, err := os.Open("pending_urls.txt")
	if err != nil {
		if !os.IsNotExist(err) {
			logger.Errorf("Failed to open pending_urls.txt: %v", err)
		}
		return
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	count := 0
	for scanner.Scan() {
		line := scanner.Text()
		if line != "" {
			var item models.DownloadItem
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				logger.Errorf("Failed to parse pending URL line: %v", err)
				continue
			}
			dlqueue <- item
			count++
		}
	}
	logger.Infof("Loaded %d pending URLs", count)

	os.Remove("pending_urls.txt")
}

func savePendingURLs(dlqueue chan models.DownloadItem) {
	f, err := os.Create("pending_urls.txt")
	if err != nil {
		logger.Errorf("Failed to create pending_urls.txt: %v", err)
		return
	}
	defer f.Close()

	count := 0
	for {
		select {
		case item := <-dlqueue:
			line, err := json.Marshal(item)
			if err != nil {
				logger.Errorf("Failed to marshal item: %v", err)
				continue
			}
			f.WriteString(string(line) + "\n")
			count++
		default:
			logger.Infof("Saved %d URLs to pending_urls.txt", count)
			return
		}
	}
}
