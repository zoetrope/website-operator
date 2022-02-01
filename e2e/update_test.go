package e2e

import (
	"errors"
	"fmt"
	"net/http"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	websitev1beta1 "github.com/zoetrope/website-operator/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
)

func testUpdate() {
	It("should update honkit-sample", func() {
		var site websitev1beta1.WebSite
		err := getResource("default", "website", "honkit-sample", "", &site)
		Expect(err).ShouldNot(HaveOccurred())
		revBeforeUpdate := site.Status.Revision

		_, err = kubectl(nil, "patch", "websites", "honkit-sample", "--type=json", "-p", `[{"op": "replace", "path": "/spec/branch", "value": "new-page"}]`)
		Expect(err).ShouldNot(HaveOccurred())

		Eventually(func() error {
			err := getResource("default", "website", "honkit-sample", "", &site)
			if err != nil {
				return err
			}
			if site.Status.Revision == revBeforeUpdate {
				return errors.New("should be updated")
			}
			return nil
		}, 3*time.Minute).Should(Succeed())

		var deployment appsv1.Deployment
		Eventually(func() error {
			err := getResource("default", "deployment", "honkit-sample", "", &deployment)
			if err != nil {
				return err
			}
			if deployment.Status.UpdatedReplicas != 1 {
				return errors.New("should be updated")
			}
			return nil
		}, 2*time.Minute).Should(Succeed())

		req, err := http.NewRequest(http.MethodGet, "http://127.0.0.1/newpage.html", nil)
		Expect(err).ShouldNot(HaveOccurred())
		req.Host = "honkit-sample.default.example.com"
		client := http.Client{}
		Eventually(func() error {
			res, err := client.Do(req)
			if err != nil {
				return err
			}
			if res.StatusCode != http.StatusOK {
				return fmt.Errorf("failed to update: %d", res.StatusCode)
			}
			return nil
		}, 1*time.Minute).Should(Succeed())
	})
}
