package view

import "strings"

// ProfileIconOption describes a selectable icon option for profile contacts.
type ProfileIconOption struct {
	Key   string `json:"key"`
	Label string `json:"label"`
}

type profileIconAsset struct {
	Key   string
	SVG   string
	Label string
}

var (
	profileIconDefinitions = []profileIconAsset{
		{Key: "wechat", Label: "微信", SVG: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M20.25 8.511C21.134 8.795 21.75 9.639 21.75 10.608v4.286c0 1.136-.847 2.1-1.98 2.192-.339.027-.678.052-1.02.072V20.25L15.75 17.25c-1.354 0-2.695-.055-4.02-.164-.298-.024-.577-.11-.825-.242M20.25 8.511a2.4 2.4 0 0 0-.476-.095C18.447 8.306 17.105 8.25 15.75 8.25c-1.355 0-2.697.056-4.024.166-1.131.093-1.976 1.056-1.976 2.192v4.285c0 .838.46 1.582 1.155 1.952M20.25 8.511V6.637c0-1.621-1.152-3.027-2.76-3.235A53.77 53.77 0 0 0 11.25 3c-2.115 0-4.198.137-6.24.402C3.402 3.61 2.25 5.016 2.25 6.637v6.225c0 1.621 1.152 3.026 2.76 3.235.577.075 1.157.139 1.74.193V21l4.155-4.155"/></svg>`},
		{Key: "github", Label: "GitHub", SVG: `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M12 .297c-6.63 0-12 5.373-12 12 0 5.303 3.438 9.8 8.205 11.385.6.113.82-.258.82-.577 0-.285-.01-1.04-.015-2.04-3.338.724-4.042-1.61-4.042-1.61-.546-1.142-1.335-1.512-1.335-1.512-1.087-.744.084-.729.084-.729 1.205.084 1.838 1.236 1.838 1.236 1.07 1.835 2.809 1.305 3.495.998.108-.776.417-1.305.76-1.605-2.665-.3-5.466-1.332-5.466-5.93 0-1.31.465-2.38 1.235-3.22-.135-.303-.54-1.523.105-3.176 0 0 1.005-.322 3.3 1.23.96-.267 1.98-.399 3-.405 1.02.006 2.04.138 3 .405 2.28-1.552 3.285-1.23 3.285-1.23.645 1.653.24 2.873.12 3.176.765.84 1.23 1.91 1.23 3.22 0 4.61-2.805 5.625-5.475 5.92.42.36.81 1.096.81 2.22 0 1.606-.015 2.896-.015 3.286 0 .315.21.69.825.57C20.565 22.092 24 17.592 24 12.297c0-6.627-5.373-12-12-12"/></svg>`},
		{Key: "email", Label: "邮箱", SVG: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M21.75 6.75v10.5a2.25 2.25 0 0 1-2.25 2.25h-15A2.25 2.25 0 0 1 2.25 17.25V6.75M21.75 6.75A2.25 2.25 0 0 0 19.5 4.5h-15A2.25 2.25 0 0 0 2.25 6.75v.243c0 .781.405 1.506 1.071 1.916l7.5 4.615a2.25 2.25 0 0 0 2.157 0l7.5-4.615a2.25 2.25 0 0 0 1.072-1.916V6.75"/></svg>`},
		{Key: "telegram", Label: "Telegram", SVG: `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M11.944 0A12 12 0 0 0 0 12a12 12 0 0 0 12 12 12 12 0 0 0 12-12A12 12 0 0 0 12 0a12 12 0 0 0-.056 0zm4.962 7.224c.1-.002.321.023.465.14a.506.506 0 0 1 .171.325c.016.093.036.306.02.472-.18 1.898-.962 6.502-1.36 8.627-.168.9-.499 1.201-.82 1.23-.696.065-1.225-.46-1.9-.902-1.056-.693-1.653-1.124-2.678-1.8-1.185-.78-.417-1.21.258-1.91.177-.184 3.247-2.977 3.307-3.23.007-.032.014-.15-.056-.212s-.174-.041-.249-.024c-.106.024-1.793 1.14-5.061 3.345-.48.33-.913.49-1.302.48-.428-.008-1.252-.241-1.865-.44-.752-.245-1.349-.374-1.297-.789.027-.216.325-.437.893-.663 3.498-1.524 5.83-2.529 6.998-3.014 3.332-1.386 4.025-1.627 4.476-1.635z"/></svg>`},
		{Key: "x", Label: "X / Twitter", SVG: `<svg viewBox="0 0 24 24" fill="currentColor" aria-hidden="true"><path d="M18.901 1.153h3.68l-8.04 9.19L24 22.846h-7.406l-5.8-7.584-6.638 7.584H.474l8.6-9.83L0 1.154h7.594l5.243 6.932ZM17.61 20.644h2.039L6.486 3.24H4.298Z"/></svg>`},
		{Key: "website", Label: "个人网站", SVG: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M12 21c4.193 0 7.716-2.867 8.716-6.747M12 21c-4.193 0-7.716-2.867-8.716-6.747M12 21c2.485 0 4.5-4.03 4.5-9s-2.015-9-4.5-9m0 18c-2.485 0-4.5-4.03-4.5-9s2.015-9 4.5-9m0-0c3.365 0 6.299 1.847 7.843 4.582M12 3c-3.365 0-6.299 1.847-7.843 4.582m15.686 0c.737 1.305 1.157 2.812 1.157 4.418 0 .778-.099 1.533-.284 2.253m-.873 4.836C18.133 15.685 15.162 16.5 12 16.5s-6.134-.815-8.716-2.247m0 0A8.948 8.948 0 0 1 3 12c0-1.605.42-3.112 1.157-4.417"/></svg>`},
	}
	defaultProfileIcon = profileIconAsset{Key: "default", Label: "默认", SVG: `<svg viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="1.5" stroke-linecap="round" stroke-linejoin="round"><path d="M17.982 18.725C16.612 16.918 14.442 15.75 12 15.75s-4.612 1.168-5.982 2.975M17.982 18.725A8.97 8.97 0 0 0 21 12c0-4.971-4.03-9-9-9s-9 4.029-9 9a8.97 8.97 0 0 0 3.018 6.725M17.982 18.725C16.392 20.14 14.296 21 12 21s-4.392-.86-5.982-2.275M15 9.75a3 3 0 1 1-6 0 3 3 0 0 1 6 0Z"/></svg>`}
	profileIconLookup  = func() map[string]profileIconAsset {
		lookup := make(map[string]profileIconAsset, len(profileIconDefinitions)+1)
		for _, icon := range profileIconDefinitions {
			lookup[icon.Key] = icon
		}
		lookup[defaultProfileIcon.Key] = defaultProfileIcon
		return lookup
	}()
)

// ProfileIconOptions exposes the selectable icon metadata for admin UI.
func ProfileIconOptions() []ProfileIconOption {
	options := make([]ProfileIconOption, 0, len(profileIconDefinitions))
	for _, icon := range profileIconDefinitions {
		options = append(options, ProfileIconOption{Key: icon.Key, Label: icon.Label})
	}
	return options
}

// ProfileIconSVGMap returns a copy of the key-to-SVG map including the default fallback.
func ProfileIconSVGMap() map[string]string {
	clones := make(map[string]string, len(profileIconLookup))
	for key, icon := range profileIconLookup {
		clones[key] = icon.SVG
	}
	return clones
}

// ProfileIconSVG resolves the SVG string for a given key, falling back to the default icon.
func ProfileIconSVG(key string) string {
	trimmed := strings.ToLower(strings.TrimSpace(key))
	if trimmed == "" {
		return defaultProfileIcon.SVG
	}
	if icon, ok := profileIconLookup[trimmed]; ok {
		return icon.SVG
	}
	return defaultProfileIcon.SVG
}

// DefaultProfileIconSVG returns the fallback SVG.
func DefaultProfileIconSVG() string {
	return defaultProfileIcon.SVG
}
