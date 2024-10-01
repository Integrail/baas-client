package client

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
	"time"

	. "github.com/onsi/gomega"
)

var (
	selectorRegexp = regexp.MustCompile("`(.+)`")
	quotesRegexp   = regexp.MustCompile("\"(.+)\"")
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

func llmExtractSingleSelectorFromResponse(resp string) string {
	selector := resp
	if extracted := selectorRegexp.FindAllStringSubmatch(resp, 1); selectorRegexp.MatchString(resp) && len(extracted) > 0 {
		selector = strings.TrimSpace(extracted[0][1])
	}
	if extracted := quotesRegexp.FindAllStringSubmatch(resp, 1); quotesRegexp.MatchString(resp) && len(extracted) > 0 {
		selector = strings.TrimSpace(extracted[0][1])
	}
	selector = strings.TrimSpace(strings.Split(selector, "\n")[0])
	multiSelector := strings.Split(selector, ",")
	if len(multiSelector) > 1 {
		selector = multiSelector[0]
	}
	return selector
}
