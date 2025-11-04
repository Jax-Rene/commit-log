package service

import (
	"fmt"
	"regexp"
	"strings"
)

var markdownImagePattern = regexp.MustCompile(`!\[[^\]]*]\((<[^>]+>|[^)\s]+)([^)]*)\)`)

// markdownImagePlaceholders 负责在生成 AI Prompt 前压缩图片链接，并在拿到结果后恢复原始链接。
type markdownImagePlaceholders struct {
	replacements map[string]string
}

// compressMarkdownImageURLs 将 Markdown 图片的长链接替换为短占位符，降低 Prompt 的 Token 消耗。
func compressMarkdownImageURLs(input string) (string, *markdownImagePlaceholders) {
	if !markdownImagePattern.MatchString(input) {
		return input, &markdownImagePlaceholders{replacements: nil}
	}

	index := 1
	placeholders := &markdownImagePlaceholders{replacements: make(map[string]string)}

	result := markdownImagePattern.ReplaceAllStringFunc(input, func(match string) string {
		groups := markdownImagePattern.FindStringSubmatch(match)
		if len(groups) < 3 {
			return match
		}

		original := groups[1]
		placeholder := fmt.Sprintf("image://asset-%d", index)
		index++

		replacement := placeholder
		if strings.HasPrefix(original, "<") && strings.HasSuffix(original, ">") {
			replacement = fmt.Sprintf("<%s>", placeholder)
		}

		placeholders.replacements[replacement] = original
		return strings.Replace(match, original, replacement, 1)
	})

	return result, placeholders
}

// Count 返回被替换的图片数量。
func (p *markdownImagePlaceholders) Count() int {
	if p == nil || len(p.replacements) == 0 {
		return 0
	}
	return len(p.replacements)
}

// Restore 将占位符恢复为原始的图片链接，确保最终内容不会丢失原始信息。
func (p *markdownImagePlaceholders) Restore(input string) string {
	if p == nil || len(p.replacements) == 0 {
		return input
	}

	output := input
	for placeholder, original := range p.replacements {
		output = strings.ReplaceAll(output, placeholder, original)

		strippedPlaceholder := strings.TrimSuffix(strings.TrimPrefix(placeholder, "<"), ">")
		strippedOriginal := strings.TrimSuffix(strings.TrimPrefix(original, "<"), ">")

		if strippedPlaceholder != placeholder {
			// 占位符包含尖括号，额外处理去掉括号的情况
			output = strings.ReplaceAll(output, strippedPlaceholder, strippedOriginal)
			continue
		}

		// 占位符本身不含尖括号，但模型可能自动加上，需要统一还原
		replacement := strippedOriginal
		if strippedOriginal == original {
			replacement = fmt.Sprintf("<%s>", strippedOriginal)
		} else {
			replacement = original
		}
		output = strings.ReplaceAll(output, fmt.Sprintf("<%s>", placeholder), replacement)
	}
	return output
}
