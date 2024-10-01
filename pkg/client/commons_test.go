package client

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

type testReporter struct{}

func (r *testReporter) Report(msg string) {
	fmt.Println(msg)
}

func newLocalDebugProgram(t *testing.T, opts ...Option) (Program, context.CancelFunc) {
	RegisterTestingT(t)
	if os.Getenv("GITHUB_RUN_ID") != "" {
		t.Skipf("Not intended to run on CI")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	p, err := NewProgram(ctx, Config{
		UseProxy:       true,
		LocalDebug:     strings.HasPrefix(os.Getenv("BAAS_URL"), "http://localhost"),
		Url:            os.Getenv("BAAS_URL"),
		ApiKey:         os.Getenv("BAAS_API_KEY"),
		Timeout:        "600s",
		MessageTimeout: "120s",
	}, &testReporter{}, opts...)
	Expect(err).To(BeNil())

	return p, cancel
}
