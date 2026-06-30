// Package fling 提供 FLiNG 修改器网站的爬虫、搜索和下载功能。
package fling

import "testing"

// TestSymbolReplacement 测试文件名安全转换函数 SymbolReplacement。
// 参考 Python download_base_thread.py:300-302。
func TestSymbolReplacement(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "冒号+空格替换为空格+破折号+空格",
			input: "Game: Subtitle",
			want:  "Game - Subtitle",
		},
		{
			name:  "单独冒号替换为破折号",
			input: "Game:Sub",
			want:  "Game-Sub",
		},
		{
			name:  "斜杠替换为下划线",
			input: "A/B",
			want:  "A_B",
		},
		{
			name:  "问号替换为空",
			input: "What?",
			want:  "What",
		},
		{
			name:  "组合替换",
			input: "Game: Sub/Title?",
			want:  "Game - Sub_Title",
		},
		{
			name:  "无特殊字符保持不变",
			input: "NormalName",
			want:  "NormalName",
		},
		{
			name:  "空字符串",
			input: "",
			want:  "",
		},
		{
			name:  "多个连续替换",
			input: "A: B/C?D",
			want:  "A - B_CD",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SymbolReplacement(tt.input)
			if got != tt.want {
				t.Errorf("SymbolReplacement(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
