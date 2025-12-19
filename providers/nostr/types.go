package nostr

import (
	//"encoding/json"
	//"fmt"
)

// 一応，EVENTのみに対応することを想定
type NostrMessage struct {
	Type           string      // "EVENT", "NOTICE", "EOSE", "OK", etc.
	SubscriptionID string      // Subscription ID for "EVENT"
	Event          *NostrEvent // Event data for "EVENT"
	Message        string      // Message content for "NOTICE"
}

// EVENT時のJSONオブジェクト
type NostrEvent struct {
	ID        string     `json:"id"`
	PubKey    string     `json:"pubkey"`
	CreatedAt int64      `json:"created_at"`
	Kind      int        `json:"kind"`
	Tags      [][]string `json:"tags"`
	Content   string     `json:"content"`
	Sig       string     `json:"sig"`
}

