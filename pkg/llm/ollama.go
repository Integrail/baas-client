package llm

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/ollama/ollama/api"
	"github.com/pkg/errors"
	"github.com/samber/lo"

	"github.com/simple-container-com/go-aws-lambda-sdk/pkg/logger"
	"github.com/simple-container-com/go-aws-lambda-sdk/pkg/util/retry"
)

func NewOllama(log logger.Logger, ollamaUrl, ollamaApiKey string) Client {
	return &ollamaClient{
		log:          log,
		ollamaApiKey: ollamaApiKey,
		ollamaUrl:    ollamaUrl,
	}
}

type ollamaClient struct {
	log          logger.Logger
	ollamaApiKey string
	ollamaUrl    string
}

type RoundTripFn func(req *http.Request) (*http.Response, error)

func (f RoundTripFn) RoundTrip(req *http.Request) (*http.Response, error) { return f(req) }

func (o *ollamaClient) Generate(ctx context.Context, request GenerateRequest) (*GenerateResponse, error) {
	baseURL, err := url.Parse(o.ollamaUrl)
	if err != nil {
		return nil, errors.Wrapf(err, "invalid ollama url %q", o.ollamaUrl)
	}
	client := api.NewClient(baseURL, &http.Client{
		Timeout: time.Second * 120,
		Transport: RoundTripFn(func(req *http.Request) (*http.Response, error) {
			req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", o.ollamaApiKey))
			return http.DefaultTransport.RoundTrip(req)
		}),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to init ollama client")
	}
	resBuf := strings.Builder{}
	_, err = retry.With(retry.Config[any]{
		AttemptErrorCallback: func(i int, err error) {
			time.Sleep(lo.If(request.RetryCooldown == 0, 50*time.Millisecond).Else(request.RetryCooldown))
		},
		Action: func() (any, error) {
			err = client.Generate(ctx, &api.GenerateRequest{
				Model:  lo.If(request.Model != "", request.Model).Else("llama3.1:8b"),
				Prompt: request.Prompt,
				Stream: lo.ToPtr(false),
			}, func(response api.GenerateResponse) error {
				resBuf.WriteString(response.Response)
				return nil
			})
			return nil, err
		},
		MaxRetries: lo.If(request.MaxRetries == 0, 1).Else(request.MaxRetries),
	})
	if err != nil {
		return nil, errors.Wrapf(err, "failed to process prompt with model after 3 retries")
	}
	return &GenerateResponse{
		Response: resBuf.String(),
	}, nil
}
