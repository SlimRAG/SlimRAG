package rag

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/rs/zerolog/log"
)

const eol = "<EOL>"
const ready = "<READY>"
const exit = "<EXIT>"

type Chunker struct {
	cmd *exec.Cmd
	r   *bufio.Scanner
	w   io.WriteCloser
}

func NewChunker() (c *Chunker, err error) {
	c = &Chunker{
		cmd: exec.Command("./.venv/bin/python3", "main.py", "repr"),
	}

	err = c.start()
	if err != nil {
		return
	}

	return
}

func (c *Chunker) start() (err error) {
	c.w, err = c.cmd.StdinPipe()
	if err != nil {
		return
	}

	var r io.Reader
	r, err = c.cmd.StdoutPipe()
	if err != nil {
		return
	}
	c.r = bufio.NewScanner(r)

	c.cmd.Stderr = os.Stderr

	err = c.cmd.Start()
	if err != nil {
		return
	}

	// wait for the script ready
	log.Info().Msg("Waiting for chunker start")
	for c.r.Scan() {
		line := c.r.Text()
		if line == ready {
			break
		}
	}
	log.Info().Msg("External chunker started")
	return
}

type Chunk struct {
	Text       string `json:"text"`
	TokenCount int    `json:"token_count"`
}

func (c *Chunker) Split(s string) (chunks []Chunk, err error) {
	_, err = fmt.Fprintln(c.w, s)
	if err != nil {
		return
	}
	_, err = fmt.Fprintln(c.w, eol)
	if err != nil {
		return
	}

	chunks = make([]Chunk, 0)

	for c.r.Scan() {
		line := c.r.Text()
		if line == eol {
			return
		}

		var chunk Chunk
		decoder := json.NewDecoder(strings.NewReader(line))
		decoder.DisallowUnknownFields()
		err = decoder.Decode(&chunk)
		if err != nil {
			return
		}
		chunks = append(chunks, chunk)
	}

	return
}

func (c *Chunker) Close() (err error) {
	_, err = fmt.Fprintln(c.w, exit)
	if err != nil {
		return
	}

	err = c.cmd.Wait()
	return
}
