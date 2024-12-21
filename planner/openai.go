package planner

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
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

	client := openai.NewClient(apiKey)

	resp, err := client.CreateImage(ctx, openai.ImageRequest{
		Prompt:         description,
		Size:           openai.CreateImageSize256x256,
		ResponseFormat: openai.CreateImageResponseFormatB64JSON,
		N:              1,
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
