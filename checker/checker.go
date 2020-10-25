package checker

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cybozu-go/well"
)

type RepoChecker struct {
	latestRevision string
	mu             sync.Mutex

	repoURL    string
	repoBranch string
	repoName   string
	workDir    string
	interval   time.Duration
}

func NewRepoChecker(repoURL, repoBranch, workDir string, interval time.Duration) *RepoChecker {
	items := strings.Split(repoURL, "/")
	last := items[len(items)-1]
	repoName := strings.TrimSuffix(last, ".git")

	return &RepoChecker{
		repoURL:    repoURL,
		repoBranch: repoBranch,
		repoName:   repoName,
		workDir:    workDir,
		interval:   interval,
	}
}

func (c *RepoChecker) Clone(ctx context.Context) error {
	cmd := well.CommandContext(ctx, "git", "clone", "-b", c.repoBranch, c.repoURL)
	cmd.Dir = c.workDir
	return cmd.Run()
}

func (c *RepoChecker) UpdateLatestRevision(ctx context.Context) error {
	err := c.fetchRemoteRevision(ctx)
	if err != nil {
		return err
	}

	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			err := c.fetchRemoteRevision(ctx)
			if err != nil {
				return err
			}
		}
	}
}

func (c *RepoChecker) LatestRevision() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.latestRevision
}

func (c *RepoChecker) fetchRemoteRevision(ctx context.Context) error {
	cmd := well.CommandContext(ctx, "git", "ls-remote", "origin")
	cmd.Dir = filepath.Join(c.workDir, c.repoName)
	out, err := cmd.Output()
	if err != nil {
		return err
	}
	for _, o := range strings.Split(string(out), "\n") {
		fields := strings.Fields(o)
		if len(fields) != 2 {
			continue
		}
		ref := strings.TrimSpace(fields[1])
		if ref == "refs/heads/"+c.repoBranch {
			c.mu.Lock()
			c.latestRevision = strings.TrimSpace(fields[0])
			c.mu.Unlock()
			return nil
		}
	}
	return errors.New("cannot found hash")
}
