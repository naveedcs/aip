package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"testing"
	"time"

	"github.com/naveedcs/aip/internal/profile"
)

func TestHandshakeParsesServerInfo(t *testing.T) {
	clientReader, clientWriter := io.Pipe()
	serverReader, serverWriter := io.Pipe()
	t.Cleanup(func() {
		_ = clientReader.Close()
		_ = clientWriter.Close()
		_ = serverReader.Close()
		_ = serverWriter.Close()
	})

	done := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(clientReader).ReadString('\n')
		if err != nil {
			done <- err
			return
		}
		var req struct {
			ID     int    `json:"id"`
			Method string `json:"method"`
		}
		if err := json.Unmarshal([]byte(line), &req); err != nil {
			done <- err
			return
		}
		if req.ID != 1 || req.Method != "initialize" {
			done <- io.ErrUnexpectedEOF
			return
		}
		_, err = serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","serverInfo":{"name":"fake","version":"9.9"}}}` + "\n"))
		done <- err
	}()

	got, err := Handshake(context.Background(), clientWriter, serverReader, ClientInfo{Name: "aip", Version: "1.2"})
	if err != nil {
		t.Fatalf("Handshake returned error: %v", err)
	}
	if got.ProtocolVersion != "2025-06-18" || got.ServerName != "fake" || got.ServerVersion != "9.9" {
		t.Fatalf("Handshake = %#v, want protocolVersion 2025-06-18 server fake 9.9", got)
	}
	if err := <-done; err != nil {
		t.Fatalf("server goroutine returned error: %v", err)
	}
}

func TestHandshakeReturnsServerError(t *testing.T) {
	clientReader, clientWriter := io.Pipe()
	serverReader, serverWriter := io.Pipe()
	t.Cleanup(func() {
		_ = clientReader.Close()
		_ = clientWriter.Close()
		_ = serverReader.Close()
		_ = serverWriter.Close()
	})

	done := make(chan error, 1)
	go func() {
		if _, err := bufio.NewReader(clientReader).ReadString('\n'); err != nil {
			done <- err
			return
		}
		_, err := serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":1,"error":{"code":-32000,"message":"boom"}}` + "\n"))
		done <- err
	}()

	_, err := Handshake(context.Background(), clientWriter, serverReader, ClientInfo{Name: "aip", Version: "1.2"})
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("Handshake error = %v, want error containing boom", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("server goroutine returned error: %v", err)
	}
}

func TestHandshakeHonorsContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := Handshake(ctx, io.Discard, blockingReader{}, ClientInfo{Name: "aip", Version: "1.2"})
	if err == nil {
		t.Fatal("Handshake error = nil, want non-nil error")
	}
}

func TestHandshakeHonorsContextCancellationAfterWriteWhileReading(t *testing.T) {
	clientReader, clientWriter := io.Pipe()
	serverReader, serverWriter := io.Pipe()
	t.Cleanup(func() {
		_ = clientReader.Close()
		_ = clientWriter.Close()
		_ = serverReader.Close()
		_ = serverWriter.Close()
	})

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() {
		_, err := Handshake(ctx, clientWriter, serverReader, ClientInfo{Name: "aip", Version: "1.2"})
		done <- err
	}()

	if _, err := bufio.NewReader(clientReader).ReadString('\n'); err != nil {
		t.Fatalf("reading initialize request returned error: %v", err)
	}
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Handshake error = nil, want non-nil error")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Handshake did not return promptly after context cancellation")
	}

	writeDone := make(chan error, 1)
	go func() {
		_, err := serverWriter.Write([]byte(`{"jsonrpc":"2.0","id":1,"result":{}}` + "\n"))
		writeDone <- err
	}()
	select {
	case err := <-writeDone:
		if err == nil {
			t.Fatal("response write error = nil, want closed pipe error")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("response writer remained blocked after context cancellation")
	}
}

func TestHandshakeHonorsContextCancellationWhileWriting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	writer := newBlockingWriteCloser()

	done := make(chan error, 1)
	go func() {
		_, err := Handshake(ctx, writer, blockingReader{}, ClientInfo{Name: "aip", Version: "1.2"})
		done <- err
	}()

	select {
	case <-writer.entered:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Handshake did not start writing promptly")
	}
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Fatal("Handshake error = nil, want non-nil error")
		}
	case <-time.After(200 * time.Millisecond):
		t.Fatal("Handshake did not return promptly while write was blocked")
	}
}

func TestHandshakeRejectsInvalidJSONRPCVersion(t *testing.T) {
	clientReader, clientWriter := io.Pipe()
	serverReader, serverWriter := io.Pipe()
	t.Cleanup(func() {
		_ = clientReader.Close()
		_ = clientWriter.Close()
		_ = serverReader.Close()
		_ = serverWriter.Close()
	})

	done := make(chan error, 1)
	go func() {
		if _, err := bufio.NewReader(clientReader).ReadString('\n'); err != nil {
			done <- err
			return
		}
		_, err := serverWriter.Write([]byte(`{"jsonrpc":"1.0","id":1,"result":{"protocolVersion":"2025-06-18","serverInfo":{"name":"fake","version":"9.9"}}}` + "\n"))
		done <- err
	}()

	_, err := Handshake(context.Background(), clientWriter, serverReader, ClientInfo{Name: "aip", Version: "1.2"})
	if err == nil || !strings.Contains(err.Error(), "jsonrpc") {
		t.Fatalf("Handshake error = %v, want error containing jsonrpc", err)
	}
	if err := <-done; err != nil {
		t.Fatalf("server goroutine returned error: %v", err)
	}
}

func TestHandshakeRejectsTooManyIgnoredResponses(t *testing.T) {
	var responses strings.Builder
	for i := 0; i < 1100; i++ {
		responses.WriteString("server log line\n")
	}

	_, err := Handshake(context.Background(), io.Discard, strings.NewReader(responses.String()), ClientInfo{Name: "aip", Version: "1.2"})
	if err == nil || !strings.Contains(err.Error(), "too many non-initialize responses") {
		t.Fatalf("Handshake error = %v, want error containing too many non-initialize responses", err)
	}
}

func TestHandshakeIgnoresUnrelatedResponsesBeforeInitializeResult(t *testing.T) {
	responses := strings.Join([]string{
		"server log line",
		`{"jsonrpc":"2.0","id":99,"result":{}}`,
		`{"jsonrpc":"2.0","method":"notifications/initialized","params":{}}`,
		`{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","serverInfo":{"name":"fake","version":"9.9"}}}`,
		"",
	}, "\n")

	got, err := Handshake(context.Background(), io.Discard, strings.NewReader(responses), ClientInfo{Name: "aip", Version: "1.2"})
	if err != nil {
		t.Fatalf("Handshake returned error: %v", err)
	}
	if got.ProtocolVersion != "2025-06-18" || got.ServerName != "fake" || got.ServerVersion != "9.9" {
		t.Fatalf("Handshake = %#v, want protocolVersion 2025-06-18 server fake 9.9", got)
	}
}

func TestProbeCleansUpChildProcessGroup(t *testing.T) {
	dir := t.TempDir()
	script := filepath.Join(dir, "fake-mcp")
	if err := os.WriteFile(script, []byte(`#!/bin/sh
read line
sh -c 'trap "" TERM HUP INT; sleep 30' &
child=$!
printf '{"jsonrpc":"2.0","id":1,"result":{"protocolVersion":"2025-06-18","serverInfo":{"name":"fake","version":"%s"}}}\n' "$child"
wait "$child"
`), 0o755); err != nil {
		t.Fatalf("WriteFile fake MCP server returned error: %v", err)
	}

	done := make(chan probeResult, 1)
	go func() {
		result, err := Probe(context.Background(), profile.MCPServer{
			Command: script,
		}, nil, ClientInfo{Name: "aip", Version: "1.2"})
		done <- probeResult{result: result, err: err}
	}()

	var got probeResult
	select {
	case got = <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("Probe did not return promptly after initialize response")
	}
	if got.err != nil {
		t.Fatalf("Probe returned error: %v", got.err)
	}
	if got.result.ServerName != "fake" {
		t.Fatalf("Probe result = %#v", got.result)
	}
	childPID, err := strconv.Atoi(got.result.ServerVersion)
	if err != nil {
		t.Fatalf("server version child PID = %q, want integer PID: %v", got.result.ServerVersion, err)
	}
	t.Cleanup(func() {
		_ = syscall.Kill(childPID, syscall.SIGKILL)
	})

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if !processExists(childPID) {
			return
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatalf("child process %d still exists after Probe cleanup", childPID)
}

type probeResult struct {
	result ProbeResult
	err    error
}

func processExists(pid int) bool {
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

type blockingReader struct{}

func (blockingReader) Read([]byte) (int, error) {
	select {}
}

type blockingWriteCloser struct {
	entered chan struct{}
	closed  chan struct{}
	once    sync.Once
}

func newBlockingWriteCloser() *blockingWriteCloser {
	return &blockingWriteCloser{
		entered: make(chan struct{}),
		closed:  make(chan struct{}),
	}
}

func (w *blockingWriteCloser) Write([]byte) (int, error) {
	w.once.Do(func() {
		close(w.entered)
	})
	<-w.closed
	return 0, io.ErrClosedPipe
}

func (w *blockingWriteCloser) Close() error {
	select {
	case <-w.closed:
	default:
		close(w.closed)
	}
	return nil
}
