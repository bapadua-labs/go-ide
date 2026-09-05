package main

import (
	"context"

	"go.lsp.dev/protocol"
)

type ideLSPClient struct {
	protocol.UnimplementedClient
	g *goplsClient
}

func (c *ideLSPClient) PublishDiagnostics(_ context.Context, params *protocol.PublishDiagnosticsParams) error {
	if params == nil || c.g == nil {
		return nil
	}
	path := params.URI.FsPath()
	diags := fileDiagnosticsFromLSP(params.Diagnostics)
	c.g.notifyDiagnostics(path, diags)
	return nil
}
