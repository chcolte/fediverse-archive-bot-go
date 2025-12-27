package bluesky

// Firehoseメタデータ (JSONL保存用)
type FirehoseMetadata struct {
	Seq        uint64   `json:"seq"`
	Time       string   `json:"time"`
	Type       string   `json:"type"`
	Repo       string   `json:"repo"`
	Rev        string   `json:"rev,omitempty"`
	Ops        []OpInfo `json:"ops,omitempty"`
	CBORFile   string   `json:"cbor_file"`
	ReceivedAt string   `json:"received_at"`
}

// 操作情報
type OpInfo struct {
	Action string `json:"action"`
	Path   string `json:"path"`
	CID    string `json:"cid,omitempty"`
}

// Commit ペイロード構造体
type CommitPayload struct {
	Repo   string        `cbor:"repo"`
	Rev    string        `cbor:"rev"`
	Seq    uint64        `cbor:"seq"`
	Since  string        `cbor:"since"`
	Time   string        `cbor:"time"`
	TooBig bool          `cbor:"tooBig"`
	Ops    []CommitOp    `cbor:"ops"`
	Blocks []byte        `cbor:"blocks"`
}

// Commit 操作
type CommitOp struct {
	Action string      `cbor:"action"`
	Path   string      `cbor:"path"`
	CID    interface{} `cbor:"cid"`
}
