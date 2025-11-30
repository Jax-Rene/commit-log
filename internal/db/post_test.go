package db

import "testing"

func TestDeriveTitleFromContentStripsEmphasis(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    string
	}{
		{
			name:    "strip bold heading",
			content: "# **粗体标题**\n正文",
			want:    "粗体标题",
		},
		{
			name:    "strip italic heading",
			content: "# *斜体标题* \n内容",
			want:    "斜体标题",
		},
		{
			name:    "strip emphasis without heading",
			content: "*无标题内容*\n更多",
			want:    "无标题内容",
		},
		{
			name:    "strip mixed emphasis",
			content: "# ***混合***",
			want:    "混合",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := DeriveTitleFromContent(tt.content)
			if got != tt.want {
				t.Fatalf("expected %q, got %q", tt.want, got)
			}
		})
	}
}
