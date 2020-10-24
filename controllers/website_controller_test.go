package controllers

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var _ = Describe("MySQLCluster controller", func() {

	ctx := context.Background()

	BeforeEach(func() {

	})

	Context("RepoChecker", func() {
		It("should be create ", func() {
			site := createWebSiteResource()

			isUpdated, err := reconciler.reconcileRepoChecker(ctx, site)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(isUpdated).Should(BeTrue())
		})
	})
})

func createWebSiteResource() *websitev1beta1.WebSite {
	site := &websitev1beta1.WebSite{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WebSite",
			APIVersion: websitev1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "test",
			Namespace: "default",
		},
		Spec: websitev1beta1.WebSiteSpec{
			PreBuildResources: nil,
			BuildImage:        "ghcr.io/zoetrope/node:12.19.0",
			BuildScript:       "npm run build",
			RepoURL:           "https://github.com/zoetrope/honkit-sample.git",
			Branch:            "main",
		},
	}
	err := k8sClient.Create(context.Background(), site)
	Expect(err).ShouldNot(HaveOccurred())
	return site
}
