package llm

import (
	"context"

	"github.com/pkg/errors"
	"github.com/tmc/langchaingo/llms"
	"github.com/tmc/langchaingo/llms/openai"

	"github.com/simple-container-com/go-aws-lambda-sdk/pkg/logger"
)

func NewOpenAI(log logger.Logger, openaiToken, openaiOrg, model string) (Client, error) {
	client, err := openai.New(
		openai.WithToken(openaiToken),
		openai.WithOrganization(openaiOrg),
		openai.WithModel(model),
	)
	if err != nil {
		return nil, err
	}

	return &openaiClient{
		log:    log,
		client: client,
	}, nil
}

type openaiClient struct {
	log    logger.Logger
	client *openai.LLM
}

func (o *openaiClient) Generate(ctx context.Context, request GenerateRequest) (*GenerateResponse, error) {
	var contents []llms.MessageContent
	contents = append(contents, llms.MessageContent{
		Role: llms.ChatMessageTypeHuman,
		Parts: []llms.ContentPart{
			llms.TextContent{
				Text: request.Prompt,
			},
		},
	})
	res, err := o.client.GenerateContent(ctx, contents)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to generate content for prompt")
	}
	if len(res.Choices) == 0 {
		return nil, errors.Errorf("response does not contain any result")
	}

	return &GenerateResponse{
		Response: res.Choices[0].Content,
	}, nil
}
