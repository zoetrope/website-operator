package e2e

import (
	"bytes"
	"encoding/json"
	"os"
	"os/exec"
	"testing"
	"time"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"
)

func TestE2E(t *testing.T) {
	if os.Getenv("E2E_TEST") == "" {
		t.Skip("Run under e2e/")
	}
	RegisterFailHandler(Fail)
	SetDefaultEventuallyTimeout(5 * time.Minute)
	SetDefaultEventuallyPollingInterval(1 * time.Second)
	RunSpecs(t, "E2E Suite")
}

func kubectl(input []byte, args ...string) (stdout []byte, err error) {
	cmd := exec.Command("./bin/kubectl", args...)
	if input != nil {
		cmd.Stdin = bytes.NewReader(input)
	}

	return cmd.Output()
}

func getResource(ns, resource, name, label string, obj interface{}) error {
	var args []string
	if ns != "" {
		args = append(args, "-n", ns)
	}
	args = append(args, "get", resource)
	if name != "" {
		args = append(args, name)
	}
	if label != "" {
		args = append(args, "-l", label)
	}
	args = append(args, "-o", "json")
	data, err := kubectl(nil, args...)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, obj)
}

var _ = Describe("website-operator", func() {
	Context("bootstrap", testBootstrap)
})
