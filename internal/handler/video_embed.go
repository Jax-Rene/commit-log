package handler

import (
	"fmt"
	htmlstd "html"
	"net/url"
	"regexp"
	"strconv"
	"strings"

	"github.com/microcosm-cc/bluemonday"
)

const (
	videoAspectLandscape = "16:9"
	videoAspectPortrait  = "9:16"
)

var (
	videoEmbedLinePattern = regexp.MustCompile(`^\s*<?((?:https?://)?[^\s]+)>?\s*$`)
	videoEmbedSrcPattern  = regexp.MustCompile(
		`^https://(?:www\.)?(?:youtube\.com/embed/|youtube-nocookie\.com/embed/|player\.bilibili\.com/player\.html(?:\?|$)|www\.iesdouyin\.com/share/video/|www\.douyin\.com/video/|v\.douyin\.com/)`,
	)
	videoEmbedTimePattern = regexp.MustCompile(`(?i)(\d+)(h|m|s)`) // for YouTube t=1h2m3s
)

func buildContentSanitizer() *bluemonday.Policy {
	policy := bluemonday.UGCPolicy()
	policy.AllowElements("iframe")
	policy.AllowAttrs("class", "data-video-embed", "data-video-platform", "data-video-aspect", "data-video-source").OnElements("div")
	policy.AllowAttrs("src").Matching(videoEmbedSrcPattern).OnElements("iframe")
	policy.AllowAttrs("title", "allow", "allowfullscreen", "frameborder", "loading", "referrerpolicy", "sandbox").OnElements("iframe")
	return policy
}

type videoEmbed struct {
	Platform string
	Source   string
	EmbedURL string
	Aspect   string
}

func applyVideoEmbeds(markdown string) string {
	if strings.TrimSpace(markdown) == "" {
		return markdown
	}

	lines := strings.Split(markdown, "\n")
	inFence := false
	fenceMarker := ""

	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if marker := detectFenceMarker(trimmed); marker != "" {
			if inFence {
				if strings.HasPrefix(trimmed, fenceMarker) {
					inFence = false
					fenceMarker = ""
				}
			} else {
				inFence = true
				fenceMarker = marker
			}
			continue
		}

		if inFence {
			continue
		}

		if isIndentedCodeLine(line) || shouldSkipEmbedLine(trimmed) {
			continue
		}

		urlValue, ok := extractVideoURL(trimmed)
		if !ok {
			continue
		}

		embed, ok := parseVideoEmbed(urlValue)
		if !ok {
			continue
		}

		lines[i] = buildVideoEmbedHTML(embed)
	}

	return strings.Join(lines, "\n")
}

func detectFenceMarker(line string) string {
	if strings.HasPrefix(line, "```") {
		return "```"
	}
	if strings.HasPrefix(line, "~~~") {
		return "~~~"
	}
	return ""
}

func isIndentedCodeLine(line string) bool {
	return strings.HasPrefix(line, "    ") || strings.HasPrefix(line, "\t")
}

func shouldSkipEmbedLine(line string) bool {
	if line == "" {
		return true
	}
	if strings.HasPrefix(line, ">") {
		return true
	}
	if strings.HasPrefix(line, "- ") || strings.HasPrefix(line, "* ") || strings.HasPrefix(line, "+ ") {
		return true
	}
	if listIndexPattern.MatchString(line) {
		return true
	}
	return false
}

var listIndexPattern = regexp.MustCompile(`^\d+\.\s+`)

func extractVideoURL(line string) (string, bool) {
	match := videoEmbedLinePattern.FindStringSubmatch(line)
	if match == nil {
		return "", false
	}
	value := strings.TrimSpace(match[1])
	if value == "" {
		return "", false
	}
	return value, true
}

func parseVideoEmbed(raw string) (videoEmbed, bool) {
	trimmed := strings.TrimSpace(raw)
	trimmed = strings.TrimPrefix(trimmed, "<")
	trimmed = strings.TrimSuffix(trimmed, ">")
	trimmed = normalizeVideoURL(trimmed)
	parsed, err := url.Parse(trimmed)
	if err != nil || parsed == nil {
		return videoEmbed{}, false
	}

	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return videoEmbed{}, false
	}

	host := strings.ToLower(parsed.Hostname())
	if host == "" {
		return videoEmbed{}, false
	}

	if embed, ok := parseYouTubeEmbed(parsed, trimmed); ok {
		return embed, true
	}
	if embed, ok := parseBilibiliEmbed(parsed, trimmed); ok {
		return embed, true
	}
	if embed, ok := parseDouyinEmbed(parsed, trimmed); ok {
		return embed, true
	}
	return videoEmbed{}, false
}

func normalizeVideoURL(raw string) string {
	if raw == "" {
		return raw
	}
	lower := strings.ToLower(raw)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") {
		return raw
	}
	knownPrefixes := []string{
		"douyin.com/",
		"www.douyin.com/",
		"iesdouyin.com/",
		"www.iesdouyin.com/",
		"v.douyin.com/",
		"bilibili.com/",
		"www.bilibili.com/",
		"youtube.com/",
		"www.youtube.com/",
		"youtu.be/",
	}
	for _, prefix := range knownPrefixes {
		if strings.HasPrefix(lower, prefix) {
			return "https://" + raw
		}
	}
	return raw
}

func parseYouTubeEmbed(u *url.URL, source string) (videoEmbed, bool) {
	host := strings.ToLower(u.Hostname())
	var videoID string

	switch {
	case host == "youtu.be":
		videoID = strings.Trim(strings.TrimPrefix(u.Path, "/"), "/")
		if strings.Contains(videoID, "/") {
			videoID = strings.Split(videoID, "/")[0]
		}
	case isHostOrSubdomain(host, "youtube.com"):
		path := strings.Trim(u.Path, "/")
		if path == "watch" {
			videoID = u.Query().Get("v")
		} else if strings.HasPrefix(path, "shorts/") {
			videoID = strings.TrimPrefix(path, "shorts/")
		} else if strings.HasPrefix(path, "embed/") {
			videoID = strings.TrimPrefix(path, "embed/")
		} else if strings.HasPrefix(path, "live/") {
			videoID = strings.TrimPrefix(path, "live/")
		}
		if strings.Contains(videoID, "/") {
			videoID = strings.Split(videoID, "/")[0]
		}
	default:
		return videoEmbed{}, false
	}

	if videoID == "" {
		return videoEmbed{}, false
	}

	embedURL := fmt.Sprintf("https://www.youtube.com/embed/%s", videoID)
	embedValues := url.Values{}
	embedValues.Set("rel", "0")
	embedValues.Set("modestbranding", "1")
	embedValues.Set("playsinline", "1")
	if start := parseYouTubeStart(u); start > 0 {
		embedValues.Set("start", strconv.Itoa(start))
	}
	embedURL = embedURL + "?" + embedValues.Encode()

	return videoEmbed{
		Platform: "youtube",
		Source:   source,
		EmbedURL: embedURL,
		Aspect:   videoAspectLandscape,
	}, true
}

func parseYouTubeStart(u *url.URL) int {
	if u == nil {
		return 0
	}
	query := u.Query()
	if value := query.Get("start"); value != "" {
		return parseYouTubeTime(value)
	}
	if value := query.Get("t"); value != "" {
		return parseYouTubeTime(value)
	}
	return 0
}

func parseYouTubeTime(value string) int {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return 0
	}
	if onlyDigits(trimmed) {
		seconds, err := strconv.Atoi(trimmed)
		if err == nil && seconds > 0 {
			return seconds
		}
		return 0
	}

	matches := videoEmbedTimePattern.FindAllStringSubmatch(trimmed, -1)
	if len(matches) == 0 {
		return 0
	}

	total := 0
	for _, match := range matches {
		value, err := strconv.Atoi(match[1])
		if err != nil || value <= 0 {
			continue
		}
		switch strings.ToLower(match[2]) {
		case "h":
			total += value * 3600
		case "m":
			total += value * 60
		case "s":
			total += value
		}
	}

	return total
}

func onlyDigits(value string) bool {
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return value != ""
}

func parseBilibiliEmbed(u *url.URL, source string) (videoEmbed, bool) {
	host := strings.ToLower(u.Hostname())
	if !isHostOrSubdomain(host, "bilibili.com") {
		return videoEmbed{}, false
	}

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	if len(segments) < 2 || segments[0] != "video" {
		return videoEmbed{}, false
	}

	rawID := segments[1]
	if rawID == "" {
		return videoEmbed{}, false
	}

	page := parsePositiveInt(u.Query().Get("p"), 1)

	values := url.Values{}
	lowerID := strings.ToLower(rawID)
	switch {
	case strings.HasPrefix(lowerID, "bv"):
		values.Set("bvid", rawID)
	case strings.HasPrefix(lowerID, "av"):
		values.Set("aid", strings.TrimPrefix(lowerID, "av"))
	case onlyDigits(rawID):
		values.Set("aid", rawID)
	default:
		return videoEmbed{}, false
	}
	values.Set("page", strconv.Itoa(page))
	values.Set("high_quality", "1")
	values.Set("danmaku", "0")
	values.Set("autoplay", "0")

	embedURL := "https://player.bilibili.com/player.html?" + values.Encode()

	return videoEmbed{
		Platform: "bilibili",
		Source:   source,
		EmbedURL: embedURL,
		Aspect:   videoAspectLandscape,
	}, true
}

func parseDouyinEmbed(u *url.URL, source string) (videoEmbed, bool) {
	host := strings.ToLower(u.Hostname())
	if !isHostOrSubdomain(host, "douyin.com") && !isHostOrSubdomain(host, "iesdouyin.com") {
		return videoEmbed{}, false
	}

	segments := strings.Split(strings.Trim(u.Path, "/"), "/")
	videoID := ""
	for idx, segment := range segments {
		if segment == "video" && idx+1 < len(segments) {
			videoID = segments[idx+1]
			break
		}
		if strings.HasPrefix(segment, "modal_id=") {
			videoID = strings.TrimPrefix(segment, "modal_id=")
			break
		}
	}
	if videoID == "" {
		if candidate := u.Query().Get("modal_id"); candidate != "" {
			videoID = candidate
		}
	}

	embedURL := ""
	if videoID != "" {
		embedURL = fmt.Sprintf("https://www.iesdouyin.com/share/video/%s", videoID)
	} else if host == "v.douyin.com" {
		embedURL = source
	} else {
		return videoEmbed{}, false
	}

	return videoEmbed{
		Platform: "douyin",
		Source:   source,
		EmbedURL: embedURL,
		Aspect:   videoAspectPortrait,
	}, true
}

func buildVideoEmbedHTML(embed videoEmbed) string {
	platform := htmlstd.EscapeString(embed.Platform)
	aspect := htmlstd.EscapeString(embed.Aspect)
	source := htmlstd.EscapeString(embed.Source)
	embedURL := htmlstd.EscapeString(embed.EmbedURL)
	title := htmlstd.EscapeString(videoEmbedTitle(embed.Platform))
	sandboxAttr := videoEmbedSandboxAttribute(embed.Platform)

	return fmt.Sprintf(
		`<div class="video-embed is-loading" data-video-embed="true" data-video-platform="%s" data-video-aspect="%s" data-video-source="%s">`+
			`<iframe src="%s" title="%s" loading="lazy" allow="%s" allowfullscreen frameborder="0" referrerpolicy="strict-origin-when-cross-origin"%s></iframe>`+
			`</div>`,
		platform,
		aspect,
		source,
		embedURL,
		title,
		videoEmbedAllowAttribute(),
		sandboxAttr,
	)
}

func videoEmbedTitle(platform string) string {
	switch platform {
	case "youtube":
		return "YouTube 视频播放器"
	case "bilibili":
		return "B 站视频播放器"
	case "douyin":
		return "抖音视频播放器"
	default:
		return "视频播放器"
	}
}

func videoEmbedAllowAttribute() string {
	return "accelerometer; clipboard-write; encrypted-media; gyroscope; picture-in-picture; web-share"
}

func videoEmbedSandboxAttribute(platform string) string {
	if platform == "bilibili" {
		return ` sandbox="allow-scripts allow-same-origin allow-presentation"`
	}
	return ""
}

func isHostOrSubdomain(host, domain string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	domain = strings.ToLower(strings.TrimSpace(domain))
	if host == "" || domain == "" {
		return false
	}
	return host == domain || strings.HasSuffix(host, "."+domain)
}
