package cmd

import (
	"context"
	"net/http"
	"os"

	"github.com/cybozu-go/well"
	"github.com/zoetrope/website-operator/checker"
)

func subMain(ctx context.Context) error {
	err := os.MkdirAll(config.workDir, 0755)
	if err != nil {
		return err
	}
	rc := checker.NewRepoChecker(config.repoURL, config.repoBranch, config.workDir, config.interval)
	err = rc.Clone(ctx)
	if err != nil {
		return err
	}

	well.Go(rc.UpdateLatestRevision)

	http.HandleFunc("/", createHandler(rc))
	serv := &well.HTTPServer{
		Server: &http.Server{
			Addr:    config.listenAddr,
			Handler: http.DefaultServeMux,
		},
	}

	err = serv.ListenAndServe()
	if err != nil {
		return err
	}
	err = well.Wait()

	if err != nil && !well.IsSignaled(err) {
		return err
	}

	return nil
}

func createHandler(rc *checker.RepoChecker) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		rev := rc.LatestRevision()
		if len(rev) == 0 {
			http.Error(w, "revision not found", http.StatusNotFound)
			return
		}
		_, err := w.Write([]byte(rev))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
	}
}
