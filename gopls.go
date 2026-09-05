package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"go.lsp.dev/jsonrpc2"
	"go.lsp.dev/protocol"
	"go.lsp.dev/uri"
)

type goplsClient struct {
	mu      sync.Mutex
	cmd     *exec.Cmd
	conn    jsonrpc2.Conn
	server  protocol.Server
	ctx     context.Context
	cancel  context.CancelFunc
	root    string
	goroot  string
	docs    map[string]int32
	ready   bool
}

type completionSuggestion struct {
	Label       string
	Detail      string
	InsertText  string
	ReplaceFrom int
	ReplaceTo   int
}

func newGoplsClient() *goplsClient {
	return &goplsClient{
		docs: make(map[string]int32),
	}
}

func (g *goplsClient) start(goroot, rootPath string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.ready && g.root == rootPath && g.goroot == goroot {
		return nil
	}

	g.stopLocked()

	if rootPath == "" {
		return fmt.Errorf("abra uma pasta de projeto para usar o autocomplete")
	}

	bin, err := resolveToolBinary(goroot, "gopls")
	if err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	cmd := exec.Command(bin, "serve")
	cmd.Dir = rootPath
	cmd.Env = os.Environ()

	stdin, err := cmd.StdinPipe()
	if err != nil {
		cancel()
		return err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return err
	}
	cmd.Stderr = io.Discard

	if err := cmd.Start(); err != nil {
		cancel()
		return err
	}

	stream := jsonrpc2.NewStream(struct {
		io.Reader
		io.Writer
		io.Closer
	}{
		Reader: stdout,
		Writer: stdin,
		Closer: stdin,
	})

	_, conn, server := protocol.NewClient(ctx, protocol.UnimplementedClient{}, stream)

	pid := int32(os.Getpid())
	wsFolders := protocol.NewNullable([]protocol.WorkspaceFolder{{
		URI:  uri.File(rootPath),
		Name: filepath.Base(rootPath),
	}})

	initParams := &protocol.InitializeParams{
		ProcessID: &pid,
		ClientInfo: protocol.ClientInfo{
			Name:    "go-ide",
			Version: protocol.NewOptional("0.1"),
		},
		WorkspaceFoldersInitializeParams: protocol.WorkspaceFoldersInitializeParams{
			WorkspaceFolders: wsFolders,
		},
		Capabilities: protocol.ClientCapabilities{
			Workspace: &protocol.WorkspaceClientCapabilities{
				WorkspaceFolders: boolPtr(true),
			},
			TextDocument: &protocol.TextDocumentClientCapabilities{
				Completion: &protocol.CompletionClientCapabilities{
					ContextSupport: boolPtr(true),
					CompletionItem: &protocol.ClientCompletionItemOptions{
						SnippetSupport: boolPtr(true),
					},
				},
			},
		},
	}

	initCtx, initCancel := context.WithTimeout(ctx, 15*time.Second)
	defer initCancel()

	if _, err := server.Initialize(initCtx, initParams); err != nil {
		_ = conn.Close()
		_ = cmd.Process.Kill()
		cancel()
		return fmt.Errorf("gopls initialize: %w", err)
	}
	if err := server.Initialized(ctx, &protocol.InitializedParams{}); err != nil {
		_ = conn.Close()
		_ = cmd.Process.Kill()
		cancel()
		return fmt.Errorf("gopls initialized: %w", err)
	}

	g.cmd = cmd
	g.conn = conn
	g.server = server
	g.ctx = ctx
	g.cancel = cancel
	g.root = rootPath
	g.goroot = goroot
	g.docs = make(map[string]int32)
	g.ready = true
	return nil
}

func (g *goplsClient) stop() {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.stopLocked()
}

func (g *goplsClient) stopLocked() {
	if !g.ready {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = g.server.Shutdown(ctx)
	_ = g.server.Exit(ctx)
	_ = g.conn.Close()
	if g.cmd != nil && g.cmd.Process != nil {
		_ = g.cmd.Process.Kill()
	}
	if g.cancel != nil {
		g.cancel()
	}
	g.cmd = nil
	g.conn = nil
	g.server = nil
	g.ctx = nil
	g.cancel = nil
	g.docs = make(map[string]int32)
	g.ready = false
}

func (g *goplsClient) openDocument(path, text string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.ready || path == "" {
		return nil
	}

	version := g.docs[path] + 1
	g.docs[path] = version

	ctx, cancel := context.WithTimeout(g.ctx, 10*time.Second)
	defer cancel()

	return g.server.DidOpen(ctx, &protocol.DidOpenTextDocumentParams{
		TextDocument: protocol.TextDocumentItem{
			URI:        uri.File(path),
			LanguageID: "go",
			Version:    version,
			Text:       text,
		},
	})
}

func (g *goplsClient) changeDocument(path, text string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.ready || path == "" {
		return nil
	}

	version := g.docs[path] + 1
	g.docs[path] = version

	ctx, cancel := context.WithTimeout(g.ctx, 10*time.Second)
	defer cancel()

	return g.server.DidChange(ctx, &protocol.DidChangeTextDocumentParams{
		TextDocument: protocol.VersionedTextDocumentIdentifier{
			TextDocumentIdentifier: protocol.TextDocumentIdentifier{URI: uri.File(path)},
			Version:                version,
		},
		ContentChanges: []protocol.TextDocumentContentChangeEvent{
			&protocol.TextDocumentContentChangeWholeDocument{Text: text},
		},
	})
}

func (g *goplsClient) closeDocument(path string) error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.ready || path == "" {
		return nil
	}

	delete(g.docs, path)

	ctx, cancel := context.WithTimeout(g.ctx, 5*time.Second)
	defer cancel()

	return g.server.DidClose(ctx, &protocol.DidCloseTextDocumentParams{
		TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(path)},
	})
}

func (g *goplsClient) completions(path, text string, row, byteCol int) ([]completionSuggestion, error) {
	g.mu.Lock()
	if !g.ready || path == "" {
		g.mu.Unlock()
		return nil, nil
	}
	server := g.server
	ctx := g.ctx
	g.mu.Unlock()

	pos := byteOffsetToPosition(text, row, byteCol)
	params := &protocol.CompletionParams{
		TextDocumentPositionParams: protocol.TextDocumentPositionParams{
			TextDocument: protocol.TextDocumentIdentifier{URI: uri.File(path)},
			Position:     pos,
		},
		Context: protocol.CompletionContext{
			TriggerKind: protocol.CompletionTriggerKindInvoked,
		},
	}

	callCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	result, err := server.Completion(callCtx, params)
	if err != nil {
		return nil, err
	}

	items := completionItemsFromResult(result)
	suggestions := make([]completionSuggestion, 0, len(items))
	for _, item := range items {
		suggestions = append(suggestions, suggestionFromItem(text, item))
	}
	return suggestions, nil
}

func completionItemsFromResult(result protocol.CompletionResult) []protocol.CompletionItem {
	if result == nil {
		return nil
	}
	switch v := result.(type) {
	case protocol.CompletionItemSlice:
		return v
	case *protocol.CompletionList:
		if v == nil {
			return nil
		}
		return v.Items
	default:
		return nil
	}
}

func suggestionFromItem(text string, item protocol.CompletionItem) completionSuggestion {
	s := completionSuggestion{Label: item.Label}
	if detail, ok := item.Detail.Get(); ok {
		s.Detail = detail
	}

	cursor := len(text)
	s.ReplaceFrom = cursor
	s.ReplaceTo = cursor
	s.InsertText = item.Label

	if edit := item.TextEdit; edit != nil {
		switch e := edit.(type) {
		case *protocol.TextEdit:
			s.ReplaceFrom = positionToByteOffset(text, e.Range.Start)
			s.ReplaceTo = positionToByteOffset(text, e.Range.End)
			s.InsertText = e.NewText
		case *protocol.InsertReplaceEdit:
			s.ReplaceFrom = positionToByteOffset(text, e.Replace.Start)
			s.ReplaceTo = positionToByteOffset(text, e.Replace.End)
			s.InsertText = e.NewText
		}
	} else if insert, ok := item.InsertText.Get(); ok {
		s.InsertText = insert
	}

	return s
}

func boolPtr(v bool) *bool {
	return &v
}

func (g *goplsClient) syncDocument(path, text string) error {
	g.mu.Lock()
	opened := g.docs[path] > 0
	g.mu.Unlock()

	if opened {
		return g.changeDocument(path, text)
	}
	return g.openDocument(path, text)
}
