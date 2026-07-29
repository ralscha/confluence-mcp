// Command confluence-mcp runs a Model Context Protocol server exposing Confluence Cloud
// tools, over either stdio or streamable HTTP, in readonly or readwrite
// mode.
package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"slices"
	"syscall"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"confluence-mcp/internal/config"
	"confluence-mcp/internal/confluence"
	"confluence-mcp/internal/mcpserver"
	"confluence-mcp/internal/version"
)

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	cfg, err := config.Load(os.Args[1:])
	if errors.Is(err, config.ErrVersionRequested) {
		_, _ = fmt.Fprintf(os.Stdout, "confluence-mcp %s\n", version.Version)
		return nil
	}
	if err != nil {
		return err
	}

	confluenceClient, err := confluence.NewClient(cfg.ConfluenceBaseURL, cfg.ConfluenceEmail, cfg.ConfluenceAPIToken, &http.Client{
		Timeout: 30 * time.Second,
	})
	if err != nil {
		return err
	}

	server := mcpserver.NewServer(cfg, confluenceClient)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	switch cfg.Transport {
	case config.TransportStdio:
		return server.Run(ctx, &mcp.StdioTransport{})
	case config.TransportHTTP:
		return runHTTP(ctx, cfg, server)
	default:
		return errors.New("confluence-mcp: unsupported transport: " + string(cfg.Transport))
	}
}

func runHTTP(ctx context.Context, cfg *config.Config, server *mcp.Server) error {
	handler := mcp.NewStreamableHTTPHandler(func(*http.Request) *mcp.Server {
		return server
	}, &mcp.StreamableHTTPOptions{Stateless: true})

	httpServer := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           checkOrigin(handler, cfg.AllowedOrigins),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		//nolint:gosec // %q escapes control characters, preventing log injection
		log.Printf("confluence-mcp: listening on %q (mode=%q)", cfg.HTTPAddr, cfg.Mode)
		errCh <- httpServer.ListenAndServe()
	}()

	select {
	case err := <-errCh:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		return httpServer.Shutdown(shutdownCtx)
	}
}

// checkOrigin rejects browser requests from unexpected origins, guarding the
// unauthenticated HTTP transport against DNS rebinding attacks. Requests
// without an Origin header come from non-browser clients and are allowed.
func checkOrigin(next http.Handler, allowedOrigins []string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")
		if origin != "" && !originAllowed(origin, allowedOrigins) {
			http.Error(w, "forbidden origin", http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// originAllowed reports whether origin may call the HTTP transport. Loopback
// origins are always permitted; anything else must be listed explicitly.
func originAllowed(origin string, allowedOrigins []string) bool {
	if slices.Contains(allowedOrigins, origin) {
		return true
	}

	u, err := url.Parse(origin)
	if err != nil || u.Host == "" {
		return false
	}

	host := u.Hostname()
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
