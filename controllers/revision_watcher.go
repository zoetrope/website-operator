package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"
	"github.com/zoetrope/website-operator"
	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func newRevisionWatcher(client client.Client, log logr.Logger, ch chan<- event.TypedGenericEvent[*websitev1beta1.WebSite], interval time.Duration, revCli RevisionClient) manager.Runnable {
	return &revisionWatcher{
		client:         client,
		log:            log,
		channel:        ch,
		interval:       interval,
		revisionClient: revCli,
	}
}

type revisionWatcher struct {
	client         client.Client
	log            logr.Logger
	channel        chan<- event.TypedGenericEvent[*websitev1beta1.WebSite]
	interval       time.Duration
	revisionClient RevisionClient
}

// Start implements Runnable.Start
func (w revisionWatcher) Start(ctx context.Context) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			err := w.revisionChanged(context.Background())
			if err != nil {
				return err
			}
		}
	}
}

func (w revisionWatcher) revisionChanged(ctx context.Context) error {
	sites := websitev1beta1.WebSiteList{}
	err := w.client.List(ctx, &sites, client.MatchingFields(map[string]string{website.WebSiteIndexField: string(corev1.ConditionTrue)}))
	if err != nil {
		return err
	}

	for _, site := range sites.Items {
		latestRev, err := w.revisionClient.GetLatestRevision(ctx, &site)
		if err != nil {
			w.log.Error(err, "failed to get latest revision")
			continue
		}
		if site.Status.Revision == latestRev {
			continue
		}
		w.log.Info("revisionChanged", "currentRevision", site.Status.Revision, "latestRevision", latestRev)
		ev := event.TypedGenericEvent[*websitev1beta1.WebSite]{
			Object: site.DeepCopy(),
		}
		w.channel <- ev
	}
	return nil
}
