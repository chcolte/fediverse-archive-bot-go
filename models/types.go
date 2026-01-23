package models

import "time"

// DownloadItem represents an item to be downloaded
type DownloadItem struct {
	URL      string
	Datetime time.Time
}

// FilenameURLMapping represents the mapping between filename and URL
type FilenameURLMapping struct {
	Filepath     string `json:"filepath"`
	URL          string `json:"url"`
	Downloadtime string `json:"downloadtime"`
}

// サーバーの基本情報
type ServerInfo struct {
	Type string // bluesky, mastodon, misskey, nostr
	URL string // baseURL
}

// サーバータイムラインの基本情報
type ServerTLInfo struct{
	ServerInfo
	Timeline string // timeline name
}
