package website

const (
	DefaultNginxContainerImage = "ghcr.io/zoetrope/nginx:1.22.1"
	WebSiteIndexField          = ".status.ready"
)

var DefaultRepoCheckerContainerImage = "ghcr.io/zoetrope/repo-checker:" + Version
