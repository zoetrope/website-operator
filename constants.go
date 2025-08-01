package website

const (
	DefaultNginxContainerImage = "ghcr.io/zoetrope/nginx:1.28.0"
	WebSiteIndexField          = ".status.ready"
)

var DefaultRepoCheckerContainerImage = "ghcr.io/zoetrope/repo-checker:" + Version
