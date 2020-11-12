package backend

import (
	"encoding/json"
	"net/http"

	"github.com/cybozu-go/log"
	"github.com/zoetrope/website-operator/api/v1beta1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func NewAPIServer(kubeClient client.Client, rawClient *kubernetes.Clientset) http.Handler {
	return &apiServer{
		kubeClient: kubeClient,
		rawClient:  rawClient,
	}
}

type apiServer struct {
	kubeClient client.Client
	rawClient  *kubernetes.Clientset
}

func (s apiServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	p := r.URL.Path[len("/api/v1/"):]
	switch {
	case r.Method == http.MethodGet && p == "websites":
		s.listWebSites(w, r)
	case r.Method == http.MethodGet && p == "logs":
		s.getBuildLog(w, r)
	default:
		http.Error(w, "requested resource is not found", http.StatusNotFound)
	}
}

func (s apiServer) listWebSites(w http.ResponseWriter, r *http.Request) {
	var websites v1beta1.WebSiteList
	err := s.kubeClient.List(r.Context(), &websites)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	err = json.NewEncoder(w).Encode(websites)
	if err != nil {
		log.Error("failed to output JSON", map[string]interface{}{
			log.FnError: err.Error(),
		})
	}
}

func (s apiServer) getBuildLog(w http.ResponseWriter, r *http.Request) {

}
