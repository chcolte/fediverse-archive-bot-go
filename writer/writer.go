// writer/writer.go
package writer

import (
    "os"
    "path/filepath"
    "time"

    "github.com/chcolte/fediverse-archive-bot-go/logger"
    "github.com/chcolte/fediverse-archive-bot-go/models"
    "github.com/chcolte/fediverse-archive-bot-go/utils"
)

type Writer struct {
    BaseDir        string // e.g. "downloads/misskey/misskey.io"
    Timeline       string // e.g. "local"
    CrawlSessionID string
}

// goroutineとして起動されることを想定。channelが閉じられると終了
func (w *Writer) Run(queue <-chan models.RawMessage) {
    for msg := range queue {
        if err := w.writeMessage(msg); err != nil {
            logger.Errorf("Failed to write message: %v", err)
        }
    }
}

func (w *Writer) writeMessage(msg models.RawMessage) error {
    dateStr := msg.CreatedAt.Format("2006-01-02")
    dailyDir := filepath.Join(w.BaseDir, dateStr)

	switch msg.DataType {
	case "cbor":
        return w.writeCBOR(msg, dailyDir, dateStr) // fix?: msgからdatesdrもdailysdrも導出できるのに，引数として取らせているのはなんかへんだよね
	
	case "json":
        return w.writeJSON(msg, dailyDir, dateStr)
	}
	return nil
}

func (w *Writer) writeJSON(msg models.RawMessage, dailyDir, dateStr string) error {
    suffix := string(time.Now().Format("2006-01-02"))
    /* msg.receivedAtをつかうべきか，time.now()をつかうべきか
        url: https://github.com/chcolte/fediverse-archive-bot-go/issues/8
        receiveしてから，書き込むまでにタイムラグが一応存在はするから，どこかでスタックした場合，外部からファイルmvしたときの衝突は起こりうる可能性はある
        ならば，受信時刻ではなく，保存時刻でファイルを分割するべきなような気がする
        そもそも，ファイルの分割が主目的であって，1時間毎に保存先を変えるとか，100MBを超える毎に保存先を変えるとか，色々選択肢を今後作ることになるんではないか。保存時刻ベースで一旦処理する
    */
    
    savePath := filepath.Join(dailyDir, dateStr+"_"+w.Timeline+"_"+suffix+".jsonl")
    return utils.SaveMessage(msg, w.CrawlSessionID, savePath)
}

func (w *Writer) writeCBOR(msg models.RawMessage, dailyDir, dateStr string) error {
    cborDir := filepath.Join(dailyDir, "cbor")
    if err := os.MkdirAll(cborDir, 0755); err != nil {
        return err
    }
    filename := msg.Metadata["filename"]
    if err := os.WriteFile(filepath.Join(cborDir, filename), msg.Data, 0644); err != nil {
        return err
    }
    // メタデータJSON
    if jsonData, ok := msg.Metadata["metadata_json"]; ok {
        metaMsg := models.RawMessage{
            Data:       []byte(jsonData),
            CreatedAt:  msg.CreatedAt,
            ReceivedAt: msg.ReceivedAt,
            DataType:   "json",
            Metadata:   msg.Metadata,
        }
        w.writeJSON(metaMsg, dailyDir, dateStr)
    }
    return nil
}