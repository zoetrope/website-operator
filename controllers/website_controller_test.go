package controllers

import (
	"context"
	"fmt"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/zoetrope/website-operator"
	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/pointer"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

var _ = Describe("WebSite controller", func() {

	ctx := context.Background()
	var stopFunc func()
	var mockClient mockRevisionClient

	BeforeEach(func() {
		err := k8sClient.DeleteAllOf(ctx, &websitev1beta1.WebSite{}, client.InNamespace("test"))
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &corev1.ConfigMap{}, client.InNamespace("test"))
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &appsv1.Deployment{}, client.InNamespace("test"))
		Expect(err).NotTo(HaveOccurred())
		err = k8sClient.DeleteAllOf(ctx, &batchv1.Job{}, client.InNamespace("test"), client.PropagationPolicy(metav1.DeletePropagationBackground))
		Expect(err).NotTo(HaveOccurred())
		svcs := &corev1.ServiceList{}
		err = k8sClient.List(ctx, svcs, client.InNamespace("test"))
		Expect(err).NotTo(HaveOccurred())
		for _, svc := range svcs.Items {
			err := k8sClient.Delete(ctx, &svc)
			Expect(err).NotTo(HaveOccurred())
		}
		time.Sleep(100 * time.Millisecond)

		mgr, err := ctrl.NewManager(cfg, ctrl.Options{
			Scheme: scheme,
		})
		Expect(err).ToNot(HaveOccurred())

		mockClient = mockRevisionClient{"rev1"}
		err = NewWebSiteReconciler(
			k8sClient,
			ctrl.Log.WithName("controllers").WithName("WebSite"),
			scheme,
			website.DefaultNginxContainerImage,
			website.DefaultRepoCheckerContainerImage,
			"website-operator-system",
			&mockClient,
		).SetupWithManager(mgr)
		Expect(err).NotTo(HaveOccurred())

		ctx, cancel := context.WithCancel(ctx)
		stopFunc = cancel
		go func() {
			err := mgr.Start(ctx)
			if err != nil {
				panic(err)
			}
		}()
		time.Sleep(100 * time.Millisecond)
	})

	AfterEach(func() {
		stopFunc()
		time.Sleep(100 * time.Millisecond)
	})

	Context("BuildScript", func() {
		It("should create buildScript ConfigMap from raw data", func() {
			site := newWebSite().withRawBuildScript().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			cm := corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite-build-script"}, &cm)
			}).Should(Succeed())
			Expect(cm.Data).Should(HaveKey("build.sh"))
		})

		It("should create buildScript ConfigMap from ConfigMap", func() {
			buildScript := `#!/bin/bash -ex
cd $HOME
git clone $REPO_URL
cd $REPO_NAME
git checkout $REVISION
npm install && npm run build
cp -r _book/* $OUTPUT/
`
			bsCm := &corev1.ConfigMap{}
			bsCm.Name = "myscript"
			bsCm.Namespace = "website-operator-system"
			bsCm.Data = map[string]string{
				"script": buildScript,
			}

			err := k8sClient.Create(ctx, bsCm)
			Expect(err).NotTo(HaveOccurred())

			site := newWebSite().withConfigMapBuildScript().build()
			err = k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			cm := corev1.ConfigMap{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite-build-script"}, &cm)
			}).Should(Succeed())
			Expect(cm.Data).Should(HaveKey("build.sh"))
		})
	})

	Context("RepoChecker", func() {
		It("should create RepoChecker Deployment", func() {
			site := newWebSite().withRawBuildScript().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			dep := appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite-repo-checker"}, &dep)
			}).Should(Succeed())
			Expect(dep.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.Containers[0].Name).Should(Equal("repo-checker"))
			Expect(dep.Spec.Template.Spec.Containers[0].Command).Should(ContainElement("--repo-branch=main"))
			Expect(dep.Spec.Template.Spec.Containers[0].VolumeMounts).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.ImagePullSecrets).Should(BeEmpty())
		})

		It("should create RepoChecker Deployment with DeployKey", func() {
			site := newWebSite().withRawBuildScript().withDeployKey().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			dep := appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite-repo-checker"}, &dep)
			}).Should(Succeed())

			Expect(dep.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.Containers[0].Name).Should(Equal("repo-checker"))
			Expect(dep.Spec.Template.Spec.Containers[0].Command).Should(ContainElement("--repo-branch=main"))
			Expect(dep.Spec.Template.Spec.Containers[0].VolumeMounts).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.ImagePullSecrets).Should(BeEmpty())
		})

		It("should create RepoChecker Deployment with ImagePullSecrets", func() {
			site := newWebSite().withRawBuildScript().withImagePullSecrets().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			dep := appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite-repo-checker"}, &dep)
			}).Should(Succeed())

			Expect(dep.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.Containers[0].Name).Should(Equal("repo-checker"))
			Expect(dep.Spec.Template.Spec.Containers[0].Command).Should(ContainElement("--repo-branch=main"))
			Expect(dep.Spec.Template.Spec.Containers[0].VolumeMounts).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.ImagePullSecrets).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("myimagepullsecret")})))
		})

		It("should create RepoChecker Service", func() {
			site := newWebSite().withRawBuildScript().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			svc := corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite-repo-checker"}, &svc)
			}).Should(Succeed())
		})
	})

	Context("Nginx", func() {
		It("should create Nginx Deployment", func() {
			site := newWebSite().withRawBuildScript().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			dep := appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite"}, &dep)
			}).Should(Succeed())
			Expect(*dep.Spec.Replicas).Should(BeNumerically("==", 1))
			Expect(dep.Spec.Template.Labels).Should(HaveLen(3))
			Expect(dep.Spec.Template.Annotations).Should(HaveLen(1))
			Expect(dep.Spec.Template.Annotations).Should(HaveKey(AnnChecksumConfig))
			Expect(dep.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.InitContainers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.InitContainers[0].VolumeMounts).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.InitContainers[0].Env).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("REVISION"), "Value": Equal("rev1")})))
			Expect(dep.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("build-script")})))
			Expect(dep.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.ImagePullSecrets).Should(BeEmpty())
		})

		It("should create Nginx Deployment with DeployKey", func() {
			site := newWebSite().withRawBuildScript().withDeployKey().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			dep := appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite"}, &dep)
			}).Should(Succeed())
			Expect(*dep.Spec.Replicas).Should(BeNumerically("==", 1))
			Expect(dep.Spec.Template.Labels).Should(HaveLen(3))
			Expect(dep.Spec.Template.Annotations).Should(HaveLen(1))
			Expect(dep.Spec.Template.Annotations).Should(HaveKey(AnnChecksumConfig))
			Expect(dep.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.InitContainers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.InitContainers[0].VolumeMounts).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.InitContainers[0].Env).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("REVISION"), "Value": Equal("rev1")})))
			Expect(dep.Spec.Template.Spec.InitContainers[0].Env).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("VAR_KEY")})))
			Expect(dep.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("build-script")})))
			Expect(dep.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.ImagePullSecrets).Should(BeEmpty())
		})

		It("should create Nginx Deployment with ImageSecretes", func() {
			site := newWebSite().withRawBuildScript().withImagePullSecrets().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			dep := appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite"}, &dep)
			}).Should(Succeed())
			Expect(*dep.Spec.Replicas).Should(BeNumerically("==", 1))
			Expect(dep.Spec.Template.Labels).Should(HaveLen(3))
			Expect(dep.Spec.Template.Annotations).Should(HaveLen(1))
			Expect(dep.Spec.Template.Annotations).Should(HaveKey(AnnChecksumConfig))
			Expect(dep.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.InitContainers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.InitContainers[0].VolumeMounts).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.InitContainers[0].Env).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("REVISION"), "Value": Equal("rev1")})))
			Expect(dep.Spec.Template.Spec.InitContainers[0].Env).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("VAR_KEY")})))
			Expect(dep.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("build-script")})))
			Expect(dep.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.ImagePullSecrets).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("myimagepullsecret")})))
		})

		It("should create Nginx Deployment with BuildSecretes", func() {
			site := newWebSite().withRawBuildScript().withBuildSecrets().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			dep := appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite"}, &dep)
			}).Should(Succeed())
			Expect(*dep.Spec.Replicas).Should(BeNumerically("==", 1))
			Expect(dep.Spec.Template.Labels).Should(HaveLen(3))
			Expect(dep.Spec.Template.Annotations).Should(HaveLen(1))
			Expect(dep.Spec.Template.Annotations).Should(HaveKey(AnnChecksumConfig))
			Expect(dep.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.InitContainers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.InitContainers[0].VolumeMounts).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.InitContainers[0].Env).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("REVISION"), "Value": Equal("rev1")})))
			Expect(dep.Spec.Template.Spec.InitContainers[0].Env).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("VAR_KEY"), "ValueFrom": Equal(&corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "mybuildsecret"}, Key: "VAR_KEY"}})})))
			Expect(dep.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("build-script")})))
			Expect(dep.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.ImagePullSecrets).Should(BeEmpty())
		})

		It("should create Nginx Deployment with Replicas", func() {
			site := newWebSite().withRawBuildScript().withReplicas(3).build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			dep := appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite"}, &dep)
			}).Should(Succeed())
			Expect(*dep.Spec.Replicas).Should(BeNumerically("==", 3))
			Expect(dep.Spec.Template.Labels).Should(HaveLen(3))
			Expect(dep.Spec.Template.Annotations).Should(HaveLen(1))
			Expect(dep.Spec.Template.Annotations).Should(HaveKey(AnnChecksumConfig))
			Expect(dep.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.InitContainers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.InitContainers[0].VolumeMounts).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.InitContainers[0].Env).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("REVISION"), "Value": Equal("rev1")})))
			Expect(dep.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("build-script")})))
			Expect(dep.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.ImagePullSecrets).Should(BeEmpty())
		})

		It("should create Nginx Deployment with PodTemplate", func() {
			site := newWebSite().withRawBuildScript().withPodTemplate().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			dep := appsv1.Deployment{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite"}, &dep)
			}).Should(Succeed())
			Expect(*dep.Spec.Replicas).Should(BeNumerically("==", 1))
			Expect(dep.Spec.Template.Labels).Should(HaveLen(4))
			Expect(dep.Spec.Template.Labels).Should(HaveKey("mylabel"))
			Expect(dep.Spec.Template.Annotations).Should(HaveLen(2))
			Expect(dep.Spec.Template.Annotations).Should(HaveKey("myann"))
			Expect(dep.Spec.Template.Annotations).Should(HaveKey(AnnChecksumConfig))
			Expect(dep.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.InitContainers).Should(HaveLen(1))
			Expect(dep.Spec.Template.Spec.InitContainers[0].VolumeMounts).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.InitContainers[0].Env).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("REVISION"), "Value": Equal("rev1")})))
			Expect(dep.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("build-script")})))
			Expect(dep.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(dep.Spec.Template.Spec.ImagePullSecrets).Should(BeEmpty())
		})

		It("should create Nginx Service with ServiceTemplate", func() {
			site := newWebSite().withRawBuildScript().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			svc := corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite"}, &svc)
			}).Should(Succeed())
			Expect(svc.Labels).Should(HaveLen(2))
			Expect(svc.Annotations).Should(HaveLen(0))
		})

		It("should create Nginx Service with ServiceTemplate", func() {
			site := newWebSite().withRawBuildScript().withServiceTemplate().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			svc := corev1.Service{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite"}, &svc)
			}).Should(Succeed())
			Expect(svc.Labels).Should(HaveLen(3))
			Expect(svc.Labels).Should(HaveKey("mylabel"))
			Expect(svc.Annotations).Should(HaveLen(1))
			Expect(svc.Annotations).Should(HaveKey("myann"))
		})
	})

	Context("ExtraResources", func() {
		It("should create extraResources", func() {
			site := newWebSite().withRawBuildScript().withExtraResources().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

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
			err = k8sClient.Create(ctx, &cm)
			Expect(err).ShouldNot(HaveOccurred())

			pod := corev1.Pod{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: site.Namespace, Name: site.Name + "-ubuntu"}, &pod)
			}).Should(Succeed())
		})
	})

	Context("AfterBuildcript", func() {
		It("should create afterBuildScript job", func() {
			site := newWebSite().withRawBuildScript().withAfterBuildScript().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			job := batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite"}, &job)
			}).Should(Succeed())

			Expect(job.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(job.Spec.Template.Labels).Should(HaveLen(2))
			Expect(job.Spec.Template.Annotations).Should(HaveLen(1))
			Expect(job.Spec.Template.Annotations).Should(HaveKey(AnnChecksumConfig))
			Expect(job.Spec.Template.Spec.Containers[0].Env).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("REVISION"), "Value": Equal("rev1")})))
			Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(job.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("after-build-script")})))
			Expect(job.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(job.Spec.Template.Spec.ImagePullSecrets).Should(BeEmpty())
		})

		It("should create afterBuildscript job with deploy key", func() {
			site := newWebSite().withRawBuildScript().withDeployKey().withAfterBuildScript().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			job := batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite"}, &job)
			}).Should(Succeed())

			Expect(job.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(job.Spec.Template.Labels).Should(HaveLen(2))
			Expect(job.Spec.Template.Annotations).Should(HaveLen(1))
			Expect(job.Spec.Template.Annotations).Should(HaveKey(AnnChecksumConfig))
			Expect(job.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(job.Spec.Template.Spec.Containers[0].Env).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("REVISION"), "Value": Equal("rev1")})))
			Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(job.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("after-build-script")})))
			Expect(job.Spec.Template.Spec.ImagePullSecrets).Should(BeEmpty())
		})

		It("should create afterBuildScirpt job with Image Secrets", func() {
			site := newWebSite().withRawBuildScript().withImagePullSecrets().withAfterBuildScript().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			job := batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite"}, &job)
			}).Should(Succeed())

			Expect(job.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(job.Spec.Template.Labels).Should(HaveLen(2))
			Expect(job.Spec.Template.Annotations).Should(HaveLen(1))
			Expect(job.Spec.Template.Annotations).Should(HaveKey(AnnChecksumConfig))
			Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(job.Spec.Template.Spec.Containers[0].Env).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("REVISION"), "Value": Equal("rev1")})))
			Expect(job.Spec.Template.Spec.Containers[0].Env).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("VAR_KEY")})))
			Expect(job.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("after-build-script")})))
			Expect(job.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(job.Spec.Template.Spec.ImagePullSecrets).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("myimagepullsecret")})))
		})

		It("should create afterBuildscript job with Build Secrets", func() {
			site := newWebSite().withRawBuildScript().withBuildSecrets().withAfterBuildScript().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			job := batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite"}, &job)
			}).Should(Succeed())

			Expect(job.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(job.Spec.Template.Labels).Should(HaveLen(2))
			Expect(job.Spec.Template.Annotations).Should(HaveLen(1))
			Expect(job.Spec.Template.Annotations).Should(HaveKey(AnnChecksumConfig))
			Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(job.Spec.Template.Spec.Containers[0].Env).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("REVISION"), "Value": Equal("rev1")})))
			Expect(job.Spec.Template.Spec.Containers[0].Env).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("VAR_KEY"), "ValueFrom": Equal(&corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{LocalObjectReference: corev1.LocalObjectReference{Name: "mybuildsecret"}, Key: "VAR_KEY"}})})))
			Expect(job.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("after-build-script")})))
			Expect(job.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(job.Spec.Template.Spec.ImagePullSecrets).Should(BeEmpty())
		})

		It("should recreate afterBuildscript job when job exists and revision is updated ", func() {
			site := newWebSite().withRawBuildScript().withAfterBuildScript().build()
			err := k8sClient.Create(ctx, site)
			Expect(err).NotTo(HaveOccurred())

			job := batchv1.Job{}
			Eventually(func() error {
				return k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite"}, &job)
			}).Should(Succeed())

			Expect(job.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(job.Spec.Template.Labels).Should(HaveLen(2))
			Expect(job.Spec.Template.Annotations).Should(HaveLen(1))
			Expect(job.Spec.Template.Annotations).Should(HaveKey(AnnChecksumConfig))
			Expect(job.Spec.Template.Spec.Containers[0].Env).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("REVISION"), "Value": Equal("rev1")})))
			Expect(job.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("after-build-script")})))
			Expect(job.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(job.Spec.Template.Spec.Containers[0].VolumeMounts).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(job.Spec.Template.Spec.ImagePullSecrets).Should(BeEmpty())

			mockClient.rev = "rev2"
			newJob := batchv1.Job{}
			Eventually(func() (bool, error) {
				err := k8sClient.Get(ctx, client.ObjectKey{Namespace: "test", Name: "mysite"}, &newJob)
				if err != nil {
					return false, fmt.Errorf("error %v", err)
				}
				return newJob.ObjectMeta.UID != job.ObjectMeta.UID, nil
			}, 60).Should(BeTrue())

			Expect(newJob.Spec.Template.Spec.Containers).Should(HaveLen(1))
			Expect(job.Spec.Template.Labels).Should(HaveLen(2))
			Expect(newJob.Spec.Template.Annotations).Should(HaveLen(1))
			Expect(newJob.Spec.Template.Annotations).Should(HaveKey(AnnChecksumConfig))
			Expect(newJob.Spec.Template.Spec.Containers[0].Env).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("REVISION"), "Value": Equal("rev2")})))
			Expect(newJob.Spec.Template.Spec.Volumes).Should(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("after-build-script")})))
			Expect(newJob.Spec.Template.Spec.Volumes).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(newJob.Spec.Template.Spec.Containers[0].VolumeMounts).ShouldNot(ContainElement(MatchFields(IgnoreExtras, Fields{"Name": Equal("deploy-key")})))
			Expect(newJob.Spec.Template.Spec.ImagePullSecrets).Should(BeEmpty())
		})
	})
})

type websiteBuilder struct {
	website *websitev1beta1.WebSite
}

func (b *websiteBuilder) build() *websitev1beta1.WebSite {
	return b.website
}

func newWebSite() *websiteBuilder {
	site := &websitev1beta1.WebSite{
		TypeMeta: metav1.TypeMeta{
			Kind:       "WebSite",
			APIVersion: websitev1beta1.GroupVersion.String(),
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      "mysite",
			Namespace: "test",
		},
		Spec: websitev1beta1.WebSiteSpec{
			BuildImage: "ghcr.io/zoetrope/node:16.13.2",
			RepoURL:    "https://github.com/zoetrope/honkit-sample.git",
			Branch:     "main",
		},
	}
	return &websiteBuilder{site}
}

func (b *websiteBuilder) withRawBuildScript() *websiteBuilder {
	buildScript := `#!/bin/bash -ex
cd $HOME
git clone $REPO_URL
cd $REPO_NAME
git checkout $REVISION
npm install && npm run build
cp -r _book/* $OUTPUT/
`
	b.website.Spec.BuildScript = websitev1beta1.DataSource{
		RawData: &buildScript,
	}

	return b
}

func (b *websiteBuilder) withAfterBuildScript() *websiteBuilder {
	afterBuildScript := `    #!/bin/bash -ex
	cd $HOME
	rm -rf $REPO_NAME
	git clone $REPO_URL
	cd $REPO_NAME
	git checkout $REVISION
	sed -i -e "/host/c\      \"host\": \"http://${RESOURCE_NAME}.${RESOURCE_NAMESPACE}.example.com/es\"," book.js
	sed -i -e "/index/c\      \"index\": \"${RESOURCE_NAME}-${REVISION}\"," book.js
	npm install
	npm run build
	curl -X DELETE ${ELASTIC_HOST}/${RESOURCE_NAME}-${REVISION}
	curl -X PUT ${ELASTIC_HOST}/${RESOURCE_NAME}-${REVISION} -H 'Content-Type: application/json' -d @mappings.json
	curl -X POST ${ELASTIC_HOST}/${RESOURCE_NAME}-${REVISION}/_bulk -H 'Content-Type: application/json' --data-binary @_book/search_index.json
`
	b.website.Spec.AfterBuildScript = &websitev1beta1.DataSource{
		RawData: &afterBuildScript,
	}

	return b
}

func (b *websiteBuilder) withConfigMapBuildScript() *websiteBuilder {
	b.website.Spec.BuildScript = websitev1beta1.DataSource{
		ConfigMap: &websitev1beta1.ConfigMapSource{
			Name:      "myscript",
			Namespace: "website-operator-system",
			Key:       "script",
		},
	}

	return b
}

func (b *websiteBuilder) withDeployKey() *websiteBuilder {
	b.website.Spec.DeployKeySecretName = pointer.StringPtr("mydeploykey")
	return b
}

func (b *websiteBuilder) withImagePullSecrets() *websiteBuilder {
	b.website.Spec.ImagePullSecrets = []corev1.LocalObjectReference{
		{
			Name: "myimagepullsecret",
		},
	}
	return b
}

func (b *websiteBuilder) withBuildSecrets() *websiteBuilder {
	b.website.Spec.BuildSecrets = []websitev1beta1.SecretKey{
		{
			Name: "mybuildsecret",
			Key:  "VAR_KEY",
		},
	}
	return b
}

func (b *websiteBuilder) withReplicas(rep int32) *websiteBuilder {
	b.website.Spec.Replicas = rep
	return b
}

func (b *websiteBuilder) withPodTemplate() *websiteBuilder {
	b.website.Spec.PodTemplate = &websitev1beta1.PodTemplate{
		ObjectMeta: websitev1beta1.ObjectMeta{
			Labels: map[string]string{
				"mylabel": "foo",
			},
			Annotations: map[string]string{
				"myann": "bar",
			},
		},
	}
	return b
}

func (b *websiteBuilder) withServiceTemplate() *websiteBuilder {
	b.website.Spec.ServiceTemplate = &websitev1beta1.ServiceTemplate{
		ObjectMeta: websitev1beta1.ObjectMeta{
			Labels: map[string]string{
				"mylabel": "foo",
			},
			Annotations: map[string]string{
				"myann": "bar",
			},
		},
	}
	return b
}

func (b *websiteBuilder) withExtraResources() *websiteBuilder {
	b.website.Spec.ExtraResources = []websitev1beta1.DataSource{
		{
			ConfigMap: &websitev1beta1.ConfigMapSource{
				Name:      "my-templates",
				Namespace: "test",
				Key:       "ubuntu",
			},
		},
	}
	return b
}
