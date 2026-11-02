//
// Tencent is pleased to support the open source community by making trpc-agent-go available.
//
// Copyright (C) 2026 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//
//

package main

import (
	"context"
	"fmt"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

const emptyResponseErrorType = "example_empty_model_response"

// hasAnswer checks the first choice consumed by LLMAgent. Reasoning alone is
// not an answer; multimodal content and tool calls are valid without text.
func hasAnswer(rsp *model.Response) bool {
	if rsp == nil || len(rsp.Choices) == 0 {
		return false
	}
	msg := rsp.Choices[0].Message
	return msg.Content != "" || len(msg.ContentParts) > 0 || len(msg.ToolCalls) > 0
}

func exhaustedResponse(maxRetries int) *model.Response {
	return &model.Response{
		Done: true,
		Error: &model.ResponseError{
			Type:    emptyResponseErrorType,
			Message: fmt.Sprintf("model returned no answer after %d attempts", maxRetries+1),
		},
	}
}

// skipEmptyResponseError keeps the terminal retry error visible to the caller
// without storing it in conversation history. Other errors retain their defaults.
func skipEmptyResponseError(h *session.AppendEventContext, next func() error) error {
	e := h.Event
	if e != nil && e.Response != nil && e.Error != nil && e.Error.Type == emptyResponseErrorType {
		return nil
	}
	return next()
}

func waitRetry(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return ctx.Err()
	}
}
