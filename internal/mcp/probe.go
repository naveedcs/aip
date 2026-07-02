package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/naveedcs/aip/internal/profile"
)

const mcpProtocolVersion = "2025-06-18"
const maxInitializeResponseLine = 1 << 20
const maxIgnoredInitializeResponseLines = 1024

type ClientInfo struct {
	Name    string
	Version string
}

type ProbeResult struct {
	ProtocolVersion string
	ServerName      string
	ServerVersion   string
}

func Probe(ctx context.Context, server profile.MCPServer, env map[string]string, client ClientInfo) (ProbeResult, error) {
	cmd := exec.CommandContext(ctx, server.Command, server.Args...)
	cmd.Env = mergedEnv(env)
	cmd.Stderr = io.Discard
	cmd.SysProcAttr = probeSysProcAttr()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return ProbeResult{}, fmt.Errorf("open stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return ProbeResult{}, fmt.Errorf("open stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return ProbeResult{}, fmt.Errorf("start MCP server: %w", err)
	}
	defer func() {
		cleanupProcessGroup(cmd, stdin, stdout)
	}()

	return Handshake(ctx, stdin, stdout, client)
}

func mergedEnv(env map[string]string) []string {
	values := map[string]string{}
	for _, entry := range os.Environ() {
		key, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		values[key] = value
	}
	for _, key := range sortedStringKeys(env) {
		values[key] = env[key]
	}
	out := make([]string, 0, len(values))
	for _, key := range sortedStringKeys(values) {
		out = append(out, key+"="+values[key])
	}
	return out
}

func Handshake(ctx context.Context, w io.Writer, r io.Reader, client ClientInfo) (ProbeResult, error) {
	if err := ctx.Err(); err != nil {
		return ProbeResult{}, err
	}

	req := initializeRequest{
		JSONRPC: "2.0",
		ID:      1,
		Method:  "initialize",
		Params: initializeParams{
			ProtocolVersion: mcpProtocolVersion,
			Capabilities:    map[string]any{},
			ClientInfo: initializeClientInfo{
				Name:    client.Name,
				Version: client.Version,
			},
		},
	}
	data, err := json.Marshal(req)
	if err != nil {
		return ProbeResult{}, err
	}
	data = append(data, '\n')

	writeDone := make(chan error, 1)
	go func() {
		_, err := w.Write(data)
		writeDone <- err
	}()

	select {
	case <-ctx.Done():
		closeIfPossible(w)
		closeIfPossible(r)
		return ProbeResult{}, ctx.Err()
	case err := <-writeDone:
		if err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ProbeResult{}, ctxErr
			}
			return ProbeResult{}, err
		}
	}

	responses := make(chan handshakeResponse, 1)
	go func() {
		responses <- readInitializeResponse(r)
	}()

	select {
	case <-ctx.Done():
		closeIfPossible(w)
		closeIfPossible(r)
		return ProbeResult{}, ctx.Err()
	case response := <-responses:
		if response.err != nil {
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ProbeResult{}, ctxErr
			}
		}
		return response.result, response.err
	}
}

func closeIfPossible(v any) {
	closer, ok := v.(io.Closer)
	if ok {
		_ = closer.Close()
	}
}

type initializeRequest struct {
	JSONRPC string           `json:"jsonrpc"`
	ID      int              `json:"id"`
	Method  string           `json:"method"`
	Params  initializeParams `json:"params"`
}

type initializeParams struct {
	ProtocolVersion string               `json:"protocolVersion"`
	Capabilities    map[string]any       `json:"capabilities"`
	ClientInfo      initializeClientInfo `json:"clientInfo"`
}

type initializeClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

type handshakeResponse struct {
	result ProbeResult
	err    error
}

func readInitializeResponse(r io.Reader) handshakeResponse {
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 0, 64*1024), maxInitializeResponseLine)
	ignored := 0
	for scanner.Scan() {
		if response, ok := parseInitializeResponse(scanner.Bytes()); ok {
			return response
		}
		ignored++
		if ignored >= maxIgnoredInitializeResponseLines {
			return handshakeResponse{err: fmt.Errorf("too many non-initialize responses")}
		}
	}
	if err := scanner.Err(); err != nil {
		return handshakeResponse{err: fmt.Errorf("read initialize response: %w", err)}
	}
	return handshakeResponse{err: io.EOF}
}

func parseInitializeResponse(data []byte) (handshakeResponse, bool) {
	var response struct {
		JSONRPC string `json:"jsonrpc"`
		ID      *int   `json:"id"`
		Result  *struct {
			ProtocolVersion string `json:"protocolVersion"`
			ServerInfo      struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		} `json:"result"`
		Error *struct {
			Code    int    `json:"code"`
			Message string `json:"message"`
		} `json:"error"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		return handshakeResponse{}, false
	}
	if response.ID == nil || *response.ID != 1 {
		return handshakeResponse{}, false
	}
	if response.JSONRPC != "2.0" {
		return handshakeResponse{err: fmt.Errorf("initialize response has invalid jsonrpc version %q", response.JSONRPC)}, true
	}
	if response.Error != nil {
		return handshakeResponse{err: fmt.Errorf("initialize failed: %s", response.Error.Message)}, true
	}
	if response.Result == nil {
		return handshakeResponse{err: fmt.Errorf("initialize response missing result")}, true
	}
	return handshakeResponse{
		result: ProbeResult{
			ProtocolVersion: response.Result.ProtocolVersion,
			ServerName:      response.Result.ServerInfo.Name,
			ServerVersion:   response.Result.ServerInfo.Version,
		},
	}, true
}
