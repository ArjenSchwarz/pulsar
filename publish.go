package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/spf13/cobra"
)

func newPublishCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "publish <file>",
		Short: "Validate a review HTML file and move it into the archive",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg := loadConfig()
			dest, err := publish(cfg, args[0], time.Now())
			if err != nil {
				return err
			}
			fmt.Fprintln(cmd.OutOrStdout(), dest)
			return nil
		},
	}
	return cmd
}

// publish runs the publish flow and returns the destination file path on success.
func publish(cfg Config, srcPath string, now time.Time) (string, error) {
	abs, err := filepath.Abs(srcPath)
	if err != nil {
		return "", err
	}
	html, err := os.ReadFile(abs)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("file not found: %s", abs)
		}
		return "", err
	}

	meta, err := extractMetadata(html)
	if err != nil {
		return "", err
	}

	normalised, err := normalizeRepoURL(meta.RepoURL)
	if err != nil {
		return "", err
	}
	meta.RepoURL = normalised
	meta.Date = now.UTC().Format(time.RFC3339)

	if err := meta.validate(); err != nil {
		return "", err
	}

	subdir := monthSubdir(meta.Date)
	date := dateSegment(meta.Date)
	var filename, glob string
	if meta.PR != nil {
		filename = prFilename(date, meta.RepoURL, *meta.PR)
		glob = prGlob(cfg.Dir, meta.RepoURL, *meta.PR)
	} else {
		filename = branchFilename(date, meta.RepoURL, meta.Branch)
		glob = branchGlob(cfg.Dir, meta.RepoURL, meta.Branch)
	}

	priorVersions, err := filepath.Glob(glob)
	if err != nil {
		return "", fmt.Errorf("listing prior versions: %w", err)
	}

	destDir := filepath.Join(cfg.Dir, subdir)
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("creating archive directory: %w", err)
	}

	updated, err := replaceMetadata(html, meta)
	if err != nil {
		return "", err
	}

	destPath := filepath.Join(destDir, filename)
	if err := writeFileAtomic(destPath, updated, 0o644); err != nil {
		return "", err
	}

	// Delete prior versions, skipping the new file in case it sits in the same dir.
	for _, p := range priorVersions {
		if p == destPath {
			continue
		}
		if err := os.Remove(p); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("removing prior version %s: %w", p, err)
		}
	}

	if abs != destPath {
		if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
			return "", fmt.Errorf("removing source file: %w", err)
		}
	}

	return destPath, nil
}

// writeFileAtomic writes data to path via a temp file + rename. The temp file
// lives in the same directory so rename is atomic on the same filesystem.
func writeFileAtomic(path string, data []byte, perm os.FileMode) error {
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".pulsar-tmp-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := f.Name()
	cleanup := func() { _ = os.Remove(tmpPath) }

	if _, err := f.Write(data); err != nil {
		f.Close()
		cleanup()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := f.Chmod(perm); err != nil {
		f.Close()
		cleanup()
		return fmt.Errorf("chmod temp file: %w", err)
	}
	if err := f.Close(); err != nil {
		cleanup()
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		cleanup()
		return fmt.Errorf("renaming temp file: %w", err)
	}
	return nil
}
