package website

const (
	DefaultNginxContainerImage       = "ghcr.io/zoetrope/nginx:1.20.2"
	DefaultRepoCheckerContainerImage = "ghcr.io/zoetrope/repo-checker:" + Version
	WebSiteIndexField                = ".status.ready"
)
