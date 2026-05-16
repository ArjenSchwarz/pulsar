package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Build-time variables injected via -ldflags "-X main.Var=value".
var (
	Version   = "dev"
	BuildTime = "unknown"
	GitCommit = "unknown"
)

func newRootCmd() *cobra.Command {
	root := &cobra.Command{
		Use:           "pulsar",
		Short:         "Code review feed server",
		Long:          "Pulsar turns a directory of locally-generated code review HTML files into an RSS feed served over Tailscale.",
		Version:       fmt.Sprintf("%s (commit %s, built %s)", Version, GitCommit, BuildTime),
		SilenceUsage:  true,
		SilenceErrors: true,
	}

	bindCommonFlags(root)

	root.AddCommand(newPublishCmd())
	root.AddCommand(newServeCmd())
	root.AddCommand(newInstallCmd())
	root.AddCommand(newUninstallCmd())

	return root
}

func main() {
	cobra.OnInitialize(initViper)
	if err := newRootCmd().Execute(); err != nil {
		fmt.Fprintf(os.Stderr, "pulsar: %v\n", err)
		os.Exit(1)
	}
}
