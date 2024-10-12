package planner

import (
	"context"
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
