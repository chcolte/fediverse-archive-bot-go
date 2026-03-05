// writer/writer.go
package writer

import (
    "os"
    "path/filepath"
	"fmt"
    "github.com/chcolte/fediverse-archive-bot-go/models"
    "github.com/chcolte/fediverse-archive-bot-go/logger"
)

type Writer struct {
    BaseDir    string // e.g. "downloads/misskey/misskey.io"
    Timeline   string // e.g. "local"
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
        return w.writeCBOR(msg, dailyDir, dateStr)
	
	case "json":
        return w.writeJSON(msg, dailyDir, dateStr)
	}
	return nil
}

func (w *Writer) writeJSON(msg models.RawMessage, dailyDir, dateStr string) error {
    if err := os.MkdirAll(dailyDir, 0755); err != nil {
        return err
    }
    savePath := filepath.Join(dailyDir, dateStr+"_"+w.Timeline+".jsonl")
    return appendToFile(string(msg.Data), savePath)
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
        jsonlPath := filepath.Join(dailyDir, dateStr+".jsonl")
        return appendToFile(jsonData, jsonlPath)
    }
    return nil
}

func appendToFile(text string, filePath string) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		logger.Errorf("Failed to create directory: %v", err)
		return err
	}

	file, err := os.OpenFile(filePath, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		logger.Errorf("Failed to open file: %v", err)
		return err
	}
	defer file.Close()
	fmt.Fprintln(file, text)
	return nil 
}