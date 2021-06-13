package controllers

import (
	"context"
	"path/filepath"
	"testing"

	corev1 "k8s.io/api/core/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	"github.com/zoetrope/website-operator"
	"k8s.io/apimachinery/pkg/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/envtest"
	"sigs.k8s.io/controller-runtime/pkg/envtest/printer"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"

	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	//+kubebuilder:scaffold:imports
)

// These tests use Ginkgo (BDD-style Go testing framework). Refer to
// http://onsi.github.io/ginkgo/ to learn more about Ginkgo.

var cfg *rest.Config
var k8sClient client.Client
var testEnv *envtest.Environment

func TestAPIs(t *testing.T) {
	RegisterFailHandler(Fail)

	RunSpecsWithDefaultAndCustomReporters(t,
		"Controller Suite",
		[]Reporter{printer.NewlineReporter{}})
}

var _ = BeforeSuite(func() {
	logf.SetLogger(zap.New(zap.WriteTo(GinkgoWriter), zap.UseDevMode(true)))

	By("bootstrapping test environment")
	testEnv = &envtest.Environment{
		CRDDirectoryPaths:     []string{filepath.Join("..", "config", "crd", "bases")},
		ErrorIfCRDPathMissing: true,
	}

	var err error
	cfg, err = testEnv.Start()
	Expect(err).NotTo(HaveOccurred())
	Expect(cfg).NotTo(BeNil())

	sch := runtime.NewScheme()
	err = websitev1beta1.AddToScheme(sch)
	Expect(err).NotTo(HaveOccurred())
	err = clientgoscheme.AddToScheme(sch)
	Expect(err).NotTo(HaveOccurred())

	//+kubebuilder:scaffold:scheme

	k8sClient, err = client.New(cfg, client.Options{Scheme: sch})
	Expect(err).NotTo(HaveOccurred())
	Expect(k8sClient).NotTo(BeNil())

	mgr, err := ctrl.NewManager(cfg, ctrl.Options{
		Scheme: sch,
	})
	Expect(err).NotTo(HaveOccurred())
	err = NewWebSiteReconciler(
		k8sClient,
		ctrl.Log.WithName("controllers").WithName("WebSite"),
		sch,
		website.DefaultNginxContainerImage,
		website.DefaultRepoCheckerContainerImage,
		"website-operator-system",
		&mockRevisionClient{"rev1"},
	).SetupWithManager(mgr)
	Expect(err).NotTo(HaveOccurred())

	go func() {
		err = mgr.Start(ctrl.SetupSignalHandler())
		Expect(err).NotTo(HaveOccurred())
	}()

	ns := &corev1.Namespace{}
	ns.Name = "website-operator-system"
	err = k8sClient.Create(context.Background(), ns)
	Expect(err).NotTo(HaveOccurred())

	ns = &corev1.Namespace{}
	ns.Name = "test"
	err = k8sClient.Create(context.Background(), ns)
	Expect(err).NotTo(HaveOccurred())

}, 60)

var _ = AfterSuite(func() {
	By("tearing down the test environment")
	err := testEnv.Stop()
	Expect(err).NotTo(HaveOccurred())
})

type mockRevisionClient struct {
	rev string
}

func (c mockRevisionClient) GetLatestRevision(ctx context.Context, webSite *websitev1beta1.WebSite) (string, error) {
	return c.rev, nil
}
