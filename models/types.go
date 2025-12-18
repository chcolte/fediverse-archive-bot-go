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
