package rag

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode/utf8"

	"github.com/go-ego/gse"
	"github.com/rs/zerolog/log"
)

// ChunkingConfig chunking configuration
type ChunkingConfig struct {
	MaxChunkSize        int     `json:"max_chunk_size"`       // Maximum chunk size (in characters)
	MinChunkSize        int     `json:"min_chunk_size"`       // Minimum chunk size (in characters)
	OverlapSize         int     `json:"overlap_size"`         // Overlap size (in characters)
	SentenceWindow      int     `json:"sentence_window"`      // Sentence window size
	Strategy            string  `json:"strategy"`             // Chunking strategy: "fixed", "semantic", "sentence", "adaptive"
	Language            string  `json:"language"`             // Language: "zh", "en", "auto"
	PreserveSections    bool    `json:"preserve_sections"`    // Whether to preserve section structure
	SimilarityThreshold float64 `json:"similarity_threshold"` // Semantic similarity threshold
}

// DefaultChunkingConfig default configuration
func DefaultChunkingConfig() *ChunkingConfig {
	return &ChunkingConfig{
		MaxChunkSize:        1000,
		MinChunkSize:        100,
		OverlapSize:         50,
		SentenceWindow:      3,
		Strategy:            "adaptive",
		Language:            "auto",
		PreserveSections:    true,
		SimilarityThreshold: 0.7,
	}
}

// LoadChunkingConfig loads configuration from JSON file
func LoadChunkingConfig(configPath string) (*ChunkingConfig, error) {
	if configPath == "" {
		return DefaultChunkingConfig(), nil
	}

	file, err := os.Open(configPath)
	if err != nil {
		return DefaultChunkingConfig(), nil // Use default configuration
	}
	defer file.Close()

	data, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	config := &ChunkingConfig{}
	err = json.Unmarshal(data, config)
	if err != nil {
		return nil, err
	}

	// Validate configuration
	if config.MaxChunkSize <= 0 {
		config.MaxChunkSize = 1000
	}
	if config.MinChunkSize <= 0 {
		config.MinChunkSize = 100
	}
	if config.OverlapSize < 0 {
		config.OverlapSize = 50
	}
	if config.Strategy == "" {
		config.Strategy = "adaptive"
	}

	return config, nil
}

// DocumentChunker document chunker
type DocumentChunker struct {
	config    *ChunkingConfig
	segmenter *gse.Segmenter
	cwd       string
}

// NewDocumentChunker creates a new document chunker
func NewDocumentChunker(config *ChunkingConfig, cwd string) (*DocumentChunker, error) {
	var err error

	if config == nil {
		config = DefaultChunkingConfig()
	}

	if cwd == "" {
		cwd, err = os.Getwd()
		if err != nil {
			log.Panic().Err(err).Stack().Msg("Getwd() failed")
		}
	}

	chunker := &DocumentChunker{
		config:    config,
		segmenter: &gse.Segmenter{},
		cwd:       cwd,
	}

	// Configure tokenizer based on language
	switch config.Language {
	case "zh":
		chunker.segmenter.LoadDict("zh_s")
	case "en":
		chunker.segmenter.LoadDict()
	default:
		// Auto-detect, load Chinese dictionary by default
		chunker.segmenter.LoadDict("zh_s")
	}

	return chunker, nil
}

// ChunkDocument chunks a document
func (c *DocumentChunker) ChunkDocument(content string, fileName string) (*Document, error) {
	if content == "" {
		return nil, fmt.Errorf("content is empty")
	}

	content = c.preprocessText(content)
	var chunks []*DocumentChunk

	switch c.config.Strategy {
	case "fixed":
		chunks = c.fixedSizeChunking(content)
	case "semantic":
		chunks = c.semanticChunking(content)
	case "sentence":
		chunks = c.sentenceChunking(content)
	case "adaptive":
		chunks = c.adaptiveChunking(content)
	default:
		chunks = c.adaptiveChunking(content)
	}

	doc := &Document{
		FileName:    fileName,
		Document:    strings.TrimSuffix(fileName, filepath.Ext(fileName)),
		RawDocument: fileName,
		Chunks:      chunks,
	}

	doc.Fix()
	return doc, nil
}

// ChunkDocumentWithFilePath chunks a document, using file path to generate document_id
func (c *DocumentChunker) ChunkDocumentWithFilePath(content string, filePath string) (*Document, error) {
	if content == "" {
		return nil, fmt.Errorf("content is empty")
	}

	content = c.preprocessText(content)
	var chunks []*DocumentChunk

	switch c.config.Strategy {
	case "fixed":
		chunks = c.fixedSizeChunking(content)
	case "semantic":
		chunks = c.semanticChunking(content)
	case "sentence":
		chunks = c.sentenceChunking(content)
	case "adaptive":
		chunks = c.adaptiveChunking(content)
	default:
		chunks = c.adaptiveChunking(content)
	}

	// Calculate relative path
	relPath, err := filepath.Rel(c.cwd, filePath)
	if err != nil {
		// If calculating relative path fails, use full path
		relPath = filePath
	}

	doc := &Document{
		FileName:    filepath.Base(filePath),
		FilePath:    relPath,
		RawDocument: filepath.Base(filePath),
		Chunks:      chunks,
	}

	doc.Fix()
	return doc, nil
}

// preprocessText preprocesses text
func (c *DocumentChunker) preprocessText(text string) string {
	// Remove extra whitespace characters
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	// Remove null characters
	text = strings.ReplaceAll(text, "\u0000", "")
	// Normalize line endings
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	// Remove extra line breaks
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}

// fixedSizeChunking fixed-size chunking
func (c *DocumentChunker) fixedSizeChunking(content string) []*DocumentChunk {
	var chunks []*DocumentChunk
	runes := []rune(content)
	totalLen := len(runes)

	for i := 0; i < totalLen; {
		end := i + c.config.MaxChunkSize
		if end > totalLen {
			end = totalLen
		}

		// Try to split at sentence boundaries
		if end < totalLen {
			for j := end; j > i+c.config.MinChunkSize && j > i; j-- {
				if c.isSentenceBoundary(runes[j]) {
					end = j + 1
					break
				}
			}
		}

		chunkText := strings.TrimSpace(string(runes[i:end]))
		if len([]rune(chunkText)) >= c.config.MinChunkSize {
			chunk := &DocumentChunk{
				Text:  chunkText,
				Index: len(chunks),
			}
			chunks = append(chunks, chunk)
		}

		// Calculate next starting position (considering overlap)
		nextStart := end - c.config.OverlapSize
		if nextStart <= i {
			nextStart = i + 1
		}
		i = nextStart
	}

	return chunks
}

// semanticChunking semantic chunking
func (c *DocumentChunker) semanticChunking(content string) []*DocumentChunk {
	// First split by paragraphs
	paragraphs := c.splitIntoParagraphs(content)
	var chunks []*DocumentChunk
	currentChunk := ""

	for _, paragraph := range paragraphs {
		// If current paragraph is too long, need further splitting
		if utf8.RuneCountInString(paragraph) > c.config.MaxChunkSize {
			// Save current chunk
			if currentChunk != "" {
				chunk := &DocumentChunk{
					Text:  strings.TrimSpace(currentChunk),
					Index: len(chunks),
				}
				chunks = append(chunks, chunk)
				currentChunk = ""
			}
			// Perform sentence-level splitting on long paragraphs
			sentenceChunks := c.splitLongParagraph(paragraph)
			chunks = append(chunks, sentenceChunks...)
			continue
		}

		// Check if adding current paragraph will exceed maximum size
		potentialChunk := currentChunk
		if potentialChunk != "" {
			potentialChunk += "\n\n"
		}
		potentialChunk += paragraph

		if utf8.RuneCountInString(potentialChunk) > c.config.MaxChunkSize {
			// Save current chunk
			if currentChunk != "" {
				chunk := &DocumentChunk{
					Text:  strings.TrimSpace(currentChunk),
					Index: len(chunks),
				}
				chunks = append(chunks, chunk)
			}
			currentChunk = paragraph
		} else {
			currentChunk = potentialChunk
		}
	}

	// Save last chunk
	if currentChunk != "" && utf8.RuneCountInString(currentChunk) >= c.config.MinChunkSize {
		chunk := &DocumentChunk{
			Text:  strings.TrimSpace(currentChunk),
			Index: len(chunks),
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

// sentenceChunking sentence-level chunking
func (c *DocumentChunker) sentenceChunking(content string) []*DocumentChunk {
	sentences := c.splitIntoSentences(content)
	var chunks []*DocumentChunk
	currentChunk := ""
	sentenceCount := 0

	for _, sentence := range sentences {
		potentialChunk := currentChunk
		if potentialChunk != "" {
			potentialChunk += " "
		}
		potentialChunk += sentence

		if utf8.RuneCountInString(potentialChunk) > c.config.MaxChunkSize ||
			sentenceCount >= c.config.SentenceWindow {
			// Save current chunk
			if currentChunk != "" && utf8.RuneCountInString(currentChunk) >= c.config.MinChunkSize {
				chunk := &DocumentChunk{
					Text:  strings.TrimSpace(currentChunk),
					Index: len(chunks),
				}
				chunks = append(chunks, chunk)
			}
			currentChunk = sentence
			sentenceCount = 1
		} else {
			currentChunk = potentialChunk
			sentenceCount++
		}
	}

	// Save last chunk
	if currentChunk != "" && utf8.RuneCountInString(currentChunk) >= c.config.MinChunkSize {
		chunk := &DocumentChunk{
			Text:  strings.TrimSpace(currentChunk),
			Index: len(chunks),
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

// adaptiveChunking adaptive chunking
func (c *DocumentChunker) adaptiveChunking(content string) []*DocumentChunk {
	// Choose strategy based on document length
	docLength := utf8.RuneCountInString(content)

	switch {
	case docLength < 500:
		// Very short document, use as a single chunk
		return []*DocumentChunk{{
			Text:  content,
			Index: 0,
		}}
	case docLength < 2000:
		// Short document, use sentence chunking
		return c.sentenceChunking(content)
	case docLength < 10000:
		// Medium document, use semantic chunking
		return c.semanticChunking(content)
	default:
		// Long document, use hybrid strategy
		return c.hybridChunking(content)
	}
}

// hybridChunking hybrid chunking strategy
func (c *DocumentChunker) hybridChunking(content string) []*DocumentChunk {
	// First try to split by sections
	sections := c.splitIntoSections(content)
	var chunks []*DocumentChunk

	for _, section := range sections {
		if utf8.RuneCountInString(section) <= c.config.MaxChunkSize {
			// Section size is appropriate, use as a single chunk
			if utf8.RuneCountInString(section) >= c.config.MinChunkSize {
				chunk := &DocumentChunk{
					Text:  strings.TrimSpace(section),
					Index: len(chunks),
				}
				chunks = append(chunks, chunk)
			}
		} else {
			// Section is too large, perform semantic chunking
			sectionChunks := c.semanticChunking(section)
			for _, chunk := range sectionChunks {
				chunk.Index = len(chunks)
				chunks = append(chunks, chunk)
			}
		}
	}

	return chunks
}

// isSentenceBoundary determines if it's a sentence boundary
func (c *DocumentChunker) isSentenceBoundary(r rune) bool {
	return r == '。' || r == '！' || r == '？' || r == '.' || r == '!' || r == '?' || r == '\n'
}

// splitIntoParagraphs splits into paragraphs
func (c *DocumentChunker) splitIntoParagraphs(content string) []string {
	paragraphs := strings.Split(content, "\n\n")
	var result []string
	for _, p := range paragraphs {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// splitIntoSentences splits into sentences
func (c *DocumentChunker) splitIntoSentences(content string) []string {
	// Use GSE for sentence segmentation
	sentences := c.segmenter.CutAll(content)
	var result []string
	currentSentence := ""

	for _, word := range sentences {
		currentSentence += word
		// Check if it's sentence end
		if len(word) > 0 {
			lastRune := []rune(word)[len([]rune(word))-1]
			if c.isSentenceBoundary(lastRune) {
				if strings.TrimSpace(currentSentence) != "" {
					result = append(result, strings.TrimSpace(currentSentence))
				}
				currentSentence = ""
			}
		}
	}

	// Add last sentence
	if strings.TrimSpace(currentSentence) != "" {
		result = append(result, strings.TrimSpace(currentSentence))
	}

	return result
}

// splitIntoSections splits into sections
func (c *DocumentChunker) splitIntoSections(content string) []string {
	// Use header pattern to split sections
	headerPattern := regexp.MustCompile(`(?m)^#{1,6}\s+.+$|^.+\n[=-]+\s*$`)
	indices := headerPattern.FindAllStringIndex(content, -1)

	if len(indices) == 0 {
		// No headers found, split by paragraphs
		return c.splitIntoParagraphs(content)
	}

	var sections []string
	lastEnd := 0

	for i, match := range indices {
		if i > 0 {
			// Add previous section
			section := strings.TrimSpace(content[lastEnd:match[0]])
			if section != "" {
				sections = append(sections, section)
			}
		}
		lastEnd = match[0]
	}

	// Add last section
	if lastEnd < len(content) {
		section := strings.TrimSpace(content[lastEnd:])
		if section != "" {
			sections = append(sections, section)
		}
	}

	return sections
}

// splitLongParagraph splits long paragraphs
func (c *DocumentChunker) splitLongParagraph(paragraph string) []*DocumentChunk {
	sentences := c.splitIntoSentences(paragraph)
	var chunks []*DocumentChunk
	currentChunk := ""

	for _, sentence := range sentences {
		potentialChunk := currentChunk
		if potentialChunk != "" {
			potentialChunk += " "
		}
		potentialChunk += sentence

		if utf8.RuneCountInString(potentialChunk) > c.config.MaxChunkSize {
			// Save current chunk
			if currentChunk != "" && utf8.RuneCountInString(currentChunk) >= c.config.MinChunkSize {
				chunk := &DocumentChunk{
					Text:  strings.TrimSpace(currentChunk),
					Index: len(chunks),
				}
				chunks = append(chunks, chunk)
			}
			currentChunk = sentence
		} else {
			currentChunk = potentialChunk
		}
	}

	// Save last chunk
	if currentChunk != "" && utf8.RuneCountInString(currentChunk) >= c.config.MinChunkSize {
		chunk := &DocumentChunk{
			Text:  strings.TrimSpace(currentChunk),
			Index: len(chunks),
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

// ChunkMarkdownFile processes Markdown file and generates chunks.json
func ChunkMarkdownFile(markdownPath, configPath, outputPath string) error {
	// Load configuration
	config, err := LoadChunkingConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	cwd, err := os.Getwd()
	if err != nil {
		log.Panic().Err(err).Stack().Msg("Getwd() failed")
	}

	// Create chunker
	chunker, err := NewDocumentChunker(config, cwd)
	if err != nil {
		return fmt.Errorf("failed to create chunker: %w", err)
	}

	// Read Markdown file
	content, err := os.ReadFile(markdownPath)
	if err != nil {
		return fmt.Errorf("failed to read markdown file: %w", err)
	}

	// Perform chunking
	fileName := filepath.Base(markdownPath)
	doc, err := chunker.ChunkDocument(string(content), fileName)
	if err != nil {
		return fmt.Errorf("failed to chunk document: %w", err)
	}

	// Output JSON
	jsonData, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	err = os.WriteFile(outputPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write output file: %w", err)
	}

	log.Info().Str("input", markdownPath).Str("output", outputPath).Int("chunks", len(doc.Chunks)).Msg("Document chunked successfully")
	return nil
}
