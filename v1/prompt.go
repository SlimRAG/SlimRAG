package rag

import (
	"fmt"
	"strings"
)

func BuildPrompt(query string, documents []DocumentChunk) string {
	var b strings.Builder
	b.WriteString("根据以下知识回答问题：\n\n")
	for i, doc := range documents {
		b.WriteString(fmt.Sprintf("知识片段 %d：%s\n\n", i, doc.Text))
	}
	b.WriteString("问题：")
	b.WriteString(query)
	return b.String()
}
