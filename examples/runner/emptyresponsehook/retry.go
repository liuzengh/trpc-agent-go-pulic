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

	"trpc.group/trpc-go/trpc-agent-go/agent"
	"trpc.group/trpc-go/trpc-agent-go/model"
)

// newRetryCallbacks bounds consecutive rejected completions in one invocation.
// Counters belong to the invocation, so one callbacks instance can serve many runs.
func newRetryCallbacks(maxRetries int, delay time.Duration) *model.Callbacks {
	const retryKey = "example:empty_response_retries"
	callbacks := model.NewCallbacks()
	callbacks.RegisterAfterModel(func(ctx context.Context, args *model.AfterModelArgs) (*model.AfterModelResult, error) {
		if args == nil || args.Response == nil || args.Error != nil {
			return nil, nil
		}
		rsp := args.Response
		if rsp.IsPartial || rsp.Error != nil {
			return nil, nil
		}
		inv, ok := agent.InvocationFromContext(ctx)
		if !ok || inv == nil {
			return nil, fmt.Errorf("empty response retry requires an invocation")
		}
		if hasAnswer(rsp) {
			inv.DeleteState(retryKey)
			return nil, nil
		}
		retries, _ := agent.GetStateValue[int](inv, retryKey)
		if retries >= maxRetries {
			return &model.AfterModelResult{CustomResponse: exhaustedResponse(maxRetries)}, nil
		}
		if err := waitRetry(ctx, delay); err != nil {
			return nil, err
		}
		inv.SetState(retryKey, retries+1)
		// Keep usage for observability, but remove all message payloads, including
		// reasoning. Done=false asks LLMAgent to start another model-call cycle.
		replacement := rsp.Clone()
		replacement.Done = false
		replacement.Choices = nil
		return &model.AfterModelResult{CustomResponse: replacement}, nil
	})
	return callbacks
}
