package e2e

import (
	"errors"
	"net/http"
	"time"

	appsv1 "k8s.io/api/apps/v1"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	corev1 "k8s.io/api/core/v1"
)

func testBootstrap() {
	It("should launch honkit-sample", func() {
		var site websitev1beta1.WebSite
		Eventually(func() error {
			err := getResource("default", "website", "honkit-sample", "", &site)
			if err != nil {
				return err
			}
			if site.Status.Ready != corev1.ConditionTrue {
				return errors.New("honkit-sample should be ready")
			}
			return nil
		}, 1*time.Minute).Should(Succeed())

		var deployment appsv1.Deployment
		Eventually(func() error {
			err := getResource("default", "deployment", "honkit-sample", "", &deployment)
			if err != nil {
				return err
			}
			if deployment.Status.AvailableReplicas != 2 {
				return errors.New("should be ready")
			}
			return nil
		}, 3*time.Minute).Should(Succeed())

		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1", nil)
		Expect(err).ShouldNot(HaveOccurred())
		req.Host = "honkit-sample.default.example.com"
		client := http.Client{}
		res, err := client.Do(req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(res.StatusCode).Should(Equal(http.StatusOK))
	})
}
