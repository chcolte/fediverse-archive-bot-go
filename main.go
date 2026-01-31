package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"os"
	"strings"
	"log"
	//"os/signal"
	//"sync"
	//"syscall"
	//"time"

	"github.com/chcolte/fediverse-archive-bot-go/crawlManager"
	"github.com/chcolte/fediverse-archive-bot-go/logger"
	"github.com/chcolte/fediverse-archive-bot-go/models"

	// for debug 
	// "net/http"
	// _ "net/http/pprof"
)

func main() {

	// for debug
	// go func() {
	// 	logger.Info("pprof server listening on :6060")
	// 	logger.Info(http.ListenAndServe("localhost:6060", nil))
	// }()
	

	system, mode, url, serverListPath, timeline, downloadDir, verbose, media, parallelDownload, scope := readFlags()
	logger.SetVerbose(verbose)

	cm := crawlManager.NewCrawlManager(downloadDir, mode, media, parallelDownload, scope)

	// set target servers
	var serverList []models.Server;
	if serverListPath != "" {
		serverList = readServerList(serverListPath)
	}
		
	if url != "" && system != "" {
		serverList = append(serverList, models.Server{
			Type: system, 
			URL: url,
		})
	}
		
	for _, server := range serverList {
		cm.NewServerReceiver <- server
	}
	
	startMessage(mode, serverList, timeline, downloadDir, media, scope)

	// start crawler
	cm.Start()

	// // シグナルハンドリング
	// quit := make(chan os.Signal, 1)
	// signal.Notify(quit, os.Interrupt, syscall.SIGTERM)

	// <-quit // シグナル待ち
	// logger.Info("Shutting down...")

	// // WebSocket接続を閉じる
	// // TODO: fix: L77で結局Reconnectしてしまう
	// provider.Close()
	// logger.Info("Closed provider connection")

	// // 未処理のURLを保存する
	// savePendingURLs(dlqueue)
	// logger.Info("Saved pending URLs to file")

	// // チャネルを閉じてワーカーを停止させる
	// close(dlqueue)
	// logger.Info("Closed download queue.")

	// // 全てのワーカーが終了するのを待つ
	// // wg.Wait() //これしちゃうと，dlqueueにURLが送られてくるときにパニックを起こすまで止まらない
	// // logger.Info("All workers finished")
}

func startMessage(mode string, serverList []models.Server, timeline string, downloadDir string, media bool, scope string) {
	logger.SetFlags(0)
	logger.Info("---------------------------------------------------")
	logger.Info("Fediverse Archive Bot v0.3.0-beta")
	logger.Info("https://github.com/chcolte/fediverse-archive-bot-go")
	logger.Info("- Mode:               ", mode)
	logger.Info("- Seed Servers:       ", serverList)
	logger.Info("- Target Timeline:    ", timeline)
	logger.Info("- Download media:     ", media)
	logger.Info("- Download Directory: ", downloadDir)
	logger.Info("- Scope:              ", scope)
	logger.Info("---------------------------------------------------")
	logger.SetFlags(log.LstdFlags)
}

func readFlags() (string, string, string, string, string, string, bool, bool, int, string) {
	var (
		s = flag.String("s", "misskey", "target system. (e.g misskey, nostr)")
		m = flag.String("m", "live", "archive mode.(currently live only)")
		u = flag.String("u", "", "server URL. (e.g. https://misskey.io)")
		a = flag.String("a", "", "server URL list. (Max 100 servers) (e.g. ./server_urls.txt)")
		t = flag.String("t", "localTimeline", "(Misskey only) timeline (e.g localTimeline, globalTimeline)")
		d = flag.String("d", "downloads", "download directory")
		v = flag.Bool("V", false, "verbose output")
		M = flag.Bool("media", false, "download media files")
		P = flag.Int("parallel-download", 1, "Number of Media Downloaders")
		S = flag.String("Scope", "server", "scope (e.g. unbounded, server, misskey, mastodon, nostr, bluesky)")
	)
	flag.Parse()
	return *s, *m, *u, *a, *t, *d, *v, *M, *P, *S
}

func readServerList(serverpath string) []models.Server {
	file, err := os.Open(serverpath)
	if err != nil {
		logger.Errorf("Failed to open %s: %v", serverpath, err)
		return nil
	}
	defer file.Close()

	var servers []models.Server
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if(len(strings.Fields(line)) != 2) {
			logger.Errorf("Invalid server line: %s", line)
			continue
		}

		server := models.Server{
			Type: strings.Fields(line)[1],
			URL: strings.Fields(line)[0],
		}
		servers = append(servers, server)
	}
	return servers
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
