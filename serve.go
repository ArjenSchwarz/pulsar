package main

import (
	"context"
	"errors"
	"fmt"
	"html"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path/filepath"
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
			if _, err := url.Parse(cfg.BaseURL); err != nil {
				return fmt.Errorf("invalid --base-url: %w", err)
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
	mux := http.NewServeMux()
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		entries, err := scanArchive(cfg.Dir)
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
		w.Write([]byte(out))
	})

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" {
			entries, err := scanArchive(cfg.Dir)
			if err != nil {
				log.Printf("pulsar: scan archive: %v", err)
				http.Error(w, "internal server error", http.StatusInternalServerError)
				return
			}
			writeIndex(w, cfg, entries)
			return
		}
		serveArchiveFile(w, r, cfg.Dir)
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
func serveArchiveFile(w http.ResponseWriter, r *http.Request, archive string) {
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	rel := strings.TrimPrefix(r.URL.Path, "/")
	if rel == "" {
		http.NotFound(w, r)
		return
	}
	// Reject any traversal attempt before resolving.
	if strings.Contains(rel, "..") {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	clean := filepath.Clean(rel)
	absArchive, err := filepath.Abs(archive)
	if err != nil {
		http.Error(w, "internal server error", http.StatusInternalServerError)
		return
	}
	target := filepath.Join(absArchive, clean)
	// Confirm the resolved path stays inside the archive directory.
	if !strings.HasPrefix(target, absArchive+string(filepath.Separator)) && target != absArchive {
		http.Error(w, "forbidden", http.StatusForbidden)
		return
	}
	info, err := os.Stat(target)
	if err != nil || info.IsDir() {
		http.NotFound(w, r)
		return
	}
	http.ServeFile(w, r, target)
}

// scanArchive walks the archive directory and returns all entries with valid metadata.
func scanArchive(archive string) ([]FeedEntry, error) {
	absArchive, err := filepath.Abs(archive)
	if err != nil {
		return nil, err
	}
	var entries []FeedEntry
	err = filepath.WalkDir(absArchive, func(path string, d os.DirEntry, walkErr error) error {
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

	// Sort newest-first by reusing buildFeed's ordering: build a temporary slice.
	sorted := make([]FeedEntry, len(entries))
	copy(sorted, entries)
	sortEntriesDesc(sorted)
	for _, e := range sorted {
		fmt.Fprintf(&b, `<li><a href="/%s">%s</a></li>`, html.EscapeString(e.RelativePath), html.EscapeString(itemTitle(e.Meta)))
	}
	b.WriteString("</ul></body></html>")
	w.Write([]byte(b.String()))
}

func sortEntriesDesc(entries []FeedEntry) {
	// Simple insertion sort: list is small in practice; avoids importing sort here twice.
	for i := 1; i < len(entries); i++ {
		for j := i; j > 0 && entries[j].Meta.Date > entries[j-1].Meta.Date; j-- {
			entries[j], entries[j-1] = entries[j-1], entries[j]
		}
	}
}
