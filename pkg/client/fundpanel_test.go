package client

import (
	"os"
	"testing"

	. "github.com/onsi/gomega"
)

func TestFundpanelDownload(t *testing.T) {
	secrets := map[string]string{
		"email":    os.Getenv("FUNDPANEL_EMAIL"),
		"password": os.Getenv("FUNDPANEL_PASSWORD"),
	}
	values := map[string]string{
		"url": os.Getenv("FUNDPANEL_URL"),
	}
	p, cancel := newLocalDebugProgram(t, WithSecrets(secrets), WithValues(values))
	defer cancel()

	s, err := p.NavigateStatus(values["url"])
	Expect(err).To(BeNil())
	Expect(s).To(Equal(200))

	err = p.LlmLogin(secrets["email"], secrets["password"])
	Expect(err).To(BeNil())

	err = p.WaitReady("body")
	Expect(err).To(BeNil())

	err = p.SaveScreenshot("studio", "screenshots/fundpanel.png")
	Expect(err).To(BeNil())

	err = p.LlmSetValueSkipVerify("authenticationcode", read2FACode())
	Expect(err).To(BeNil())

	err = p.Sleep("2s")
	Expect(err).To(BeNil())

	err = p.SaveScreenshot("studio", "screenshots/fundpanel-aferpin.png")
	Expect(err).To(BeNil())

	// TODO : click Documents, collect table and click download
}

func read2FACode() string {
	codeBytes, _ := os.ReadFile("2facode.txt")
	return string(codeBytes)
}
