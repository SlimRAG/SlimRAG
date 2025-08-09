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

// ChunkingConfig 分块配置
type ChunkingConfig struct {
	MaxChunkSize        int     `json:"max_chunk_size"`       // 最大块大小（字符数）
	MinChunkSize        int     `json:"min_chunk_size"`       // 最小块大小（字符数）
	OverlapSize         int     `json:"overlap_size"`         // 重叠大小（字符数）
	SentenceWindow      int     `json:"sentence_window"`      // 句子窗口大小
	Strategy            string  `json:"strategy"`             // 分块策略: "fixed", "semantic", "sentence", "adaptive"
	Language            string  `json:"language"`             // 语言: "zh", "en", "auto"
	PreserveSections    bool    `json:"preserve_sections"`    // 是否保留章节结构
	SimilarityThreshold float64 `json:"similarity_threshold"` // 语义相似度阈值
}

// DefaultChunkingConfig 默认配置
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

// LoadChunkingConfig 从JSON文件加载配置
func LoadChunkingConfig(configPath string) (*ChunkingConfig, error) {
	if configPath == "" {
		return DefaultChunkingConfig(), nil
	}

	file, err := os.Open(configPath)
	if err != nil {
		return DefaultChunkingConfig(), nil // 使用默认配置
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

	// 验证配置
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

// DocumentChunker 文档分块器
type DocumentChunker struct {
	config    *ChunkingConfig
	segmenter *gse.Segmenter
}

// NewDocumentChunker 创建新的文档分块器
func NewDocumentChunker(config *ChunkingConfig) (*DocumentChunker, error) {
	if config == nil {
		config = DefaultChunkingConfig()
	}

	chunker := &DocumentChunker{
		config:    config,
		segmenter: &gse.Segmenter{},
	}

	// 根据语言配置分词器
	switch config.Language {
	case "zh":
		chunker.segmenter.LoadDict("zh_s")
	case "en":
		chunker.segmenter.LoadDict()
	default:
		// 自动检测，默认加载中文词典
		chunker.segmenter.LoadDict("zh_s")
	}

	return chunker, nil
}

// ChunkDocument 对文档进行分块
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

// preprocessText 预处理文本
func (c *DocumentChunker) preprocessText(text string) string {
	// 移除多余的空白字符
	text = regexp.MustCompile(`\s+`).ReplaceAllString(text, " ")
	// 移除空字符
	text = strings.ReplaceAll(text, "\u0000", "")
	// 标准化换行符
	text = strings.ReplaceAll(text, "\r\n", "\n")
	text = strings.ReplaceAll(text, "\r", "\n")
	// 移除多余的换行符
	text = regexp.MustCompile(`\n{3,}`).ReplaceAllString(text, "\n\n")

	return strings.TrimSpace(text)
}

// fixedSizeChunking 固定大小分块
func (c *DocumentChunker) fixedSizeChunking(content string) []*DocumentChunk {
	var chunks []*DocumentChunk
	runes := []rune(content)
	totalLen := len(runes)

	for i := 0; i < totalLen; {
		end := i + c.config.MaxChunkSize
		if end > totalLen {
			end = totalLen
		}

		// 尝试在句子边界处分割
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

		// 计算下一个起始位置（考虑重叠）
		nextStart := end - c.config.OverlapSize
		if nextStart <= i {
			nextStart = i + 1
		}
		i = nextStart
	}

	return chunks
}

// semanticChunking 语义分块
func (c *DocumentChunker) semanticChunking(content string) []*DocumentChunk {
	// 先按段落分割
	paragraphs := c.splitIntoParagraphs(content)
	var chunks []*DocumentChunk
	currentChunk := ""

	for _, paragraph := range paragraphs {
		// 如果当前段落太长，需要进一步分割
		if utf8.RuneCountInString(paragraph) > c.config.MaxChunkSize {
			// 保存当前块
			if currentChunk != "" {
				chunk := &DocumentChunk{
					Text:  strings.TrimSpace(currentChunk),
					Index: len(chunks),
				}
				chunks = append(chunks, chunk)
				currentChunk = ""
			}
			// 对长段落进行句子级分割
			sentenceChunks := c.splitLongParagraph(paragraph)
			chunks = append(chunks, sentenceChunks...)
			continue
		}

		// 检查添加当前段落是否会超过最大大小
		potentialChunk := currentChunk
		if potentialChunk != "" {
			potentialChunk += "\n\n"
		}
		potentialChunk += paragraph

		if utf8.RuneCountInString(potentialChunk) > c.config.MaxChunkSize {
			// 保存当前块
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

	// 保存最后一个块
	if currentChunk != "" && utf8.RuneCountInString(currentChunk) >= c.config.MinChunkSize {
		chunk := &DocumentChunk{
			Text:  strings.TrimSpace(currentChunk),
			Index: len(chunks),
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

// sentenceChunking 句子级分块
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
			// 保存当前块
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

	// 保存最后一个块
	if currentChunk != "" && utf8.RuneCountInString(currentChunk) >= c.config.MinChunkSize {
		chunk := &DocumentChunk{
			Text:  strings.TrimSpace(currentChunk),
			Index: len(chunks),
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

// adaptiveChunking 自适应分块
func (c *DocumentChunker) adaptiveChunking(content string) []*DocumentChunk {
	// 根据文档长度选择策略
	docLength := utf8.RuneCountInString(content)

	switch {
	case docLength < 500:
		// 很短的文档，直接作为一个块
		return []*DocumentChunk{{
			Text:  content,
			Index: 0,
		}}
	case docLength < 2000:
		// 短文档，使用句子分块
		return c.sentenceChunking(content)
	case docLength < 10000:
		// 中等文档，使用语义分块
		return c.semanticChunking(content)
	default:
		// 长文档，使用混合策略
		return c.hybridChunking(content)
	}
}

// hybridChunking 混合分块策略
func (c *DocumentChunker) hybridChunking(content string) []*DocumentChunk {
	// 首先尝试按章节分割
	sections := c.splitIntoSections(content)
	var chunks []*DocumentChunk

	for _, section := range sections {
		if utf8.RuneCountInString(section) <= c.config.MaxChunkSize {
			// 章节大小合适，直接作为一个块
			if utf8.RuneCountInString(section) >= c.config.MinChunkSize {
				chunk := &DocumentChunk{
					Text:  strings.TrimSpace(section),
					Index: len(chunks),
				}
				chunks = append(chunks, chunk)
			}
		} else {
			// 章节太大，进行语义分块
			sectionChunks := c.semanticChunking(section)
			for _, chunk := range sectionChunks {
				chunk.Index = len(chunks)
				chunks = append(chunks, chunk)
			}
		}
	}

	return chunks
}

// isSentenceBoundary 判断是否为句子边界
func (c *DocumentChunker) isSentenceBoundary(r rune) bool {
	return r == '。' || r == '！' || r == '？' || r == '.' || r == '!' || r == '?' || r == '\n'
}

// splitIntoParagraphs 分割段落
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

// splitIntoSentences 分割句子
func (c *DocumentChunker) splitIntoSentences(content string) []string {
	// 使用GSE进行句子分割
	sentences := c.segmenter.CutAll(content)
	var result []string
	currentSentence := ""

	for _, word := range sentences {
		currentSentence += word
		// 检查是否为句子结束
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

	// 添加最后一个句子
	if strings.TrimSpace(currentSentence) != "" {
		result = append(result, strings.TrimSpace(currentSentence))
	}

	return result
}

// splitIntoSections 分割章节
func (c *DocumentChunker) splitIntoSections(content string) []string {
	// 使用标题模式分割章节
	headerPattern := regexp.MustCompile(`(?m)^#{1,6}\s+.+$|^.+\n[=-]+\s*$`)
	indices := headerPattern.FindAllStringIndex(content, -1)

	if len(indices) == 0 {
		// 没有找到标题，按段落分割
		return c.splitIntoParagraphs(content)
	}

	var sections []string
	lastEnd := 0

	for i, match := range indices {
		if i > 0 {
			// 添加前一个章节
			section := strings.TrimSpace(content[lastEnd:match[0]])
			if section != "" {
				sections = append(sections, section)
			}
		}
		lastEnd = match[0]
	}

	// 添加最后一个章节
	if lastEnd < len(content) {
		section := strings.TrimSpace(content[lastEnd:])
		if section != "" {
			sections = append(sections, section)
		}
	}

	return sections
}

// splitLongParagraph 分割长段落
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
			// 保存当前块
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

	// 保存最后一个块
	if currentChunk != "" && utf8.RuneCountInString(currentChunk) >= c.config.MinChunkSize {
		chunk := &DocumentChunk{
			Text:  strings.TrimSpace(currentChunk),
			Index: len(chunks),
		}
		chunks = append(chunks, chunk)
	}

	return chunks
}

// ChunkMarkdownFile 处理Markdown文件并生成chunks.json
func ChunkMarkdownFile(markdownPath, configPath, outputPath string) error {
	// 加载配置
	config, err := LoadChunkingConfig(configPath)
	if err != nil {
		return fmt.Errorf("failed to load config: %w", err)
	}

	// 创建分块器
	chunker, err := NewDocumentChunker(config)
	if err != nil {
		return fmt.Errorf("failed to create chunker: %w", err)
	}

	// 读取Markdown文件
	content, err := os.ReadFile(markdownPath)
	if err != nil {
		return fmt.Errorf("failed to read markdown file: %w", err)
	}

	// 进行分块
	fileName := filepath.Base(markdownPath)
	doc, err := chunker.ChunkDocument(string(content), fileName)
	if err != nil {
		return fmt.Errorf("failed to chunk document: %w", err)
	}

	// 输出JSON
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
