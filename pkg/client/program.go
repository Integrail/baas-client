package client

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/savioxavier/termlink"

	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/integrail/baas-client/pkg/client/dto"
)

type Program interface {
	Error() error
	NavigateStatus(url string) (int, error)
	TakeScreenshot(name string) ([]byte, error)
	LlmSetValue(desc, value string) error
	LlmSetValueSkipVerify(desc, value string) error
	LlmLogin(username, password string) error
	GetURL() (string, error)
	Click(selector string) error
	GetInnerText(selector string) (string, error)
	GetSecret(name string) (string, error)
	GetValue(name string) (string, error)
	InnerHtml(selector string) error
	IsElementPresent(selector string) (bool, error)
	LlmClick(description string) error
	LlmSendKeys(description, value string) error
	LlmText(description string) (string, error)
	Log(message string) error
	LogURL() error
	Navigate(url string) error
	OuterHtml(selector string) error
	ReplaceInnerHtml(selector, html string) error
	SendKeys(text string) error
	Sleep(duration string) error
	Submit(selector string) error
	Text(selector string) (string, error)
	WaitFileDownload(duration string) (bool, error)
	WaitReady(selector string) error
	WaitVisible(selector string) error
	SaveScreenshot(name string, fileName string) error
	FindVisibleElements(elements []string, attributeName string) (string, error)
}

type Reporter interface {
	Report(msg string)
}

type Config struct {
	UseProxy       bool     `json:"useProxy" yaml:"useProxy"`
	LocalDebug     bool     `json:"localDebug" yaml:"localDebug"`
	Url            string   `json:"url" yaml:"url"`
	ApiKey         string   `json:"apiKey" yaml:"apiKey"`
	Timeout        string   `json:"timeout" yaml:"timeout"`
	MessageTimeout string   `json:"messageTimeout" yaml:"messageTimeout"`
	Secrets        []string `json:"secrets" yaml:"secrets"`
	Values         []string `json:"values" yaml:"values"`
}

type Option func(p *program)

func WithSecrets(secrets map[string]string) Option {
	return func(p *program) {
		p.secrets = secrets
	}
}

func WithValues(values map[string]string) Option {
	return func(p *program) {
		p.values = values
	}
}

func NewProgram(ctx context.Context, cfg Config, reporter Reporter, opts ...Option) (Program, error) {
	client := NewClient(cfg.Url, cfg.ApiKey, time.Second*30)
	ctx, cancel := context.WithCancel(ctx)

	p := &program{
		client:   client,
		ctx:      ctx,
		cancel:   cancel,
		reporter: reporter,
	}

	for _, opt := range opts {
		opt(p)
	}

	go func() {
		defer cancel()
		res, wait, err := client.RunAsync(ctx, dto.Config{
			Browser: dto.BrowserOpts{
				Headful:          cfg.LocalDebug,
				ReturnScreenshot: lo.ToPtr(true),
				Timeout:          cfg.Timeout,
			},
			UseRandomProxy: lo.ToPtr(cfg.UseProxy),
		})
		if err != nil {
			p.exitWithError(err)
			return
		}
		if res.Error != "" {
			p.exitWithError(errors.Errorf("%s", res.Error))
			return
		}
		p.sessionID = res.SessionID
		wait()
	}()

	// wait until ready
	for p.sessionID == "" {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
			reporter.Report("Waiting for sessionID...")
			time.Sleep(200 * time.Millisecond)
		}
	}
	reporter.Report("Got sessionID: " + p.sessionID)

	return p, nil
}

type program struct {
	client    Client
	err       error
	ctx       context.Context
	cancel    func()
	sessionID string
	reporter  Reporter
	secrets   map[string]string
	values    map[string]string
}

func (p *program) Error() error {
	return p.err
}

func (p *program) exitWithError(err error) {
	p.err = err
	if p.cancel != nil {
		p.cancel()
	}
}

func (p *program) runProgram(prog string) (*dto.BrowserMessageOut, error) {
	p.reporter.Report(fmt.Sprintf("Executing %q...", prog))
	res, err := p.client.Message(p.ctx, dto.BrowserMessageIn{
		SessionID: p.sessionID,
		Program:   prog,
		Secrets:   p.secrets,
		Values:    p.values,
		Timeout:   "60s",
	})
	p.reporter.Report(fmt.Sprintf("Got result: %v (%s), %v", lo.FromPtr(res).Value, lo.FromPtr(res).Error, err))
	if err != nil {
		return nil, err
	}
	if res.Error != "" {
		return nil, errors.Errorf("%s", res.Error)
	}
	return res, nil
}

func (p *program) Click(selector string) error {
	_, err := p.runProgram(fmt.Sprintf("click('%s')", selector))
	if err != nil {
		return err
	}
	return nil
}

func (p *program) GetInnerText(selector string) (string, error) {
	res, err := p.runProgram(fmt.Sprintf("getInnerText('%s')", selector))
	if err != nil {
		return "", err
	}
	return res.Value.(string), nil
}

func (p *program) GetSecret(name string) (string, error) {
	res, err := p.runProgram(fmt.Sprintf("getSecret('%s')", name))
	if err != nil {
		return "", err
	}
	return res.Value.(string), nil
}

func (p *program) GetValue(name string) (string, error) {
	res, err := p.runProgram(fmt.Sprintf("getValue('%s')", name))
	if err != nil {
		return "", err
	}
	return res.Value.(string), nil
}

func (p *program) InnerHtml(selector string) error {
	_, err := p.runProgram(fmt.Sprintf("innerHtml('%s')", selector))
	return err
}

func (p *program) IsElementPresent(selector string) (bool, error) {
	res, err := p.runProgram(fmt.Sprintf("isElementPresent('%s')", selector))
	if err != nil {
		return false, err
	}
	return res.Value.(bool), nil
}

func (p *program) LlmClick(description string) error {
	_, err := p.runProgram(fmt.Sprintf("llmClick('%s')", description))
	return err
}

func (p *program) LlmSendKeys(description, value string) error {
	_, err := p.runProgram(fmt.Sprintf("llmSendKeys('%s', '%s')", description, value))
	return err
}

func (p *program) FindVisibleElements(elements []string, addAttributeName string) (string, error) {
	res, err := p.runProgram(fmt.Sprintf("findVisibleElements('%s','%s')", strings.Join(elements, ","), addAttributeName))
	if err != nil {
		return "", err
	}
	return res.Value.(string), nil
}

func (p *program) LlmText(description string) (string, error) {
	res, err := p.runProgram(fmt.Sprintf("llmText('%s')", description))
	if err != nil {
		return "", err
	}
	return res.Value.(string), nil
}

func (p *program) Log(message string) error {
	_, err := p.runProgram(fmt.Sprintf("log('%s')", message))
	return err
}

func (p *program) LogURL() error {
	_, err := p.runProgram("logURL()")
	return err
}

func (p *program) Navigate(url string) error {
	_, err := p.runProgram(fmt.Sprintf("navigate('%s')", url))
	return err
}

func (p *program) OuterHtml(selector string) error {
	_, err := p.runProgram(fmt.Sprintf("outerHtml('%s')", selector))
	return err
}

func (p *program) ReplaceInnerHtml(selector, html string) error {
	_, err := p.runProgram(fmt.Sprintf("replaceInnerHtml('%s', '%s')", selector, html))
	return err
}

func (p *program) SendKeys(text string) error {
	_, err := p.runProgram(fmt.Sprintf("sendKeys('%s')", text))
	return err
}

func (p *program) Sleep(duration string) error {
	_, err := p.runProgram(fmt.Sprintf("sleep('%s')", duration))
	return err
}

func (p *program) Submit(selector string) error {
	_, err := p.runProgram(fmt.Sprintf("submit('%s')", selector))
	return err
}

func (p *program) Text(selector string) (string, error) {
	res, err := p.runProgram(fmt.Sprintf("text('%s')", selector))
	if err != nil {
		return "", err
	}
	return res.Value.(string), nil
}

func (p *program) WaitFileDownload(duration string) (bool, error) {
	res, err := p.runProgram(fmt.Sprintf("waitFileDownload('%s')", duration))
	if err != nil {
		return false, err
	}
	return res.Value.(bool), nil
}

func (p *program) WaitReady(selector string) error {
	_, err := p.runProgram(fmt.Sprintf("waitReady('%s')", selector))
	return err
}

func (p *program) WaitVisible(selector string) error {
	_, err := p.runProgram(fmt.Sprintf("waitVisible('%s')", selector))
	return err
}

func (p *program) NavigateStatus(url string) (int, error) {
	res, err := p.runProgram(fmt.Sprintf("navigateStatus('%s')", url))
	if err != nil {
		return 0, err
	}
	status, ok := res.Value.(float64)
	if !ok {
		return 0, errors.Errorf("Failed to convert status code to int %v", res.Value)
	}
	return int(status), nil
}

func (p *program) TakeScreenshot(name string) ([]byte, error) {
	res, err := p.runProgram(fmt.Sprintf("takeScreenshot('%s')", name))
	if err != nil {
		return nil, err
	}
	if len(res.Screenshots[name]) == 0 {
		return nil, errors.Errorf("screenshot with name %s wasn't returned", name)
	}
	return res.Screenshots[name], nil
}

func (p *program) SaveScreenshot(name string, fileName string) error {
	screenshot, err := p.TakeScreenshot(name)
	if err != nil {
		return err
	}
	var message string
	if err := os.WriteFile(fileName, screenshot, 0o644); err != nil {
		message = fmt.Sprintf("failed to save %q to %s: %q", name, fileName, err.Error())
		p.reporter.Report(message)
		return err
	} else {
		message = fmt.Sprintf("%q saved to ", name) +
			termlink.ColorLink(name, fmt.Sprintf("file://%s", fileName), "italic green")
		p.reporter.Report(message)
	}
	return nil
}

func (p *program) LlmSetValue(desc, value string) error {
	_, err := p.runProgram(fmt.Sprintf("llmSetValue('%s', '%s')", desc, value))
	if err != nil {
		return err
	}
	return nil
}

func (p *program) LlmSetValueSkipVerify(desc, value string) error {
	_, err := p.runProgram(fmt.Sprintf("llmSetValueSkipVerify('%s', '%s')", desc, value))
	if err != nil {
		return err
	}
	return nil
}

func (p *program) LlmLogin(username, password string) error {
	_, err := p.runProgram(fmt.Sprintf("llmLogin('%s', '%s')", username, password))
	if err != nil {
		return err
	}
	return nil
}

func (p *program) GetURL() (string, error) {
	res, err := p.runProgram("getURL()")
	if err != nil {
		return "", err
	}
	return res.Value.(string), nil
}
