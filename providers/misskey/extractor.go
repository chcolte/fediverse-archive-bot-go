package misskey

// Noteオブジェクトから全てのURLを抽出する
func extractURL(note Note) []string {
	var urls []string
	urls = append(urls, extractURLUser(note.User)...)
	urls = append(urls, extractURLEmojis(note.Emojis)...)
	urls = append(urls, extractURLReactionEmojis(note.ReactionEmojis)...)

	for _, file := range note.Files {
		urls = append(urls, extractURLFile(file)...)
	}

	if note.Renote != nil {
		urls = append(urls, extractURL(*note.Renote)...)
	}

	if note.Reply != nil {
		urls = append(urls, extractURLReply(note.Reply)...)
	}

	return urls
}

// 空文字列をフィルタリングしてNoteオブジェクトからURLを抽出する
func SafeExtractURL(note Note) []string {
	urls := extractURL(note)
	var safeUrls []string
	for _, url := range urls {
		if url != "" {
			safeUrls = append(safeUrls, url)
		}
	}
	return safeUrls
}

func extractURLInstance(instance Instance) []string {
	var urls []string
	urls = append(urls, instance.IconURL)
	urls = append(urls, instance.FaviconURL)
	return urls
}

func extractURLAvatarDecoration(decoration AvatarDecoration) []string {
	var urls []string
	urls = append(urls, decoration.URL)
	return urls
}

func extractURLEmojis(emojis map[string]string) []string {
	var urls []string
	for _, url := range emojis {
		urls = append(urls, url)
	}
	return urls
}

func extractURLReactionEmojis(emojis map[string]string) []string {
	return extractURLEmojis(emojis)
}

func extractURLBadgeRole(badgeRole BadgeRole) []string {
	var urls []string
	urls = append(urls, badgeRole.IconURL)
	return urls
}

func extractURLUser(user User) []string {
	var urls []string
	urls = append(urls, user.AvatarURL)
	for _, decoration := range user.AvatarDecorations {
		urls = append(urls, extractURLAvatarDecoration(decoration)...)
	}

	urls = append(urls, extractURLEmojis(user.Emojis)...)

	for _, role := range user.BadgeRoles {
		urls = append(urls, extractURLBadgeRole(role)...)
	}

	urls = append(urls, extractURLInstance(user.Instance)...)

	return urls
}

func extractURLFile(file DriveFile) []string {
	var urls []string
	urls = append(urls, file.URL)
	urls = append(urls, file.ThumbnailURL)
	urls = append(urls, extractURLUser(file.User)...)
	return urls
}

func extractURLReply(reply *Note) []string {
	var urls []string
	if reply == nil {
		return urls
	}
	urls = append(urls, extractURLUser(reply.User)...)
	urls = append(urls, extractURLReactionEmojis(reply.ReactionEmojis)...)
	urls = append(urls, extractURLEmojis(reply.Emojis)...)

	for _, file := range reply.Files {
		urls = append(urls, extractURLFile(file)...)
	}

	if reply.Renote != nil {
		urls = append(urls, extractURL(*reply.Renote)...)
	}
	if reply.Reply != nil {
		urls = append(urls, extractURLReply(reply.Reply)...)
	}
	return urls
}
