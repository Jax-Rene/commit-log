package handler

import (
	"regexp"
	"strings"
	"testing"
)

var iframeAllowAutoplayPattern = regexp.MustCompile(`allow="[^"]*autoplay`)

func TestRenderMarkdown_VideoEmbeds(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		markdown   string
		wantSrc    string
		wantVendor string
	}{
		{
			name:       "youtube",
			markdown:   "https://www.youtube.com/watch?v=dQw4w9WgXcQ",
			wantSrc:    "https://www.youtube.com/embed/dQw4w9WgXcQ",
			wantVendor: "youtube",
		},
		{
			name:       "bilibili",
			markdown:   "https://www.bilibili.com/video/BV1x5411c7mD",
			wantSrc:    "player.bilibili.com/player.html",
			wantVendor: "bilibili",
		},
		{
			name:       "douyin",
			markdown:   "https://www.iesdouyin.com/share/video/7234567890123456789",
			wantSrc:    "iesdouyin.com/share/video/7234567890123456789",
			wantVendor: "douyin",
		},
		{
			name:       "douyin-modal-id",
			markdown:   "douyin.com/modal_id=7602245594001771802",
			wantSrc:    "iesdouyin.com/share/video/7602245594001771802",
			wantVendor: "douyin",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			rendered, err := renderMarkdown(tt.markdown)
			if err != nil {
				t.Fatalf("render markdown: %v", err)
			}

			html := string(rendered)
			if !strings.Contains(html, "<iframe") {
				t.Fatalf("expected iframe in output, got: %s", html)
			}
			if !strings.Contains(html, `class="video-embed is-loading"`) {
				t.Fatalf("expected loading class on video container, got: %s", html)
			}
			if !strings.Contains(html, tt.wantSrc) {
				t.Fatalf("expected iframe src to include %q, got: %s", tt.wantSrc, html)
			}
			if tt.wantVendor == "bilibili" && !strings.Contains(html, "autoplay=0") {
				t.Fatalf("expected bilibili iframe src to disable autoplay, got: %s", html)
			}
			if iframeAllowAutoplayPattern.MatchString(html) {
				t.Fatalf("expected iframe allow attr to disable autoplay, got: %s", html)
			}
			if !strings.Contains(html, "data-video-platform=\""+tt.wantVendor+"\"") {
				t.Fatalf("expected platform %q marker, got: %s", tt.wantVendor, html)
			}
		})
	}
}

func TestRenderMarkdown_SkipsVideoEmbedInsideCodeFence(t *testing.T) {
	t.Parallel()

	markdown := "```\nhttps://www.youtube.com/watch?v=dQw4w9WgXcQ\n```"
	rendered, err := renderMarkdown(markdown)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}

	html := string(rendered)
	if strings.Contains(html, "<iframe") {
		t.Fatalf("expected no iframe inside code fence, got: %s", html)
	}
}

func TestRenderMarkdown_YouTubeEmbedHasDisplayParams(t *testing.T) {
	t.Parallel()

	markdown := "<https://www.youtube.com/watch?v=dQw4w9WgXcQ&t=62s>"
	rendered, err := renderMarkdown(markdown)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}

	html := string(rendered)
	if !strings.Contains(html, "modestbranding=1") {
		t.Fatalf("expected modestbranding parameter, got: %s", html)
	}
	if !strings.Contains(html, "rel=0") {
		t.Fatalf("expected rel parameter, got: %s", html)
	}
	if !strings.Contains(html, "playsinline=1") {
		t.Fatalf("expected playsinline parameter, got: %s", html)
	}
	if !strings.Contains(html, "start=62") {
		t.Fatalf("expected start parameter, got: %s", html)
	}
}

func TestRenderMarkdown_BilibiliEmbedBlocksTopLevelNavigation(t *testing.T) {
	t.Parallel()

	markdown := "https://www.bilibili.com/video/BV1x5411c7mD"
	rendered, err := renderMarkdown(markdown)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}

	html := string(rendered)
	if !strings.Contains(html, `sandbox="allow-scripts allow-same-origin allow-presentation"`) {
		t.Fatalf("expected bilibili iframe sandbox to block top-level navigation, got: %s", html)
	}
}

func TestRenderMarkdown_SkipsInlineVideoURL(t *testing.T) {
	t.Parallel()

	markdown := "观看链接：https://www.youtube.com/watch?v=dQw4w9WgXcQ"
	rendered, err := renderMarkdown(markdown)
	if err != nil {
		t.Fatalf("render markdown: %v", err)
	}

	html := string(rendered)
	if strings.Contains(html, "<iframe") {
		t.Fatalf("expected inline url to remain a link, got: %s", html)
	}
}

func TestRenderMarkdown_RejectsLookalikeVideoDomains(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		markdown string
	}{
		{
			name:     "fake youtube domain",
			markdown: "https://notyoutube.com/watch?v=dQw4w9WgXcQ",
		},
		{
			name:     "fake bilibili domain",
			markdown: "https://notbilibili.com/video/BV1x5411c7mD",
		},
		{
			name:     "fake douyin domain",
			markdown: "https://notdouyin.com/video/7234567890123456789",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rendered, err := renderMarkdown(tt.markdown)
			if err != nil {
				t.Fatalf("render markdown: %v", err)
			}

			html := string(rendered)
			if strings.Contains(html, "<iframe") {
				t.Fatalf("expected no iframe for lookalike domain, got: %s", html)
			}
		})
	}
}
