package planner

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"os"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

func chatCompletion(ctx context.Context, msg string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", errors.New("no openai api key found")
	}

	c := openai.NewClient(option.WithAPIKey(apiKey))
	resp, err := c.Chat.Completions.New(ctx, openai.ChatCompletionNewParams{
		Messages: []openai.ChatCompletionMessageParamUnion{openai.UserMessage(msg)},
		Model:    openai.ChatModelGPT4o,
	})

	if err != nil {
		return "", fmt.Errorf("error creating chat completion: %w", err)
	}

	return resp.Choices[0].Message.Content, nil
}

func imageGeneration(ctx context.Context, description string) ([]byte, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return nil, errors.New("no openai api key found")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	resp, err := client.Images.Generate(ctx, openai.ImageGenerateParams{
		Prompt:         description,
		Model:          openai.ImageModelDallE3,
		ResponseFormat: openai.ImageGenerateParamsResponseFormatB64JSON,
		N:              openai.Int(1),
	})
	if err != nil {
		return nil, fmt.Errorf("error creating image: %w", err)
	}

	imgBytes, err := base64.StdEncoding.DecodeString(resp.Data[0].B64JSON)
	if err != nil {
		return nil, fmt.Errorf("error decoding base64 image: %w", err)
	}

	return imgBytes, nil
}
