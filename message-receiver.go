package main

import (
	"log"
	"os"
	"sync"

	"golang.org/x/net/websocket"
)

var overflowMu sync.Mutex

// websocketからのデータを受信する
func MessageReceiver(ws *websocket.Conn, dlqueue chan string, wg *sync.WaitGroup) {
	log.Println("MessageReceiver started")
	var recvMsg string
	for {
		recvErr := websocket.Message.Receive(ws, &recvMsg)
		if recvErr != nil {
			log.Fatal(recvErr)
			break
		}
		go messageProcesser(recvMsg, dlqueue)
	}
	defer wg.Done()
}

// 受信データを処理する
func messageProcesser(rawNote string, dlqueue chan string) {
	streamingMessage, err := ParseStreamingMessage(rawNote)
	if err != nil {
		log.Printf("Error parsing note: %v", err)
		return
	}

	note, err := ExtractNote(streamingMessage)
	if err != nil {
		log.Printf("Error extracting note: %v", err)
		return
	}

	for _, url := range SafeExtractURL(note) {
		select {
		case dlqueue <- url:
			log.Println("queued: ", url)
		default:
			log.Printf("Queue full, saving to overflow file: %s", url)
			saveToOverflow(url)
		}
	}
}

func saveToOverflow(url string) {
	overflowMu.Lock()
	defer overflowMu.Unlock()

	f, err := os.OpenFile("overflow_urls.txt", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Printf("Failed to open overflow file: %v", err)
		return
	}
	defer f.Close()

	if _, err := f.WriteString(url + "\n"); err != nil {
		log.Printf("Failed to write to overflow file: %v", err)
	}
}
