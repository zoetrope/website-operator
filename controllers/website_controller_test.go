package controllers

import (
	"context"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("WebSite controller", func() {

	ctx := context.Background()

	BeforeEach(func() {

	})

	Context("RepoChecker", func() {
		It("should create RepoChecker Deployment", func() {
			site := createWebSiteResource()

			isUpdated, err := reconciler.reconcileRepoCheckerDeployment(ctx, site)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(isUpdated).Should(BeTrue())

			isUpdated, err = reconciler.reconcileRepoCheckerDeployment(ctx, site)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(isUpdated).Should(BeFalse())
		})
	})

	Context("ExtraResources", func() {
		It("should create extraResources", func() {
			site := createWebSiteResource()
			site.Spec.ExtraResources = append(site.Spec.ExtraResources, websitev1beta1.DataSource{
				ConfigMap: &websitev1beta1.ConfigMapSource{
					Name: "my-templates",
					Key:  "ubuntu",
				},
			})

			cm := corev1.ConfigMap{}
			cm.Namespace = site.Namespace
			cm.Name = "my-templates"
			cm.Data = map[string]string{
				"ubuntu": `apiVersion: v1
kind: Pod
metadata:
  name: {{.ResourceName}}-ubuntu
  namespace: unknown
spec:
  containers:
  - name: ubuntu
    image: ghcr.io/zoetrope/ubuntu:20.04
    command: ["/usr/local/bin/pause"]
`,
			}
			err := k8sClient.Create(ctx, &cm)
			Expect(err).ShouldNot(HaveOccurred())

			isUpdated, err := reconciler.reconcileExtraResources(ctx, site)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(isUpdated).Should(BeTrue())

			pod := corev1.Pod{}
			err = k8sClient.Get(ctx, client.ObjectKey{Namespace: site.Namespace, Name: site.Name + "-ubuntu"}, &pod)
			Expect(err).ShouldNot(HaveOccurred())

			isUpdated, err = reconciler.reconcileExtraResources(ctx, site)
			Expect(err).ShouldNot(HaveOccurred())
			Expect(isUpdated).Should(BeFalse())
		})
	})
})

func createWebSiteResource() *websitev1beta1.WebSite {
	buildScript := `#!/bin/bash -ex
cd $HOME
git clone $REPO_URL
cd $REPO_NAME
git checkout $REVISION
npm install && npm run build
cp -r _book/* $OUTPUT/
`
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
			ExtraResources: nil,
			BuildImage:     "ghcr.io/zoetrope/node:12.19.0",
			BuildScript: websitev1beta1.DataSource{
				RawData: &buildScript,
			},
			RepoURL: "https://github.com/zoetrope/honkit-sample.git",
			Branch:  "main",
		},
	}
	_, err := ctrl.CreateOrUpdate(context.Background(), k8sClient, site, func() error {
		return nil
	})
	Expect(err).ShouldNot(HaveOccurred())
	return site
}
