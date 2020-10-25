package checker

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestPublicRepository(t *testing.T) {
	workDir, err := ioutil.TempDir("/tmp", "website-operator-checker-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)

	rc := NewRepoChecker("https://github.com/zoetrope/honkit-sample.git", "main", workDir, 5*time.Second)
	ctx := context.Background()
	err = rc.Clone(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(filepath.Join(workDir, rc.repoName, "README.md"))
	if err != nil {
		t.Fatal(err)
	}

	rev := rc.LatestRevision()
	if rev != "" {
		t.Fatal("something wrong")
	}

	err = rc.fetchRemoteRevision(ctx)
	if err != nil {
		t.Fatal(err)
	}

	rev = rc.LatestRevision()
	if rev == "" {
		t.Fatal("failed to get latest revision")
	}
}

func TestPrivateRepository(t *testing.T) {
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	err = os.Setenv("GIT_SSH_COMMAND", "ssh -F "+filepath.Join(wd, "..", "e2e", "manifests", ".ssh", "config")+" -i "+filepath.Join(wd, "..", "e2e", "manifests", ".ssh", "id_rsa"))
	if err != nil {
		t.Fatal(err)
	}

	workDir, err := ioutil.TempDir("/tmp", "website-operator-checker-test")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(workDir)

	rc := NewRepoChecker("git@github.com:zoetrope/mkdocs-sample.git", "main", workDir, 5*time.Second)
	ctx := context.Background()
	err = rc.Clone(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = os.Stat(filepath.Join(workDir, rc.repoName, "README.md"))
	if err != nil {
		t.Fatal(err)
	}

	rev := rc.LatestRevision()
	if rev != "" {
		t.Fatal("something wrong")
	}

	err = rc.fetchRemoteRevision(ctx)
	if err != nil {
		t.Fatal(err)
	}

	rev = rc.LatestRevision()
	if rev == "" {
		t.Fatal("failed to get latest revision")
	}
}
