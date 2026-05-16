package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
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

// initViper wires environment variables and the optional config file into viper.
func initViper() {
	viper.SetEnvPrefix("PULSAR")
	viper.AutomaticEnv()
	viper.BindEnv(keyBaseURL, "PULSAR_BASE_URL")
	viper.BindEnv(keyMaxItems, "PULSAR_MAX_ITEMS")

	home, err := os.UserHomeDir()
	if err == nil {
		viper.AddConfigPath(filepath.Join(home, ".config", "pulsar"))
	}
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")

	viper.SetDefault(keyDir, defaultDir())
	viper.SetDefault(keyPort, 8765)
	viper.SetDefault(keyTitle, "Code Reviews")
	viper.SetDefault(keyDescription, "Locally generated code reviews")
	viper.SetDefault(keyMaxItems, 200)

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			fmt.Fprintf(os.Stderr, "pulsar: warning: failed to read config file: %v\n", err)
		}
	}
}

func defaultDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return "CodeReviews"
	}
	return filepath.Join(home, "CodeReviews")
}

// bindCommonFlags registers flags shared by both subcommands.
func bindCommonFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().String("dir", "", "archive directory (default: $HOME/CodeReviews)")
	viper.BindPFlag(keyDir, cmd.PersistentFlags().Lookup("dir"))
}

// bindServeFlags registers flags only used by the serve subcommand.
func bindServeFlags(cmd *cobra.Command) {
	cmd.Flags().Int("port", 0, "HTTP port to bind on 127.0.0.1")
	cmd.Flags().String("base-url", "", "public base URL used in generated feed links (required)")
	cmd.Flags().String("title", "", "RSS channel title")
	cmd.Flags().String("description", "", "RSS channel description")
	cmd.Flags().Int("max-items", 0, "maximum number of items in the feed")

	viper.BindPFlag(keyPort, cmd.Flags().Lookup("port"))
	viper.BindPFlag(keyBaseURL, cmd.Flags().Lookup("base-url"))
	viper.BindPFlag(keyTitle, cmd.Flags().Lookup("title"))
	viper.BindPFlag(keyDescription, cmd.Flags().Lookup("description"))
	viper.BindPFlag(keyMaxItems, cmd.Flags().Lookup("max-items"))
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
