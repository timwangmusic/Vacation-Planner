package planner

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	openai2 "github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
	"github.com/sashabaranov/go-openai"
	"os"
)

func chatCompletion(ctx context.Context, msg string) (string, error) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		return "", errors.New("no openai api key found")
	}

	client := openai.NewClient(apiKey)

	resp, err := client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
		Model: openai.GPT4o,
		Messages: []openai.ChatCompletionMessage{{
			Role:    openai.ChatMessageRoleUser,
			Content: msg}},
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

	client := openai2.NewClient(option.WithAPIKey(apiKey))

	resp, err := client.Images.Generate(ctx, openai2.ImageGenerateParams{
		Prompt:         openai2.String(description),
		Model:          openai2.F(openai2.ImageModelDallE3),
		ResponseFormat: openai2.F(openai2.ImageGenerateParamsResponseFormatB64JSON),
		N:              openai2.Int(1),
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
