package rag

import (
	"embed"
	"fmt"
	"strings"
)

//go:embed prompts/*.prompt
var prompts embed.FS

func MustGetPrompt(name string) string {
	prompt, err := prompts.ReadFile(fmt.Sprintf("prompts/%s.prompt", name))
	if err != nil {
		panic(err)
	}
	return string(prompt)
}

func buildPrompt(query string, documents []DocumentChunk) string {
	var b strings.Builder
	b.WriteString("根据以下知识，使用中文回答问题：\n\n")
	for i, doc := range documents {
		b.WriteString(fmt.Sprintf("知识片段 %d：%s\n\n", i, doc.Text))
	}
	b.WriteString("问题：")
	b.WriteString(query)
	return b.String()
}
