package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	"github.com/spf13/viper"
)

// Config holds the resolved configuration for both subcommands.
type Config struct {
	Dir         string
	Port        int
	BaseURL     string
	Title       string
	Description string
	MaxItems    int
}

const (
	keyDir         = "dir"
	keyPort        = "port"
	keyBaseURL     = "base_url"
	keyTitle       = "title"
	keyDescription = "description"
	keyMaxItems    = "max_items"
)

// mustBindEnv panics on bind error so a typo in an env var name fails fast at startup.
func mustBindEnv(key string, envVar string) {
	if err := viper.BindEnv(key, envVar); err != nil {
		panic(fmt.Sprintf("pulsar: bind env %s: %v", envVar, err))
	}
}

// mustBindPFlag panics on bind error so a typo in a flag name fails fast at startup.
func mustBindPFlag(key string, flag *pflag.Flag) {
	if flag == nil {
		panic(fmt.Sprintf("pulsar: bind flag %q: flag not registered", key))
	}
	if err := viper.BindPFlag(key, flag); err != nil {
		panic(fmt.Sprintf("pulsar: bind flag %q: %v", key, err))
	}
}

// initViper wires environment variables and the optional config file into viper.
func initViper() {
	viper.SetEnvPrefix("PULSAR")
	viper.AutomaticEnv()
	mustBindEnv(keyBaseURL, "PULSAR_BASE_URL")
	mustBindEnv(keyMaxItems, "PULSAR_MAX_ITEMS")

	home, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(filepath.Join(home, ".config", "pulsar"))
	}
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	viper.SetDefault(keyDir, defaultDir(home))
	viper.SetDefault(keyPort, 8765)
	viper.SetDefault(keyTitle, "Code Reviews")
	viper.SetDefault(keyDescription, "Locally generated code reviews")
	viper.SetDefault(keyMaxItems, 200)

	if err := viper.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			fmt.Fprintf(os.Stderr, "pulsar: warning: failed to read config file: %v\n", err)
		}
	}
}

func defaultDir(home string) string {
	if home == "" {
		return "CodeReviews"
	}
	return filepath.Join(home, "CodeReviews")
}

// bindCommonFlags registers flags shared by both subcommands.
func bindCommonFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String("dir", "", "archive directory (default: $HOME/CodeReviews)")
	mustBindPFlag(keyDir, cmd.PersistentFlags().Lookup("dir"))
}

// bindServeFlags registers flags only used by the serve subcommand.
func bindServeFlags(cmd *cobra.Command) {
	cmd.Flags().Int("port", 0, "HTTP port to bind on 127.0.0.1")
	cmd.Flags().String("base-url", "", "public base URL used in generated feed links (required)")
	cmd.Flags().String("title", "", "RSS channel title")
	cmd.Flags().String("description", "", "RSS channel description")
	cmd.Flags().Int("max-items", 0, "maximum number of items in the feed")

	mustBindPFlag(keyPort, cmd.Flags().Lookup("port"))
	mustBindPFlag(keyBaseURL, cmd.Flags().Lookup("base-url"))
	mustBindPFlag(keyTitle, cmd.Flags().Lookup("title"))
	mustBindPFlag(keyDescription, cmd.Flags().Lookup("description"))
	mustBindPFlag(keyMaxItems, cmd.Flags().Lookup("max-items"))
}

// loadConfig returns the resolved configuration.
func loadConfig() Config {
	return Config{
		Dir:         viper.GetString(keyDir),
		Port:        viper.GetInt(keyPort),
		BaseURL:     viper.GetString(keyBaseURL),
		Title:       viper.GetString(keyTitle),
		Description: viper.GetString(keyDescription),
		MaxItems:    viper.GetInt(keyMaxItems),
	}
}
