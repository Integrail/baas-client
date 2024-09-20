package client

import (
	"fmt"
	"os"
	"testing"

	. "github.com/onsi/gomega"
)

func TestPerfStudioLogin(t *testing.T) {
	secrets := map[string]string{
		"username": os.Getenv("STUDIO_USERNAME"),
		"password": os.Getenv("STUDIO_PASSWORD"),
	}
	p, cancel := newLocalDebugProgram(t, WithSecrets(secrets))
	defer cancel()

	s, err := p.NavigateStatus("https://perf-studio.integrail.ai")
	Expect(err).To(BeNil())
	Expect(s).To(Equal(200))

	err = p.LlmLogin(secrets["username"], secrets["password"])
	Expect(err).To(BeNil())

	err = p.WaitReady("body")
	Expect(err).To(BeNil())

	err = p.Sleep("5s")
	Expect(err).To(BeNil())

	html, err := p.FindVisibleElements([]string{"p", "div", "span", "input"}, "data-llm-id")
	Expect(err).To(BeNil())
	Expect(html).NotTo(BeEmpty())
	fmt.Println(html)

	err = p.SaveScreenshot("studio", "screenshots/studio.png")
	Expect(err).To(BeNil())
}
