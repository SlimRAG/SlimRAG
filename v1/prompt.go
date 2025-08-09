package rag

import (
	"fmt"
	"strings"
)

func BuildPrompt(query string, documents []DocumentChunk) string {
	return BuildPromptWithSystem(query, documents, "Answer the question based on the following knowledge in Chinese: ")
}

func BuildPromptWithSystem(query string, documents []DocumentChunk, systemPrompt string) string {
	var b strings.Builder
	b.WriteString(systemPrompt)
	b.WriteString("\n\n")
	for i, doc := range documents {
		b.WriteString(fmt.Sprintf("Knowledge fragment %d: %s\n\n", i, doc.Text))
	}
	b.WriteString("Question: ")
	b.WriteString(query)
	return b.String()
}
