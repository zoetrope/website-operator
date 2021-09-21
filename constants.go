package website

const (
	DefaultNginxContainerImage       = "ghcr.io/zoetrope/nginx:1.20.1"
	DefaultRepoCheckerContainerImage = "ghcr.io/zoetrope/repo-checker:" + Version
	WebSiteIndexField                = ".status.ready"
)
