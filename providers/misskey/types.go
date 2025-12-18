package misskey

import (
	"time"
)

type AvatarDecoration struct {
	ID      string  `json:"id"`
	Angle   float64 `json:"angle"`
	FlipH   bool    `json:"flipH"`
	URL     string  `json:"url"`
	OffsetX float64 `json:"offsetX"`
	OffsetY float64 `json:"offsetY"`
}

type Instance struct {
	Name            string `json:"name"`
	SoftwareName    string `json:"softwareName"`
	SoftwareVersion string `json:"softwareVersion"`
	IconURL         string `json:"iconUrl"`
	FaviconURL      string `json:"faviconUrl"`
	ThemeColor      string `json:"themeColor"`
}

type BadgeRole struct {
	Name         string `json:"name"`
	IconURL      string `json:"iconUrl"`
	DisplayOrder int    `json:"displayOrder"`
	Behavior     string `json:"behavior"`
}

type User struct {
	ID                           string             `json:"id"`
	Name                         string             `json:"name"`
	Username                     string             `json:"username"`
	Host                         string             `json:"host"`
	AvatarURL                    string             `json:"avatarUrl"`
	AvatarBlurhash               string             `json:"avatarBlurhash"`
	AvatarDecorations            []AvatarDecoration `json:"avatarDecorations"`
	IsBot                        bool               `json:"isBot"`
	IsCat                        bool               `json:"isCat"`
	RequireSigninToViewContents  bool               `json:"requireSigninToViewContents"`
	MakeNotesFollowersOnlyBefore int                `json:"makeNotesFollowersOnlyBefore"`
	MakeNotesHiddenBefore        int                `json:"makeNotesHiddenBefore"`
	Instance                     Instance           `json:"instance"`
	Emojis                       map[string]string  `json:"emojis"`
	OnlineStatus                 string             `json:"onlineStatus"`
	BadgeRoles                   []BadgeRole        `json:"badgeRoles"`
}

type DriveFolder struct {
	ID           string       `json:"id"`
	CreatedAt    time.Time    `json:"createdAt"`
	Name         string       `json:"name"`
	ParentID     string       `json:"parentId"`
	FoldersCount int          `json:"foldersCount"`
	FilesCount   int          `json:"filesCount"`
	Parent       *DriveFolder `json:"parent"`
}

type DriveFile struct {
	ID                     string    `json:"id"`
	CreatedAt              time.Time `json:"createdAt"`
	Name                   string    `json:"name"`
	Type                   string    `json:"type"`
	Md5                    string    `json:"md5"`
	Size                   int       `json:"size"`
	IsSensitive            bool      `json:"isSensitive"`
	IsSensitiveByModerator bool      `json:"isSensitiveByModerator"`
	Blurhash               string    `json:"blurhash"`
	Properties             struct {
		Width       int    `json:"width"`
		Height      int    `json:"height"`
		Orientation int    `json:"orientation"`
		AvgColor    string `json:"avgColor"`
	} `json:"properties"`
	URL          string      `json:"url"`
	ThumbnailURL string      `json:"thumbnailUrl"`
	Comment      string      `json:"comment"`
	FolderID     string      `json:"folderId"`
	Folder       DriveFolder `json:"folder"`
	UserID       string      `json:"userId"`
	User         User        `json:"user"`
}

type Channel struct {
	ID                    string `json:"id"`
	Name                  string `json:"name"`
	Color                 string `json:"color"`
	IsSensitive           bool   `json:"isSensitive"`
	AllowRenoteToExternal bool   `json:"allowRenoteToExternal"`
	UserID                string `json:"userId"`
}

type Poll struct {
	Multiple  bool      `json:"multiple"`
	ExpiresAt time.Time `json:"expiresAt"`
	Choices   []struct {
		ID    int    `json:"id"`
		Text  string `json:"text"`
		Votes int    `json:"votes"`
	} `json:"choices"`
}

type Note struct {
	ID                       string            `json:"id"`
	CreatedAt                time.Time         `json:"createdAt"`
	DeletedAt                any               `json:"deletedAt"`
	Text                     any               `json:"text"`
	Cw                       any               `json:"cw"`
	UserID                   string            `json:"userId"`
	User                     User              `json:"user"`
	ReplyID                  string            `json:"replyId"`
	RenoteID                 string            `json:"renoteId"`
	Reply                    *Note             `json:"reply"`
	Renote                   *Note             `json:"renote"`
	IsHidden                 bool              `json:"isHidden"`
	Visibility               string            `json:"visibility"`
	Mentions                 []string          `json:"mentions"`
	VisibleUserIds           []string          `json:"visibleUserIds"`
	FileIds                  []string          `json:"fileIds"`
	Files                    []DriveFile       `json:"files"`
	Tags                     []string          `json:"tags"`
	Poll                     Poll              `json:"poll"`
	Emojis                   map[string]string `json:"emojis"`
	ChannelID                string            `json:"channelId"`
	Channel                  Channel           `json:"channel"`
	LocalOnly                bool              `json:"localOnly"`
	ReactionAcceptance       string            `json:"reactionAcceptance"`
	ReactionEmojis           map[string]string `json:"reactionEmojis"`
	Reactions                map[string]int    `json:"reactions"`
	ReactionCount            int               `json:"reactionCount"`
	RenoteCount              int               `json:"renoteCount"`
	RepliesCount             int               `json:"repliesCount"`
	URI                      string            `json:"uri"`
	URL                      string            `json:"url"`
	ReactionAndUserPairCache []string          `json:"reactionAndUserPairCache"`
	ClippedCount             int               `json:"clippedCount"`
	MyReaction               string            `json:"myReaction"`
}

type StreamingMessage struct {
	Type string        `json:"type"`
	Body StreamingBody `json:"body"`
}

type StreamingBody struct {
	ID   string `json:"id"`
	Type string `json:"type"`
	Body Note   `json:"body"`
}
