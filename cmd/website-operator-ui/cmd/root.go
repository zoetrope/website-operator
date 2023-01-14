package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zoetrope/website-operator"
)

var config struct {
	listenAddr string
	contentDir string
	allowCORS  bool
}

var rootCmd = &cobra.Command{
	Use:     "website-operator-ui",
	Version: website.Version,
	Short:   "Web UI for WebSite Operator",
	Long:    `Web UI for WebSite Operator.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain()
	},
}

// Execute adds all child commands to the root command and sets flags appropriately.
// This is called by main.main(). It only needs to happen once to the rootCmd.
func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}

func init() {
	fs := rootCmd.Flags()
	fs.StringVar(&config.listenAddr, "listen-addr", ":8080", "The address the endpoint binds to")
	fs.StringVar(&config.contentDir, "content-dir", "/dist", "The path of content files")
	fs.BoolVar(&config.allowCORS, "allow-cors", false, "Allow CORS (for development)")
}
