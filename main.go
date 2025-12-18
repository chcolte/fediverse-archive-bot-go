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

	"github.com/chcolte/fediverse-archive-bot-go/models"
	"github.com/chcolte/fediverse-archive-bot-go/providers/misskey"
)

func main() {
	mode, url, timeline := readFlags()
	startMessage(mode, url, timeline)

	// ダウンロードキューとディレクトリ
	dlqueue := make(chan models.DownloadItem, 100)
	downloadDir := "downloads"
	loadPendingURLs(dlqueue)

	var wg sync.WaitGroup

	// Misskey Provider
	misskeyProvider := misskey.NewMisskeyProvider(url, timeline)
	if err := misskeyProvider.Connect(); err != nil {
		log.Fatal("Failed to connect:", err)
	}
	defer misskeyProvider.Close()

	if err := misskeyProvider.ConnectChannel(); err != nil {
		log.Fatal("Failed to connect channel:", err)
	}

	// メッセージ受信を開始
	wg.Add(1)
	go func() {
		defer wg.Done()
		misskeyProvider.ReceiveMessages(dlqueue)
	}()

	// ダウンローダーを開始
	wg.Add(1)
	go MediaDownloader(dlqueue, &wg, downloadDir)

	// シグナルハンドリング
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	<-quit // シグナル待ち
	log.Println("Shutting down...")

	// 未処理のURLを保存する
	savePendingURLs(dlqueue)
	log.Println("Saved pending URLs to file")

	// チャネルを閉じてワーカーを停止させる

	close(dlqueue)
	log.Println("Closed download queue")
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

func readFlags() (string, string, string) {
	var (
		m = flag.String("m", "live", "archive mode.(live or past)")
		u = flag.String("u", "", "server URL. (e.g. https://misskey.io)")
		t = flag.String("t", "localTimeline", "timeline (e.g localTimeline, globalTimeline)")
	)
	flag.Parse()
	return *m, *u, *t
}

func loadPendingURLs(dlqueue chan models.DownloadItem) {
	file, err := os.Open("pending_urls.txt")
	if err != nil {
		if !os.IsNotExist(err) {
			log.Printf("Failed to open pending_urls.txt: %v", err)
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
				log.Printf("Failed to parse pending URL line: %v", err)
				continue
			}
			dlqueue <- item
			count++
		}
	}
	log.Printf("Loaded %d pending URLs", count)

	os.Remove("pending_urls.txt")
}

func savePendingURLs(dlqueue chan models.DownloadItem) {
	f, err := os.Create("pending_urls.txt")
	if err != nil {
		log.Printf("Failed to create pending_urls.txt: %v", err)
		return
	}
	defer f.Close()

	count := 0
	for {
		select {
		case item := <-dlqueue:
			line, err := json.Marshal(item)
			if err != nil {
				log.Printf("Failed to marshal item: %v", err)
				continue
			}
			f.WriteString(string(line) + "\n")
			count++
		default:
			log.Printf("Saved %d URLs to pending_urls.txt", count)
			return
		}
	}
}
