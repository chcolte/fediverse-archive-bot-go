package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"

	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"github.com/chcolte/fediverse-archive-bot-go/models"
	//"github.com/chcolte/fediverse-archive-bot-go/providers/misskey"
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
	

	mode, url, timeline, verbose := readFlags()
	startMessage(mode, url, timeline)

	logger.SetVerbose(verbose)

	// ダウンロードキューとディレクトリ
	dlqueue := make(chan models.DownloadItem, 100)
	downloadDir := "downloads"
	loadPendingURLs(dlqueue)

	var wg sync.WaitGroup

	// // Misskey Provider
	// misskeyProvider := misskey.NewMisskeyProvider(url, timeline, downloadDir)
	// if err := misskeyProvider.Connect(); err != nil {
	// 	logger.Error("Failed to connect:", err)
	// }
	// defer misskeyProvider.Close()

	// if err := misskeyProvider.ConnectChannel(); err != nil {
	// 	logger.Error("Failed to connect channel:", err)
	// }

	// // メッセージ受信を開始
	// wg.Add(1)
	// go func() {
	// 	defer wg.Done()
	// 	misskeyProvider.ReceiveMessages(dlqueue)
	// }()

	// Nostr Provider
	nostrProvider := nostr.NewNostrProvider(url, downloadDir)
	if err := nostrProvider.Connect(); err != nil {
		logger.Error("Failed to connect:", err)
	}
	defer nostrProvider.Close()

	if err := nostrProvider.ConnectChannel(); err != nil {
		logger.Error("Failed to connect channel:", err)
	}

	// メッセージ受信を開始
	wg.Add(1)
	go func() {
		defer wg.Done()
		nostrProvider.ReceiveMessages(dlqueue)
	}()




	// ダウンローダーを開始
	wg.Add(1)
	go MediaDownloader(dlqueue, &wg, downloadDir)

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

func startMessage(mode string, url string, timeline string) {
	log.SetFlags(0)
	log.Println("---------------------------------------------------")
	log.Println("Fediverse Archive Bot v1.0.0")
	log.Println("https://github.com/chcolte/fediverse-archive-bot-go")
	log.Println("- Mode:", mode)
	log.Println("- URL:", url)
	log.Println("- Timeline:", timeline)
	log.Println("---------------------------------------------------")
}

func readFlags() (string, string, string, bool) {
	var (
		m = flag.String("m", "live", "archive mode.(currently live only)")
		u = flag.String("u", "", "server URL. (e.g. https://misskey.io)")
		t = flag.String("t", "localTimeline", "(Misskey only) timeline (e.g localTimeline, globalTimeline)")
		v = flag.Bool("V", false, "verbose output")
	)
	flag.Parse()
	return *m, *u, *t, *v
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
