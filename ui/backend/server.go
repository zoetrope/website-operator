package backend

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/cybozu-go/log"
	"github.com/zoetrope/website-operator/api/v1beta1"
	"github.com/zoetrope/website-operator/controllers"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewAPIServer(kubeClient client.Client, rawClient *kubernetes.Clientset, allowCORS bool) http.Handler {
	return &apiServer{
		kubeClient: kubeClient,
		rawClient:  rawClient,
		allowCORS:  allowCORS,
	}
}

type apiServer struct {
	kubeClient client.Client
	rawClient  *kubernetes.Clientset
	allowCORS  bool
}

func (s apiServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.allowCORS {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		if r.Method == http.MethodOptions {
			return
		}
	}

	p := r.URL.Path[len("/api/v1/"):]
	switch {
	case r.Method == http.MethodGet && p == "websites":
		s.listWebSites(w, r)
	case r.Method == http.MethodGet && strings.HasPrefix(p, "logs/"):
		s.getBuildLog(w, r)
	default:
		http.Error(w, "requested resource is not found", http.StatusNotFound)
	}
}

type website struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Status    string `json:"status"`
	Revision  string `json:"revision"`
	RepoURL   string `json:"repo"`
	PublicURL string `json:"public"`
	Branch    string `json:"branch"`
}

func (s apiServer) listWebSites(w http.ResponseWriter, r *http.Request) {
	var websites v1beta1.WebSiteList
	err := s.kubeClient.List(r.Context(), &websites)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	resp := make([]website, len(websites.Items))
	for i, item := range websites.Items {
		status, err := s.getStatus(r, item)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		rev := item.Status.Revision
		if len(rev) > 7 {
			rev = rev[:7]
		}
		resp[i] = website{
			Namespace: item.Namespace,
			Name:      item.Name,
			Status:    status,
			Revision:  rev,
			RepoURL:   item.Spec.RepoURL,
			PublicURL: item.Spec.PublicURL,
			Branch:    item.Spec.Branch,
		}
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(resp)
	if err != nil {
		log.Error("failed to output JSON", map[string]interface{}{
			log.FnError: err.Error(),
		})
	}
}

func (s apiServer) getStatus(r *http.Request, website v1beta1.WebSite) (string, error) {
	if website.Status.Ready != corev1.ConditionTrue {
		return "NotReady", nil
	}

	var pods corev1.PodList
	err := s.kubeClient.List(r.Context(), &pods, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"app.kubernetes.io/instance":   website.Name,
			"app.kubernetes.io/managed-by": "website-operator",
		}),
		Namespace: website.Namespace,
	})
	if err != nil {
		return "", err
	}

	for _, pod := range pods.Items {
		if pod.Status.Phase != corev1.PodRunning {
			return string(pod.Status.Phase), nil
		}
	}
	return "Running", nil
}

func (s apiServer) getBuildLog(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path[len("/api/v1/logs/"):]
	params := strings.Split(p, "/")
	if len(params) != 2 {
		http.Error(w, "invalid parameter", http.StatusBadRequest)
		return
	}
	ns := params[0]
	resName := params[1]

	var pods corev1.PodList
	err := s.kubeClient.List(r.Context(), &pods, &client.ListOptions{
		LabelSelector: labels.SelectorFromSet(map[string]string{
			"app.kubernetes.io/name":       controllers.AppNameNginx,
			"app.kubernetes.io/instance":   resName,
			"app.kubernetes.io/managed-by": "website-operator",
		}),
		Namespace: ns,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	if len(pods.Items) == 0 {
		http.Error(w, "not found", http.StatusNotFound)
		return
	}
	latestPod := pods.Items[0]
	creationTimestamp := latestPod.GetCreationTimestamp().Time
	for _, pod := range pods.Items {
		if pod.GetCreationTimestamp().After(creationTimestamp) {
			latestPod = pod
			creationTimestamp = pod.GetCreationTimestamp().Time
		}
	}

	req := s.rawClient.CoreV1().Pods(ns).GetLogs(latestPod.GetName(), &corev1.PodLogOptions{
		Container: "build",
	})

	readCloser, err := req.Stream(r.Context())
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	defer readCloser.Close()
	_, err = io.Copy(w, readCloser)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}
