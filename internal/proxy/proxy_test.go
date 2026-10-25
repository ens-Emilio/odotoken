package proxy

import (
	"io"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ens-Emilio/odotoken/internal/parser"
)

type gatedBody struct {
	firstSent chan struct{}
	release   chan struct{}
	reader    *strings.Reader
	sent      sync.Once
}

func (b *gatedBody) Read(p []byte) (int, error) {
	if b.reader.Len() == 0 {
		return 0, io.EOF
	}
	b.sent.Do(func() {
		close(b.firstSent)
		<-b.release
	})
	return b.reader.Read(p)
}

func (b *gatedBody) Close() error { return nil }

func TestCountingBodyDoesNotBufferStream(t *testing.T) {
	first := "data: {\"model\":\"gpt-test\",\"usage\":{\"prompt_tokens\":10,\"completion_tokens\":20,\"total_tokens\":30}}\n"
	body := &gatedBody{
		firstSent: make(chan struct{}),
		release:   make(chan struct{}),
		reader:    strings.NewReader(first + "data: [DONE]\n"),
	}
	counting := newCountingReadCloser(body, parser.NewStreamAccumulator("openai", ""))
	buf := make([]byte, len(first))

	readDone := make(chan error, 1)
	go func() {
		_, err := io.ReadFull(counting, buf)
		readDone <- err
	}()

	select {
	case <-body.firstSent:
		// The first Read reached upstream. If the wrapper buffered the whole
		// stream, it would block here waiting for release and never return.
	case <-time.After(time.Second):
		t.Fatal("first read never reached upstream")
	}
	close(body.release)
	select {
	case err := <-readDone:
		if err != nil {
			t.Fatalf("read first chunk: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("first chunk was retained until stream completion")
	}
	if string(buf) != first {
		t.Fatalf("first chunk = %q, want %q", buf, first)
	}

	_, _ = io.ReadAll(counting)
	if err := counting.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
}
