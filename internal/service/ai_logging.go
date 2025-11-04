package service

import (
	"log"
	"strings"
	"unicode/utf8"
)

const maxAILogSnippetRunes = 1024

// logAIExchange 用于输出 AI 请求与响应的关键信息，方便排查模型行为。
func logAIExchange(kind, phase, content string) {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		log.Printf("[AI %s] %s: <empty>", kind, phase)
		return
	}

	runeCount := utf8.RuneCountInString(trimmed)
	snippet := trimmed
	if runeCount > maxAILogSnippetRunes {
		snippet = string([]rune(trimmed)[:maxAILogSnippetRunes]) + "…(truncated)"
	}
	log.Printf("[AI %s] %s (runes=%d): %s", kind, phase, runeCount, snippet)
}
