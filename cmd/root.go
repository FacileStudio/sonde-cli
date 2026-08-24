package cmd

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/FacileStudio/sonde-cli/internal/ui"
)

var version = "dev"

var rootCmd = &cobra.Command{
	Use:   "sonde",
	Short: "Uptime monitoring for the Facile suite",
	Long:  "Sonde is the companion CLI for a Sonde uptime-monitoring instance. It pushes heartbeats from cron jobs and manages monitors, status and incidents.",
}

func init() {
	rootCmd.Version = version
	rootCmd.SetVersionTemplate("{{.Name}} {{.Version}}\n")
	rootCmd.PersistentFlags().Bool("no-color", false, "Disable colored output")
	rootCmd.PersistentFlags().String("instance", "", "Sonde instance URL")
	cobra.OnInitialize(func() {
		if v, _ := rootCmd.PersistentFlags().GetBool("no-color"); v {
			ui.DisableColor()
		}
	})
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
