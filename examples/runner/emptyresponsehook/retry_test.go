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
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

func TestHookPreservesPartialsAndRejectsOnlyCompleteResponses(t *testing.T) {
	ctx := agent.NewInvocationContext(context.Background(), agent.NewInvocation())
	cb := newRetryCallbacks(1, 0)
	partial := &model.Response{IsPartial: true, Choices: []model.Choice{{Delta: model.Message{ReasoningContent: "thinking"}}}}
	for i := 0; i < 3; i++ {
		result, err := cb.RunAfterModel(ctx, &model.AfterModelArgs{Response: partial})
		if err != nil || result != nil {
			t.Fatalf("partial was intercepted: %+v %v", result, err)
		}
	}
	src := completion("", "rejected-attempt")
	src.Usage = &model.Usage{TotalTokens: 10}
	result, err := cb.RunAfterModel(ctx, &model.AfterModelArgs{Response: src})
	if err != nil {
		t.Fatal(err)
	}
	if result == nil || result.CustomResponse == nil || result.CustomResponse.Done || result.CustomResponse.IsValidContent() {
		t.Fatalf("expected non-final response without content: %+v", result)
	}
	if !src.Done || src.Choices[0].Message.ReasoningContent != "rejected-attempt" {
		t.Fatal("callback mutated provider response")
	}
	if result.CustomResponse.Usage.TotalTokens != 10 {
		t.Fatal("callback dropped usage")
	}
}

func TestHookCancellationStopsRetry(t *testing.T) {
	parent, cancel := context.WithCancel(context.Background())
	cancel()
	ctx := agent.NewInvocationContext(parent, agent.NewInvocation())
	cb := newRetryCallbacks(2, time.Hour)
	_, err := cb.RunAfterModel(ctx, &model.AfterModelArgs{Response: completion("", "")})
	if err != context.Canceled {
		t.Fatalf("got %v", err)
	}
}
