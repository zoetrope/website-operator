package e2e

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
)

func testWebSite(name string) {
	var site websitev1beta1.WebSite
	Eventually(func() error {
		err := getResource("default", "website", name, "", &site)
		if err != nil {
			return err
		}
		if site.Status.Ready != corev1.ConditionTrue {
			return fmt.Errorf("%s should be ready", name)
		}
		return nil
	}, 3*time.Minute).Should(Succeed())

	var deployment appsv1.Deployment
	Eventually(func() error {
		err := getResource("default", "deployment", name, "", &deployment)
		if err != nil {
			return err
		}
		if deployment.Status.AvailableReplicas != 1 {
			return errors.New("should be ready")
		}
		return nil
	}, 5*time.Minute).Should(Succeed())

	req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1", nil)
	Expect(err).ShouldNot(HaveOccurred())
	req.Host = name + ".default.example.com"
	client := http.Client{}
	Eventually(func() error {
		res, err := client.Do(req)
		if err != nil {
			return err
		}
		if res.StatusCode != http.StatusOK {
			return fmt.Errorf("status should be ok: %s", res.Status)
		}
		return nil
	}, 30*time.Second).Should(Succeed())
}

func testBootstrap() {
	It("should launch honkit-sample", func() {
		testWebSite("honkit-sample")
	})

	It("should launch mkdocs-sample", func() {
		testWebSite("mkdocs-sample")
	})

	It("should launch gatsby-sample", func() {
		testWebSite("gatsby-sample")
	})
}
