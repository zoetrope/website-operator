package e2e

import (
	"errors"
	"net/http"

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
		}).Should(Succeed())

		_, err := kubectl(nil, "wait", "pod", "-l", "app.kubernetes.io/instance=honkit-sample", "--for", "condition=Ready", "--timeout=120s")
		Expect(err).ShouldNot(HaveOccurred())

		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1", nil)
		Expect(err).ShouldNot(HaveOccurred())
		req.Host = "honkit-sample.default.example.com"
		client := http.Client{}
		res, err := client.Do(req)
		Expect(err).ShouldNot(HaveOccurred())
		Expect(res.StatusCode).Should(Equal(http.StatusOK))
	})
}
