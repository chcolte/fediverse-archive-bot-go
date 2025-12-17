package main

import (
	//"log"
	"sync"
)

func MediaDownloader(dlqueue chan string, wg *sync.WaitGroup){
	defer wg.Done()
	// for url := range dlqueue {
	// 	log.Println("dequeued: " + url)
	// 	// 実際のダウンロード処理はここに記述
	// 	// time.Sleep(100 * time.Millisecond) // 処理のシミュレーション
	// }
}