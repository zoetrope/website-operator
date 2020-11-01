package controllers

import (
	"context"
	"time"

	"github.com/go-logr/logr"

	"github.com/zoetrope/website-operator"
	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/event"
	"sigs.k8s.io/controller-runtime/pkg/manager"
)

func newRevisionWatcher(client client.Client, log logr.Logger, ch chan<- event.GenericEvent, interval time.Duration) manager.Runnable {
	return &revisionWatcher{
		client:   client,
		log:      log,
		channel:  ch,
		interval: interval,
	}
}

type revisionWatcher struct {
	client   client.Client
	log      logr.Logger
	channel  chan<- event.GenericEvent
	interval time.Duration
}

// Start implements Runnable.Start
func (w revisionWatcher) Start(ch <-chan struct{}) error {
	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ch:
			return nil
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
		latestRev, err := getLatestRevision(ctx, &site)
		if err != nil {
			w.log.Error(err, "failed to get latest revision")
			continue
		}
		if site.Status.Revision == latestRev {
			continue
		}
		w.log.Info("revisionChanged", "currentRevision", site.Status.Revision, "latestRevision", latestRev)
		w.channel <- event.GenericEvent{
			Meta: &metav1.ObjectMeta{
				Namespace: site.Namespace,
				Name:      site.Name,
			},
		}
	}
	return nil
}
