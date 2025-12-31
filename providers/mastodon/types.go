package mastodon

import (
	"time"
	"encoding/json"
)

type StreamingMessage struct {
	Stream []string `json:"stream"`
	Event string `json:"event"`
	Payload string `json:"payload"`
}

type Payload struct {
	ID                 string    `json:"id"`
	CreatedAt          time.Time `json:"created_at"`
	InReplyToID        any       `json:"in_reply_to_id"`
	InReplyToAccountID any       `json:"in_reply_to_account_id"`
	Sensitive          bool      `json:"sensitive"`
	SpoilerText        string    `json:"spoiler_text"`
	Visibility         string    `json:"visibility"`
	Language           string    `json:"language"`
	URI                string    `json:"uri"`
	URL                string    `json:"url"`
	RepliesCount       int       `json:"replies_count"`
	ReblogsCount       int       `json:"reblogs_count"`
	FavouritesCount    int       `json:"favourites_count"`
	EditedAt           any       `json:"edited_at"`
	Content            string    `json:"content"`
	Reblog             any       `json:"reblog"`
	Application        Application `json:"application"`
	Account          Account   `json:"account"`
	MediaAttachments []MediaAttachment     `json:"media_attachments"`
	Mentions         []any     `json:"mentions"`
	Tags             []Tag     `json:"tags"`
	Emojis           []CustomEmoji   `json:"emojis"`
	Card             any       `json:"card"`
	Poll             Poll       `json:"poll"`
}

type Application struct {
	ID		string `json:"id"`
	Name    string `json:"name"`
	Website string `json:"website"`
	Scopes  []string `json:"scopes"`
	RedirectURI string `json:"redirect_uri"`
	RedirectURIs []string `json:"redirect_uris"`
	VapidKey string `json:"vapid_key"`
}

type Account struct {
	ID             string    `json:"id"`
	Username       string    `json:"username"`
	Acct           string    `json:"acct"`
	DisplayName    string    `json:"display_name"`
	Locked         bool      `json:"locked"`
	Bot            bool      `json:"bot"`
	Discoverable   bool      `json:"discoverable"`
	Group          bool      `json:"group"`
	CreatedAt      time.Time `json:"created_at"`
	Note           string    `json:"note"`
	URL            string    `json:"url"`
	Avatar         string    `json:"avatar"`
	AvatarStatic   string    `json:"avatar_static"`
	Header         string    `json:"header"`
	HeaderStatic   string    `json:"header_static"`
	FollowersCount int       `json:"followers_count"`
	FollowingCount int       `json:"following_count"`
	StatusesCount  int       `json:"statuses_count"`
	LastStatusAt   string    `json:"last_status_at"`
	Indexable      bool      `json:"indexable"`
	Noindex        bool      `json:"noindex"`
	Emojis         []CustomEmoji   `json:"emojis"`
	Roles          []Role    `json:"roles"`
	Fields         []Field   `json:"fields"`
}

type CustomEmoji struct {
	Shortcode  string `json:"shortcode"`
	URL    string `json:"url"`
	StaticURL string `json:"static_url"`
	VisibleInPicker bool `json:"visible_in_picker"`
	Category string `json:"category"`	
}

type Role struct {
	ID string `json:"id"`
	Name string `json:"name"`
	Color string `json:"color"`
}

type Field struct {
	Name string `json:"name"`
	Value string `json:"value"`
	VerifiedAt time.Time `json:"verified_at"`
}

type MediaAttachment struct {
	ID string `json:"id"`
	Type string `json:"type"`
	URL string `json:"url"`
	PreviewURL string `json:"preview_url"`
	RemoteURL string `json:"remote_url"`
	TextURL string `json:"text_url"`
	Meta json.RawMessage `json:"meta"`
}

type ImageMeta struct {
	Original struct {
		Width  int     `json:"width"`
		Height int     `json:"height"`
		Size   string  `json:"size"`
		Aspect float64 `json:"aspect"`
	} `json:"original"`
	Small struct {
		Width  int     `json:"width"`
		Height int     `json:"height"`
		Size   string  `json:"size"`
		Aspect float64 `json:"aspect"`
	} `json:"small"`
	Focus struct {
		X float64 `json:"x"`
		Y float64 `json:"y"`
	} `json:"focus"`
}

type VideoMeta struct {
	Length        string  `json:"length"`
	Duration      float64 `json:"duration"`
	Fps           int     `json:"fps"`
	Size          string  `json:"size"`
	Width         int     `json:"width"`
	Height        int     `json:"height"`
	Aspect        float64 `json:"aspect"`
	AudioEncode   string  `json:"audio_encode"`
	AudioBitrate  string  `json:"audio_bitrate"`
	AudioChannels string  `json:"audio_channels"`
	Original      struct {
		Width     int     `json:"width"`
		Height    int     `json:"height"`
		FrameRate string  `json:"frame_rate"`
		Duration  float64 `json:"duration"`
		Bitrate   int     `json:"bitrate"`
	} `json:"original"`
	Small struct {
		Width  int     `json:"width"`
		Height int     `json:"height"`
		Size   string  `json:"size"`
		Aspect float64 `json:"aspect"`
	} `json:"small"`
}

type GIFVMeta struct {
	Length   string  `json:"length"`
	Duration float64 `json:"duration"`
	Fps      int     `json:"fps"`
	Size     string  `json:"size"`
	Width    int     `json:"width"`
	Height   int     `json:"height"`
	Aspect   float64 `json:"aspect"`
	Original struct {
		Width     int     `json:"width"`
		Height    int     `json:"height"`
		FrameRate string  `json:"frame_rate"`
		Duration  float64 `json:"duration"`
		Bitrate   int     `json:"bitrate"`
	} `json:"original"`
	Small struct {
		Width  int     `json:"width"`
		Height int     `json:"height"`
		Size   string  `json:"size"`
		Aspect float64 `json:"aspect"`
	} `json:"small"`
}

type AudioMeta struct {
	Length        string  `json:"length"`
	Duration      float64 `json:"duration"`
	AudioEncode   string  `json:"audio_encode"`
	AudioBitrate  string  `json:"audio_bitrate"`
	AudioChannels string  `json:"audio_channels"`
	Original      struct {
		Duration float64 `json:"duration"`
		Bitrate  int     `json:"bitrate"`
	} `json:"original"`
}

type Tag struct {
	ID      string `json:"id"`
	Name    string `json:"name"`
	URL     string `json:"url"`
	History []struct {
		Day      string `json:"day"`
		Uses     string `json:"uses"`
		Accounts string `json:"accounts"`
	} `json:"history"`
	Following bool `json:"following"`
	Featuring bool `json:"featuring"`
}

type Poll struct {
	ID          string    `json:"id"`
	ExpiresAt   time.Time `json:"expires_at"`
	Expired     bool      `json:"expired"`
	Multiple    bool      `json:"multiple"`
	VotesCount  int       `json:"votes_count"`
	VotersCount any       `json:"voters_count"`
	Voted       bool      `json:"voted"`
	OwnVotes    []int     `json:"own_votes"`
	Options     []struct {
		Title      string `json:"title"`
		VotesCount int    `json:"votes_count"`
	} `json:"options"`
	Emojis []CustomEmoji `json:"emojis"`
}