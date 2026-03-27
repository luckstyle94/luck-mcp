package mcp

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strconv"
	"strings"
	"sync"

	"luck-mpc/internal/service"
)

const (
	jsonRPCVersion = "2.0"
	protocolVer    = "2024-11-05"
)

type Server struct {
	context  *service.ContextService
	codebase *service.CodebaseService
	logger   *slog.Logger
	name     string
	version  string

	reader *bufio.Reader
	writer io.Writer
	mu     sync.Mutex
	mode   messageMode
}

type messageMode int

const (
	modeUnknown messageMode = iota
	modeFramed
	modeLineDelimited
)

func NewServer(
	contextSvc *service.ContextService,
	codebaseSvc *service.CodebaseService,
	logger *slog.Logger,
	name string,
	version string,
	stdin io.Reader,
	stdout io.Writer,
) *Server {
	if logger == nil {
		logger = slog.Default()
	}
	if name == "" {
		name = "context-memory-mcp"
	}
	if version == "" {
		version = "0.1.0"
	}
	return &Server{
		context:  contextSvc,
		codebase: codebaseSvc,
		logger:   logger,
		name:     name,
		version:  version,
		reader:   bufio.NewReader(stdin),
		writer:   stdout,
	}
}

func (s *Server) Run(ctx context.Context) error {
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		payload, err := s.readMessage()
		if err != nil {
			if errors.Is(err, io.EOF) {
				return nil
			}
			return fmt.Errorf("read rpc message: %w", err)
		}

		var req rpcRequest
		if err := json.Unmarshal(payload, &req); err != nil {
			s.logger.Error("invalid json-rpc payload", slog.String("error", err.Error()))
			_ = s.writeResponse(rpcResponse{
				JSONRPC: jsonRPCVersion,
				Error: &rpcErrorObject{
					Code:    -32700,
					Message: "parse error",
				},
			})
			continue
		}

		if req.JSONRPC != "" && req.JSONRPC != jsonRPCVersion {
			s.writeRPCError(req.ID, -32600, "invalid jsonrpc version")
			continue
		}

		if len(req.ID) == 0 {
			s.handleNotification(ctx, req)
			continue
		}

		res := s.handleRequest(ctx, req)
		if err := s.writeResponse(res); err != nil {
			return fmt.Errorf("write rpc response: %w", err)
		}
	}
}

func (s *Server) handleNotification(ctx context.Context, req rpcRequest) {
	switch req.Method {
	case "notifications/initialized", "initialized":
		s.logger.Debug("mcp initialized notification received")
	case "$/cancelRequest":
		s.logger.Debug("cancel request notification received")
	default:
		s.logger.Debug("ignored notification", slog.String("method", req.Method))
	}
	_ = ctx
}

func (s *Server) handleRequest(ctx context.Context, req rpcRequest) rpcResponse {
	switch req.Method {
	case "initialize":
		var params initializeParams
		if err := decodeJSONLoose(req.Params, &params); err != nil {
			return invalidParams(req.ID, err)
		}
		selectedVersion := strings.TrimSpace(params.ProtocolVersion)
		if selectedVersion == "" {
			selectedVersion = protocolVer
		}
		result := initializeResult{ProtocolVersion: selectedVersion}
		result.Capabilities.Tools.ListChanged = false
		result.ServerInfo.Name = s.name
		result.ServerInfo.Version = s.version
		return rpcResponse{JSONRPC: jsonRPCVersion, ID: req.ID, Result: result}
	case "tools/list", "list_tools":
		return rpcResponse{JSONRPC: jsonRPCVersion, ID: req.ID, Result: toolsListResult{Tools: toolDefinitions()}}
	case "tools/call", "call_tool":
		return s.handleToolCall(ctx, req)
	case "ping":
		return rpcResponse{JSONRPC: jsonRPCVersion, ID: req.ID, Result: map[string]any{}}
	default:
		return rpcResponse{
			JSONRPC: jsonRPCVersion,
			ID:      req.ID,
			Error: &rpcErrorObject{
				Code:    -32601,
				Message: "method not found",
			},
		}
	}
}

func (s *Server) handleToolCall(ctx context.Context, req rpcRequest) rpcResponse {
	var params toolCallParams
	if err := decodeJSONLoose(req.Params, &params); err != nil {
		return invalidParams(req.ID, err)
	}

	toolName := strings.TrimSpace(params.Name)
	if toolName == "" {
		toolName = strings.TrimSpace(params.Tool)
	}
	if toolName == "" {
		return invalidParams(req.ID, errors.New("tool name is required"))
	}

	args := params.Arguments
	if len(args) == 0 {
		args = params.Args
	}
	if len(args) == 0 {
		args = []byte("{}")
	}

	s.logger.Debug("tool call",
		slog.String("tool", toolName),
	)

	switch toolName {
	case "repo_list":
		repos, err := s.codebase.ListRepos(ctx)
		if err != nil {
			return toolExecutionResponse(req.ID, map[string]string{"error": err.Error()}, true)
		}
		return toolExecutionResponse(req.ID, map[string]any{"repos": repos}, false)

	case "repo_register":
		var in service.RepoRegisterInput
		if err := decodeJSONStrict(args, &in); err != nil {
			return invalidParams(req.ID, err)
		}
		repo, err := s.codebase.RegisterRepo(ctx, in)
		if err != nil {
			return toolExecutionResponse(req.ID, map[string]string{"error": err.Error()}, true)
		}
		return toolExecutionResponse(req.ID, repo, false)

	case "repo_search":
		var in service.RepoSearchInput
		if err := decodeJSONStrict(args, &in); err != nil {
			return invalidParams(req.ID, err)
		}
		results, err := s.codebase.RepoSearch(ctx, in)
		if err != nil {
			return toolExecutionResponse(req.ID, map[string]string{"error": err.Error()}, true)
		}
		return toolExecutionResponse(req.ID, results, false)

	case "search_across_repos":
		var in service.SearchAcrossReposInput
		if err := decodeJSONStrict(args, &in); err != nil {
			return invalidParams(req.ID, err)
		}
		results, err := s.codebase.SearchAcrossRepos(ctx, in)
		if err != nil {
			return toolExecutionResponse(req.ID, map[string]string{"error": err.Error()}, true)
		}
		return toolExecutionResponse(req.ID, results, false)

	case "repo_find_files":
		var in service.FindFilesInput
		if err := decodeJSONStrict(args, &in); err != nil {
			return invalidParams(req.ID, err)
		}
		results, err := s.codebase.FindFiles(ctx, in)
		if err != nil {
			return toolExecutionResponse(req.ID, map[string]string{"error": err.Error()}, true)
		}
		return toolExecutionResponse(req.ID, results, false)

	case "repo_find_docs":
		var in service.FindFilesInput
		if err := decodeJSONStrict(args, &in); err != nil {
			return invalidParams(req.ID, err)
		}
		results, err := s.codebase.FindDocs(ctx, in)
		if err != nil {
			return toolExecutionResponse(req.ID, map[string]string{"error": err.Error()}, true)
		}
		return toolExecutionResponse(req.ID, results, false)

	case "context_add":
		var in service.AddContextInput
		if err := decodeJSONStrict(args, &in); err != nil {
			return invalidParams(req.ID, err)
		}
		id, err := s.context.AddContext(ctx, in)
		if err != nil {
			return toolExecutionResponse(req.ID, map[string]string{"error": err.Error()}, true)
		}
		return toolExecutionResponse(req.ID, contextAddResult{ID: id}, false)

	case "context_search":
		var in service.SearchContextInput
		if err := decodeJSONStrict(args, &in); err != nil {
			return invalidParams(req.ID, err)
		}
		results, err := s.context.SearchContext(ctx, in)
		if err != nil {
			return toolExecutionResponse(req.ID, map[string]string{"error": err.Error()}, true)
		}
		return toolExecutionResponse(req.ID, results, false)

	case "project_brief":
		var in service.ProjectBriefInput
		if err := decodeJSONStrict(args, &in); err != nil {
			return invalidParams(req.ID, err)
		}
		brief, err := s.context.ProjectBrief(ctx, in)
		if err != nil {
			return toolExecutionResponse(req.ID, map[string]string{"error": err.Error()}, true)
		}
		return toolExecutionResponse(req.ID, projectBriefResult{Brief: brief}, false)
	default:
		return invalidParams(req.ID, fmt.Errorf("unknown tool %q", toolName))
	}
}

func decodeJSONLoose(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, out)
}

func decodeJSONStrict(raw json.RawMessage, out any) error {
	if len(raw) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(out); err != nil {
		return err
	}
	return nil
}

func invalidParams(id json.RawMessage, err error) rpcResponse {
	return rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error: &rpcErrorObject{
			Code:    -32602,
			Message: "invalid params: " + err.Error(),
		},
	}
}

func toolExecutionResponse(id json.RawMessage, payload any, isError bool) rpcResponse {
	text := "{}"
	if b, err := json.Marshal(payload); err == nil {
		text = string(b)
	}
	result := toolCallResult{
		Content: []toolContent{{
			Type: "text",
			Text: text,
		}},
		StructuredContent: payload,
		IsError:           isError,
	}
	return rpcResponse{JSONRPC: jsonRPCVersion, ID: id, Result: result}
}

func (s *Server) writeRPCError(id json.RawMessage, code int, msg string) {
	_ = s.writeResponse(rpcResponse{
		JSONRPC: jsonRPCVersion,
		ID:      id,
		Error: &rpcErrorObject{
			Code:    code,
			Message: msg,
		},
	})
}

func (s *Server) readMessage() ([]byte, error) {
	contentLength := -1
	seenNonEmpty := false
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			// Some clients send newline-delimited JSON and may close without trailing newline.
			if errors.Is(err, io.EOF) && strings.TrimSpace(line) != "" {
				trimmed := strings.TrimSpace(line)
				if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
					return []byte(trimmed), nil
				}
			}
			return nil, err
		}
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			if !seenNonEmpty {
				continue
			}
			break
		}
		if !seenNonEmpty {
			seenNonEmpty = true
			// Compatibility mode for clients that use JSON-RPC line-delimited on stdio.
			if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
				if s.mode == modeUnknown {
					s.mode = modeLineDelimited
				}
				return []byte(trimmed), nil
			}
		}
		parts := strings.SplitN(trimmed, ":", 2)
		if len(parts) != 2 {
			continue
		}
		name := strings.ToLower(strings.TrimSpace(parts[0]))
		value := strings.TrimSpace(parts[1])
		if name == "content-length" {
			n, err := strconv.Atoi(value)
			if err != nil {
				return nil, fmt.Errorf("invalid content-length %q: %w", value, err)
			}
			if s.mode == modeUnknown {
				s.mode = modeFramed
			}
			contentLength = n
		}
	}

	if contentLength < 0 {
		return nil, errors.New("missing content-length header")
	}
	if contentLength == 0 {
		return nil, errors.New("empty content-length")
	}

	payload := make([]byte, contentLength)
	if _, err := io.ReadFull(s.reader, payload); err != nil {
		return nil, err
	}
	return payload, nil
}

func (s *Server) writeResponse(resp rpcResponse) error {
	b, err := json.Marshal(resp)
	if err != nil {
		return fmt.Errorf("marshal rpc response: %w", err)
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.mode == modeLineDelimited {
		if _, err := s.writer.Write(b); err != nil {
			return err
		}
		if _, err := io.WriteString(s.writer, "\n"); err != nil {
			return err
		}
		return nil
	}

	header := fmt.Sprintf("Content-Length: %d\r\nContent-Type: application/json\r\n\r\n", len(b))
	if _, err := io.WriteString(s.writer, header); err != nil {
		return err
	}
	if _, err := s.writer.Write(b); err != nil {
		return err
	}
	return nil
}
