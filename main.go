package main

import (
	"log"
	"flag"
	"strings"
	"sync"
	"os"
	"os/signal"
	"syscall"
	"bufio"
	"encoding/json"

	"golang.org/x/net/websocket"
	"github.com/google/uuid"
)

// WebSocket Client Sample
func main() {
	mode, url, timeline := readFlags()
	ws_url, http_url := urlAdjust(url)
	startMessage(mode, url, timeline)
	
	// WebSocket Dial
	ws, dialErr := websocket.Dial(ws_url, "", http_url)
	if dialErr != nil {
		log.Fatal(dialErr)
	}else {
		log.Println("Connected to", ws_url)
	}
	defer ws.Close()

	// connect to timeline
	connectChannel(ws, timeline)
	
	// Processing
	var wg = sync.WaitGroup{}
	dlqueue := make(chan DownloadItem, 100)
	downloadDir := "downloads"
	loadPendingURLs(dlqueue)

	wg.Add(1)
	go MessageReceiver(ws, dlqueue, &wg)
	
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

func startMessage(mode string, url string, timeline string){
	log.SetFlags(0)
	log.Println("---------------------------------------------------")
	log.Println("Fediverse Archive Bot v1.0.0");
	log.Println("https://github.com/chcolte/fediverse-archive-bot-go");
	log.Println("- Mode: " + mode)
	log.Println("- URL: " + url)
	log.Println("- Timeline: " + timeline)
	log.Println("---------------------------------------------------")
}

func readFlags()(string, string, string){
	var (
		m = flag.String("m", "live", "archive mode.(live or past)")
		u = flag.String("u", "", "server URL. (e.g. https://misskey.io)")
		t = flag.String("t", "localTimeline", "timeline (e.g localTimeline, globalTimeline)")
	)
	flag.Parse()
	return *m, *u, *t
}

func urlAdjust(url string)(ws string, http string){
	if(strings.HasPrefix(url, "https://")){
		return strings.Replace(url, "https://", "wss://", -1), url
	}
	if(strings.HasPrefix(url, "http://")){
		return strings.Replace(url, "http://", "ws://", -1), url
	}
	if(strings.HasPrefix(url, "wss://")){
		return url, strings.Replace(url, "wss://", "https://", -1)
	}
	if(strings.HasPrefix(url, "ws://")){
		return strings.Replace(url, "ws://", "http://", -1), url
	}
	return "wss://"+url, "https://"+url
}

// Connect Channel
func connectChannel(ws *websocket.Conn, channel string) {
	uuidV1, err := uuid.NewRandom()
		if err != nil {
			log.Fatal(err)
		}
	
	sendRestMsg(ws, `{
		"type": "connect",
		"body": {
			"channel": "`+channel+`",
			"id": "`+uuidV1.String()+`"
		}
	}`)

	log.Println("Connected to", channel)
}

// Send Message Ligic
func sendRestMsg(ws *websocket.Conn, msg string) {
	sendErr := websocket.Message.Send(ws, msg)
	if sendErr != nil {
		log.Fatal(sendErr)
	}
}

func loadPendingURLs(dlqueue chan DownloadItem) {
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
			// Parse JSON line to get URL and datetime
			var item DownloadItem
			if err := json.Unmarshal([]byte(line), &item); err != nil {
				log.Printf("Failed to parse pending URL line: %v", err)
				continue
			}
			dlqueue <- item
			count++
		}
	}
	log.Printf("Loaded %d pending URLs", count)
	
	// 読み込み終わったらファイルを削除（または空にする）
	os.Remove("pending_urls.txt")
}

func savePendingURLs(dlqueue chan DownloadItem) {
	// チャネルに残っているものを取り出してファイルに保存
	f, err := os.Create("pending_urls.txt")
	if err != nil {
		log.Printf("Failed to create pending_urls.txt: %v", err)
		return
	}
	defer f.Close()

	count := 0
	// チャネルが空になるまで取り出す
	// 注意: ワーカーも動いているので競合するが、ここで取り出せた分だけ保存する
	for {
		select {
		case item := <-dlqueue:
			// Save as JSON to preserve both URL and datetime
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
