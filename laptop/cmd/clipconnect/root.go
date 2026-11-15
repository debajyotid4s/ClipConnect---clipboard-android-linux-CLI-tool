package main

import (
	"github.com/spf13/cobra"
)

// rootCmd is `clipconnect`: launch the interactive menu.
var rootCmd = &cobra.Command{
	Use:   "clipconnect",
	Short: "ClipConnect — Android↔Linux synced clipboard",
	Long: "ClipConnect is a CLI-based tool for automated, encrypted clipboard\n" +
		"syncing between an Android phone and a Linux laptop on the same LAN.\n\n" +
		"Run it with no command to open the interactive menu, or use one of the\n" +
		"subcommands: pair, unpair, logs, help, version.",
	RunE: func(cmd *cobra.Command, args []string) error {
		return runInteractive()
	},
}

// versionCmd is `clipconnect version`.
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the ClipConnect version",
	Run: func(cmd *cobra.Command, args []string) {
		println(version)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

// Execute runs the root command.
func Execute() error {
	return rootCmd.Execute()
}
