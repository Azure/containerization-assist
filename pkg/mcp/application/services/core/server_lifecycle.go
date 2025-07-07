package core

import (
	"context"

	"github.com/Azure/container-kit/pkg/mcp/application/core"
	"github.com/Azure/container-kit/pkg/mcp/application/internal/constants"
	"github.com/Azure/container-kit/pkg/mcp/domain/errors"
)

// Start starts the MCP server
func (s *Server) Start(ctx context.Context) error {
	s.logger.Info("Starting Container Kit MCP Server",
		"transport", s.config.TransportType,
		"workspace_dir", s.config.WorkspaceDir,
		"max_sessions", s.config.MaxSessions)

	s.sessionManager.StartCleanupRoutine()
	if s.gomcpManager == nil {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Severity(errors.SeverityCritical).
			Message("gomcp manager is nil - server initialization failed").
			Context("module", "core/server-lifecycle").
			Context("component", "MCPServer").
			Context("phase", "server_initialization").
			Suggestion("Ensure server is properly created with NewMCPServer").
			WithLocation().
			Build()
	}
	concreteManager, ok := s.gomcpManager.(*GomcpManager)
	if !ok {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Severity(errors.SeverityCritical).
			Message("gomcp manager is not the expected concrete type").
			Context("module", "core/server-lifecycle").
			Context("component", "MCPServer").
			Context("phase", "type_assertion").
			Suggestion("Ensure gomcp manager is created with NewGomcpManager").
			WithLocation().
			Build()
	}

	if err := concreteManager.Initialize(); err != nil {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Severity(errors.SeverityCritical).
			Message("failed to initialize gomcp manager").
			Context("module", "core/server-lifecycle").
			Context("component", "MCPServer").
			Context("phase", "gomcp_initialization").
			Cause(err).
			Suggestion("Check transport configuration and dependencies").
			WithLocation().
			Build()
	}

	concreteManager.SetToolOrchestrator(s.toolOrchestrator)
	if err := concreteManager.RegisterTools(s); err != nil {
		return errors.NewError().
			Code(errors.CodeInternalError).
			Type(errors.ErrTypeInternal).
			Severity(errors.SeverityHigh).
			Message("failed to register tools with gomcp").
			Context("module", "core/server-lifecycle").
			Context("component", "MCPServer").
			Context("phase", "tool_registration").
			Cause(err).
			Suggestion("Check tool configuration and ensure tools are properly implemented").
			WithLocation().
			Build()
	}

	if httpTransport, ok := s.transport.(interface{ SetServer(interface{}) }); ok {
		httpTransport.SetServer(s)
		s.logger.Info("Set server reference on HTTP transport")
	}

	if setter, ok := s.transport.(interface{ SetHandler(interface{}) }); ok {
		setter.SetHandler(s)
	}
	transportDone := make(chan error, 1)
	go func() {
		transportDone <- concreteManager.StartServer()
	}()

	select {
	case err := <-transportDone:
		if err != nil {
			s.logger.Error("Transport error", "error", err)
			return err
		}
		return nil
	case <-ctx.Done():
		s.logger.Info("Context cancelled")
		shutdownCtx, cancel := context.WithTimeout(context.Background(), constants.ShutdownTimeout)
		defer cancel()
		return s.Shutdown(shutdownCtx)
	}
}

// HandleRequest implements the LocalRequestHandler interface
func (s *Server) HandleRequest(ctx context.Context, req *core.MCPRequest) (*core.MCPResponse, error) {
	return &core.MCPResponse{
		ID: req.ID,
		Error: &core.MCPError{
			Code:    -32601,
			Message: "direct request handling not implemented",
		},
	}, nil
}

// Stop gracefully stops the MCP server
func (s *Server) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), constants.ShutdownTimeout)
	defer cancel()
	return s.Shutdown(ctx)
}
