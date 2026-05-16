package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP server that exposes the feed and archive",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadConfig()
			if cfg.BaseURL == "" {
				return errors.New("--base-url is required (or set PULSAR_BASE_URL)")
			}
			parsed, err := url.Parse(cfg.BaseURL)
			if err != nil {
				return fmt.Errorf("invalid --base-url: %w", err)
			}
			if parsed.Scheme == "" || parsed.Host == "" {
				return fmt.Errorf("invalid --base-url: missing scheme or host in %q", cfg.BaseURL)
			}

			handler := newServeHandler(cfg)
			addr := fmt.Sprintf("127.0.0.1:%d", cfg.Port)
			srv := &http.Server{
				Addr:         addr,
				Handler:      handler,
				ReadTimeout:  10 * time.Second,
				WriteTimeout: 30 * time.Second,
				IdleTimeout:  60 * time.Second,
			}

			ln, err := net.Listen("tcp", addr)
			if err != nil {
				return fmt.Errorf("listen: %w", err)
			}
			log.Printf("pulsar serve: listening on %s (archive=%s)", addr, cfg.Dir)

			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()

			errCh := make(chan error, 1)
			go func() { errCh <- srv.Serve(ln) }()

			select {
			case <-ctx.Done():
				log.Print("pulsar serve: shutting down")
				shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				return srv.Shutdown(shutdownCtx)
			case err := <-errCh:
				if errors.Is(err, http.ErrServerClosed) {
					return nil
				}
				return err
			}
		},
	}
	bindServeFlags(cmd)
	return cmd
}

// newServeHandler returns the HTTP handler for pulsar serve.
func newServeHandler(cfg Config) http.Handler {
	// Resolve the archive path once at handler construction so each request
	// avoids a redundant filesystem syscall.
	absArchive, err := filepath.Abs(cfg.Dir)
	if err != nil {
		absArchive = cfg.Dir
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		entries, err := scanArchive(absArchive)
		if err != nil {
			log.Printf("pulsar: scan archive: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		out, err := buildFeed(cfg, entries, time.Now())
		if err != nil {
			log.Printf("pulsar: build feed: %v", err)
			http.Error(w, "internal server error", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		_, _ = io.WriteString(w, out)
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			entries, err := scanArchive(absArchive)
			if err != nil {
				log.Printf("pulsar: scan archive: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			writeIndex(w, cfg, entries)
			return
		}
		serveArchiveFile(w, r, absArchive)
	})

	return loggingMiddleware(mux)
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		log.Printf("%s %s %s %s", r.RemoteAddr, r.Method, r.URL.Path, time.Since(start))
	})
}

// serveArchiveFile serves files from the archive directory with traversal protection.
// absArchive must be a cleaned absolute path.
func serveArchiveFile(w http.ResponseWriter, r *http.Request, absArchive string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, "/")
	if rel == "" {
		http.NotFound(w, r)
		return
	}
	// filepath.IsLocal rejects absolute paths, paths escaping via "..",
	// and (on Windows) reserved names.
	if !filepath.IsLocal(rel) {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	target := filepath.Join(absArchive, rel)
	// Refuse symlinks: an attacker who plants one in the archive could
	// otherwise read arbitrary files.
	info, err := os.Lstat(target)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	if info.Mode()&os.ModeSymlink != 0 {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	http.ServeFile(w, r, target)
}

// scanArchive walks the archive directory and returns all entries with valid metadata.
// absArchive must be a cleaned absolute path.
func scanArchive(absArchive string) ([]FeedEntry, error) {
	var entries []FeedEntry
	err := filepath.WalkDir(absArchive, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) && path == absArchive {
				return nil
			}
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		if !strings.HasSuffix(strings.ToLower(d.Name()), ".html") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			log.Printf("pulsar: read %s: %v", path, err)
			return nil
		}
		m, err := extractMetadata(data)
		if err != nil {
			log.Printf("pulsar: skip %s: %v", path, err)
			return nil
		}
		if err := m.validate(); err != nil {
			log.Printf("pulsar: skip %s: %v", path, err)
			return nil
		}
		rel, err := filepath.Rel(absArchive, path)
		if err != nil {
			return nil
		}
		entries = append(entries, FeedEntry{Meta: m, RelativePath: filepath.ToSlash(rel)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return entries, nil
}

// writeIndex renders a minimal HTML index of recent items.
func writeIndex(w http.ResponseWriter, cfg Config, entries []FeedEntry) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	var b strings.Builder
	fmt.Fprintf(&b, `<!doctype html>
<html><head><meta charset="utf-8"><title>%s</title></head>
<body>
<h1>%s</h1>
<p><a href="/feed.xml">RSS feed</a></p>
<ul>
`, html.EscapeString(cfg.Title), html.EscapeString(cfg.Title))

	sorted := slices.Clone(entries)
	sortEntriesDesc(sorted)
	for _, e := range sorted {
		fmt.Fprintf(&b, `<li><a href="/%s">%s</a></li>`, html.EscapeString(e.RelativePath), html.EscapeString(itemTitle(e.Meta)))
	}
	b.WriteString("</ul></body></html>")
	_, _ = io.WriteString(w, b.String())
}
