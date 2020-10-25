package cmd

import (
	"fmt"
	"os"
	"time"

	"github.com/spf13/cobra"
	"github.com/zoetrope/website-operator"
)

var config struct {
	listenAddr string
	repoURL    string
	repoBranch string
	workDir    string
	interval   time.Duration
}

var rootCmd = &cobra.Command{
	Use:     "repo-checker",
	Version: website.Version,
	Short:   "repo-checker periodically checks the latest hash of the target GitHub repository",
	Long:    `repo-checker periodically checks the latest hash of the target GitHub repository.`,

	RunE: func(cmd *cobra.Command, args []string) error {
		cmd.SilenceUsage = true
		return subMain(cmd.Context())
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
	fs.StringVar(&config.listenAddr, "listen-addr", ":9090", "The address the endpoint binds to")
	fs.StringVar(&config.repoURL, "repo-url", "", "The URL of the repository to be checked")
	fs.StringVar(&config.repoBranch, "repo-branch", "master", "The branch name of the repository")
	fs.StringVar(&config.workDir, "work-dir", "/tmp/repos", "The working directory")
	fs.DurationVar(&config.interval, "interval", 10*time.Minute, "The interval to check the repository")
}
