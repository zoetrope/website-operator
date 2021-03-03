package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/zoetrope/website-operator"
)

var config struct {
	metricsAddr               string
	probeAddr                 string
	enableLeaderElection      bool
	leaderElectionID          string
	nginxContainerImage       string
	repoCheckerContainerImage string
	development               bool
}

var rootCmd = &cobra.Command{
	Use:     "website-operator",
	Version: website.Version,
	Short:   "WebSite Operator",
	Long:    `WebSite Operator.`,

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
	fs.StringVar(&config.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to")
	fs.StringVar(&config.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	fs.StringVar(&config.leaderElectionID, "leader-election-id", "website-operator", "ID for leader election by controller-runtime")
	fs.BoolVar(&config.enableLeaderElection, "leader-elect", false, "Enable leader election for controller manager. Enabling this will ensure there is only one active controller manager.")
	fs.StringVar(&config.nginxContainerImage, "nginx-container-image", website.DefaultNginxContainerImage, "The container image name of nginx")
	fs.StringVar(&config.repoCheckerContainerImage, "repochecker-container-image", website.DefaultRepoCheckerContainerImage, "The container image name of repo-checker")
	fs.BoolVar(&config.development, "development", false, "Zap development mode")
}
