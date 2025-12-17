package main

import (
	"log"

	"golang.org/x/net/websocket"
	"github.com/google/uuid"
)

// WebSocket Client Sample
func main() {
	log.SetFlags(log.Lmicroseconds)

	// WebSocket Dial
	ws, dialErr := websocket.Dial("wss://misskey.io/streaming", "", "https://misskey.io")
	if dialErr != nil {
		log.Fatal(dialErr)
	}
	defer ws.Close()

	connectChannel(ws, "globalTimeline")
	
	// Receive Message Ligic
	var recvMsg string
	for {
		recvErr := websocket.Message.Receive(ws, &recvMsg)
		if recvErr != nil {
			log.Fatal(recvErr)
			break
		}
		channelon(recvMsg)
	}
}

// when received
func channelon(rawNote string) {
	log.Println("Received")
	streamingMessage, err := ParseStreamingMessage(rawNote)
	if err != nil {
		log.Printf("Error parsing note: %v", err)
		return
	}
	//log.Println(rawNote)
	log.Println(streamingMessage.Body.Body.UserID)
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
}

// Send Message Ligic
func sendRestMsg(ws *websocket.Conn, msg string) {
	sendErr := websocket.Message.Send(ws, msg)
	if sendErr != nil {
		log.Fatal(sendErr)
	}
	log.Println("Send : " + msg + ", to Server")
}
