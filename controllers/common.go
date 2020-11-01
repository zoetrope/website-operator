package controllers

import (
	"context"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/cybozu-go/well"
	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
)

func getLatestRevision(ctx context.Context, webSite *websitev1beta1.WebSite) (string, error) {
	repoCheckerHost := fmt.Sprintf("%s%s.%s.svc.cluster.local", webSite.Name, RepoCheckerSuffix, webSite.Namespace)
	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodGet,
		fmt.Sprintf("http://%s/", repoCheckerHost),
		nil,
	)
	if err != nil {
		return "", err
	}

	cli := &well.HTTPClient{Client: &http.Client{}}
	resp, err := cli.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusNotFound {
		return "", errRevisionNotReady
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to repo check: %s", resp.Status)
	}

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	return string(b), nil
}
