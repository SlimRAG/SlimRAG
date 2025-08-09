package rag

import (
	"fmt"
	"strings"
)

func BuildPrompt(query string, documents []DocumentChunk) string {
	return BuildPromptWithSystem(query, documents, "根据以下知识，使用中文回答问题：")
}

func BuildPromptWithSystem(query string, documents []DocumentChunk, systemPrompt string) string {
	var b strings.Builder
	b.WriteString(systemPrompt)
	b.WriteString("\n\n")
	for i, doc := range documents {
		b.WriteString(fmt.Sprintf("知识片段 %d：%s\n\n", i, doc.Text))
	}
	b.WriteString("问题：")
	b.WriteString(query)
	return b.String()
}
