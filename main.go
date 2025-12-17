package main

import (
	"log"
	"flag"
	"strings"

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
	
	// Receive Message Logic
	var recvMsg string
	for {
		recvErr := websocket.Message.Receive(ws, &recvMsg)
		if recvErr != nil {
			log.Fatal(recvErr)
			break
		}
		go channelon(recvMsg) // message processing
	}
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

// when received
func channelon(rawNote string) {
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
		log.Println(url)
	}
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
