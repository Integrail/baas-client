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

type ActionOption func(args []string) []string

func WithoutTimeout() ActionOption {
	return func(args []string) []string {
		return append(args, "withoutTimeout")
	}
}

func WithAllowTags(tags ...string) ActionOption {
	return func(args []string) []string {
		return append(args, fmt.Sprintf("allowTags:%s", strings.Join(tags, ",")))
	}
}

func WithAllowAttrs(attrs ...string) ActionOption {
	return func(args []string) []string {
		return append(args, fmt.Sprintf("allowAttributes:%s", strings.Join(attrs, ",")))
	}
}

func WithTimeout(timeout string) ActionOption {
	return func(args []string) []string {
		return append(args, fmt.Sprintf("timeout:%s", timeout))
	}
}

func WithSelector(selector string) ActionOption {
	return func(args []string) []string {
		return append(args, fmt.Sprintf("selector:%s", selector))
	}
}

func WithSecretArgs() ActionOption {
	return func(args []string) []string {
		return append(args, "secretArgs")
	}
}

func WithIncludeInvisible() ActionOption {
	return func(args []string) []string {
		return append(args, "includeInvisible")
	}
}

func WithIframe(selector string) ActionOption {
	return func(args []string) []string {
		return append(args, fmt.Sprintf("iframe:%s", selector))
	}
}

type Program interface {
	Error() error
	NavigateStatus(url string, opts ...ActionOption) (int, error)
	TakeScreenshot(name string, opts ...ActionOption) ([]byte, error)
	LlmSetValue(desc, value string, opts ...ActionOption) error
	LlmSetValueSkipVerify(desc, value string, opts ...ActionOption) error
	LlmLogin(username, password string, opts ...ActionOption) error
	GetURL(opts ...ActionOption) (string, error)
	Click(selector string, opts ...ActionOption) error
	ClickN(selector string, index int, opts ...ActionOption) error
	SetValueN(selector string, index int, value string, opts ...ActionOption) error
	GetInnerText(selector string, opts ...ActionOption) (string, error)
	GetSecret(name string, opts ...ActionOption) (string, error)
	GetValue(name string, opts ...ActionOption) (string, error)
	OuterHtml(selector string, opts ...ActionOption) (string, error)
	InnerHtml(selector string, opts ...ActionOption) (string, error)
	IsElementPresent(selector string, opts ...ActionOption) (bool, error)
	LlmClick(description string, opts ...ActionOption) error
	LlmClickElement(elems []string, description string, opts ...ActionOption) error
	LlmSendKeys(description, value string, opts ...ActionOption) error
	LlmText(description string, opts ...ActionOption) (string, error)
	Log(message string, opts ...ActionOption) error
	LogURL(opts ...ActionOption) error
	Navigate(url string, opts ...ActionOption) error
	ReplaceInnerHtml(selector, html string, opts ...ActionOption) error
	SendKeys(text string, opts ...ActionOption) error
	Sleep(duration string, opts ...ActionOption) error
	Submit(selector string, opts ...ActionOption) error
	Text(selector string, opts ...ActionOption) (string, error)
	WaitFileDownload(duration string, opts ...ActionOption) (bool, error)
	ExecuteAndDownloadFile(program string, fileName string, waitStarted, waitDownloaded string, opts ...ActionOption) ([]byte, error)
	DownloadFile(fileName string, waitStarted, waitDownloaded string, opts ...ActionOption) ([]byte, error)
	WaitReady(selector string, opts ...ActionOption) error
	WaitVisible(selector string, opts ...ActionOption) error
	SaveScreenshot(name string, fileName string, opts ...ActionOption) error
	FindVisibleElements(elements []string, attributeName string, opts ...ActionOption) (string, error)
	Execute(program string, opts ...ActionOption) (any, error)
	DragAndDropBySelectors(from, to string, opts ...ActionOption) error
}

type Reporter interface {
	Report(msg string)
}

type Config struct {
	UseProxy       bool                `json:"useProxy" yaml:"useProxy"`
	LocalDebug     bool                `json:"localDebug" yaml:"localDebug"`
	Url            string              `json:"url" yaml:"url"`
	ApiKey         string              `json:"apiKey" yaml:"apiKey"`
	Timeout        string              `json:"timeout" yaml:"timeout"`
	MessageTimeout string              `json:"messageTimeout" yaml:"messageTimeout"`
	Secrets        []string            `json:"secrets" yaml:"secrets"`
	Values         []string            `json:"values" yaml:"values"`
	Cookies        []dto.BrowserCookie `json:"cookies" yaml:"cookies"`
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
		cfg:      cfg,
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
				Cookies:          cfg.Cookies,
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
	cfg       Config
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
		Timeout:   p.cfg.MessageTimeout,
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

func (p *program) SetValueN(selector string, index int, value string, opts ...ActionOption) error {
	_, err := p.runProgram(fmt.Sprintf("setValueN('%s', %d, '%s', %s)", selector, index, value, p.addArgs(opts)))
	if err != nil {
		return err
	}
	return nil
}

func (p *program) ClickN(selector string, index int, opts ...ActionOption) error {
	_, err := p.runProgram(fmt.Sprintf("clickN('%s', %d, %s)", selector, index, p.addArgs(opts)))
	if err != nil {
		return err
	}
	return nil
}

func (p *program) Click(selector string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall1("click", selector, opts...))
	if err != nil {
		return err
	}
	return nil
}

func (p *program) GetInnerText(selector string, opts ...ActionOption) (string, error) {
	res, err := p.runProgram(p.functionCall1("getInnerText", selector, opts...))
	if err != nil {
		return "", err
	}
	return res.Value.(string), nil
}

func (p *program) GetSecret(name string, opts ...ActionOption) (string, error) {
	res, err := p.runProgram(p.functionCall1("getSecret", name, opts...))
	if err != nil {
		return "", err
	}
	return res.Value.(string), nil
}

func (p *program) GetValue(name string, opts ...ActionOption) (string, error) {
	res, err := p.runProgram(p.functionCall1("getValue", name, opts...))
	if err != nil {
		return "", err
	}
	return res.Value.(string), nil
}

func (p *program) IsElementPresent(selector string, opts ...ActionOption) (bool, error) {
	res, err := p.runProgram(p.functionCall1("isElementPresent", selector, opts...))
	if err != nil {
		return false, err
	}
	return res.Value.(bool), nil
}

func (p *program) LlmClick(description string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall1("llmClick", description, opts...))
	return err
}

func (p *program) LlmSendKeys(description, value string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall2("llmSendKeys", description, value, opts...))
	return err
}

func (p *program) LlmClickElement(elements []string, description string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall2("llmClickElement", strings.Join(elements, ","), description, opts...))
	if err != nil {
		return err
	}
	return nil
}

func (p *program) FindVisibleElements(elements []string, addAttributeName string, opts ...ActionOption) (string, error) {
	res, err := p.runProgram(p.functionCall2("findVisibleElements", strings.Join(elements, ","), addAttributeName, opts...))
	if err != nil {
		return "", err
	}
	return res.Value.(string), nil
}

func (p *program) LlmText(description string, opts ...ActionOption) (string, error) {
	res, err := p.runProgram(p.functionCall1("llmText", description, opts...))
	if err != nil {
		return "", err
	}
	return res.Value.(string), nil
}

func (p *program) Log(message string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall1("log", message, opts...))
	return err
}

func (p *program) LogURL(opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall0("logURL", opts...))
	return err
}

func (p *program) Navigate(url string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall1("navigate", url, opts...))
	return err
}

func (p *program) OuterHtml(selector string, opts ...ActionOption) (string, error) {
	res, err := p.runProgram(p.functionCall1("outerHtml", selector, opts...))
	if err != nil {
		return "", err
	}
	return res.OutHTML, nil
}

func (p *program) InnerHtml(selector string, opts ...ActionOption) (string, error) {
	res, err := p.runProgram(p.functionCall1("innerHtml", selector, opts...))
	if err != nil {
		return "", err
	}
	return res.OutHTML, nil
}

func (p *program) ReplaceInnerHtml(selector, html string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall2("replaceInnerHtml", selector, html, opts...))
	return err
}

func (p *program) SendKeys(text string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall1("sendKeys", text, opts...))
	return err
}

func (p *program) Sleep(duration string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall1("sleep", duration, opts...))
	return err
}

func (p *program) Submit(selector string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall1("submit", selector, opts...))
	return err
}

func (p *program) Text(selector string, opts ...ActionOption) (string, error) {
	res, err := p.runProgram(p.functionCall1("text", selector, opts...))
	if err != nil {
		return "", err
	}
	return res.Value.(string), nil
}

func (p *program) WaitFileDownloadStarted(duration string, opts ...ActionOption) (bool, error) {
	res, err := p.runProgram(p.functionCall1("waitFileDownloadStarted", duration, opts...))
	if err != nil {
		return false, err
	}
	return res.Value.(bool), nil
}

func (p *program) WaitFileDownload(duration string, opts ...ActionOption) (bool, error) {
	res, err := p.runProgram(p.functionCall1("waitFileDownload", duration, opts...))
	if err != nil {
		return false, err
	}
	return res.Value.(bool), nil
}

func (p *program) ExecuteAndDownloadFile(program string, fileName string, waitStarted, waitDownloaded string, opts ...ActionOption) ([]byte, error) {
	res, err := p.runProgram(fmt.Sprintf(`
			%s
			if (!%s) {
				throw 'File download did not start within %s';
			}
			%s`,
		program,
		p.functionCall1("waitFileDownloadStarted", waitStarted, opts...),
		waitStarted,
		p.functionCall1("waitFileDownload", waitDownloaded, opts...)))
	if err != nil {
		return nil, err
	}
	if len(res.DownloadedFile) == 0 {
		return nil, errors.Errorf("downloaded file size is zero")
	}
	var message string
	if err := os.WriteFile(fileName, res.DownloadedFile, 0o644); err != nil {
		message = fmt.Sprintf("failed to save file %s: %q", fileName, err.Error())
		p.reporter.Report(message)
		return nil, err
	} else {
		message = fmt.Sprintf("%q saved to ", fileName) +
			termlink.ColorLink(fileName, fmt.Sprintf("file://%s", fileName), "italic green")
		p.reporter.Report(message)
	}
	return res.DownloadedFile, nil
}

func (p *program) DownloadFile(fileName string, waitStarted, waitDownloaded string, opts ...ActionOption) ([]byte, error) {
	return p.ExecuteAndDownloadFile("", fileName, waitStarted, waitDownloaded, opts...)
}

func (p *program) DragAndDropBySelectors(from, to string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall2("dragAndDropBySelectors", from, to, opts...))
	return err
}

func (p *program) WaitReady(selector string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall1("waitReady", selector, opts...))
	return err
}

func (p *program) WaitVisible(selector string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall1("waitVisible", selector, opts...))
	return err
}

func (p *program) NavigateStatus(url string, opts ...ActionOption) (int, error) {
	res, err := p.runProgram(p.functionCall1("navigateStatus", url, opts...))
	if err != nil {
		return 0, err
	}
	status, ok := res.Value.(float64)
	if !ok {
		return 0, errors.Errorf("Failed to convert status code to int %v", res.Value)
	}
	return int(status), nil
}

func (p *program) TakeScreenshot(name string, opts ...ActionOption) ([]byte, error) {
	res, err := p.runProgram(p.functionCall1("takeScreenshot", name, opts...))
	if err != nil {
		return nil, err
	}
	if len(res.Screenshots[name]) == 0 {
		return nil, errors.Errorf("screenshot with name %s wasn't returned", name)
	}
	return res.Screenshots[name], nil
}

func (p *program) SaveScreenshot(name string, fileName string, opts ...ActionOption) error {
	screenshot, err := p.TakeScreenshot(name, opts...)
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

func (p *program) LlmSetValue(desc, value string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall2("llmSetValue", desc, value, opts...))
	if err != nil {
		return err
	}
	return nil
}

func (p *program) LlmSetValueSkipVerify(desc, value string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall2("llmSetValueSkipVerify", desc, value, opts...))
	if err != nil {
		return err
	}
	return nil
}

func (p *program) LlmLogin(username, password string, opts ...ActionOption) error {
	_, err := p.runProgram(p.functionCall2("llmLogin", username, password, opts...))
	if err != nil {
		return err
	}
	return nil
}

func (p *program) Execute(program string, opts ...ActionOption) (any, error) {
	res, err := p.runProgram(program)
	if err != nil {
		return "", err
	}
	return res.Value, nil
}

func (p *program) GetURL(opts ...ActionOption) (string, error) {
	res, err := p.runProgram(p.functionCall0("getURL", opts...))
	if err != nil {
		return "", err
	}
	return res.Value.(string), nil
}

func (p *program) functionCall0(name string, opts ...ActionOption) string {
	return fmt.Sprintf("%s(%s)", name, p.addArgs(opts))
}

func (p *program) functionCall1(name, arg1 string, opts ...ActionOption) string {
	return fmt.Sprintf("%s('%s'%s)", name, arg1, p.addArgs(opts))
}

func (p *program) functionCall2(name, arg1, arg2 string, opts ...ActionOption) string {
	return fmt.Sprintf("%s('%s', '%s'%s)", name, arg1, arg2, p.addArgs(opts))
}

func (p *program) addArgs(opts []ActionOption) string {
	var addArgs []string
	for _, opt := range opts {
		addArgs = opt(addArgs)
	}
	addArgsString := ""
	if len(addArgs) > 0 {
		addArgsString = ", " + fmt.Sprintf("'%s'", strings.Join(addArgs, "','"))
	}
	return addArgsString
}
