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
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

// retryModel exposes only model.Model. Keeping the provider in a named field
// prevents a provider iterator method from bypassing validation.
// Each underlying stream must end with a non-partial, aggregated response, as
// model/openai does. Partials are discarded; callers receive only the final result.
// The wrapper is immutable and safe to share if its underlying model is safe.
type retryModel struct {
	base       model.Model
	maxRetries int
	delay      time.Duration
}

// Info returns the underlying model's information.
func (m *retryModel) Info() model.Info { return m.base.Info() }

// GenerateContent waits for validation before returning one buffered response.
// It retries only empty completions, including streams closed without a complete
// response. Provider errors pass through without retries. Cancellation interrupts
// both receiving and backoff. No response from a rejected attempt is published.
func (m *retryModel) GenerateContent(ctx context.Context, req *model.Request) (<-chan *model.Response, error) {
	if req == nil {
		return nil, fmt.Errorf("nil model request")
	}
	if m.base == nil || m.maxRetries < 0 || m.delay < 0 {
		return nil, fmt.Errorf("invalid empty response retry configuration")
	}
	// Snapshot serialized fields once. Each attempt receives a fresh copy so
	// provider-side request tailoring cannot change the next attempt's prompt.
	payload, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("snapshot model request: %w", err)
	}
	tools := make(map[string]tool.Tool, len(req.Tools))
	for name, impl := range req.Tools {
		tools[name] = impl
	}
	for attempt := 0; ; attempt++ {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		attemptReq, err := restoreRequest(payload, tools)
		if err != nil {
			return nil, err
		}
		rsp, err := m.generateAttempt(ctx, attemptReq)
		if err != nil {
			return nil, err
		}
		if rsp != nil && (rsp.Error != nil || hasAnswer(rsp)) {
			return responseChannel(rsp), nil
		}
		if attempt >= m.maxRetries {
			return responseChannel(exhaustedResponse(m.maxRetries)), nil
		}
		if err := waitRetry(ctx, m.delay); err != nil {
			return nil, err
		}
	}
}

func restoreRequest(payload []byte, tools map[string]tool.Tool) (*model.Request, error) {
	var req model.Request
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if err := decoder.Decode(&req); err != nil {
		return nil, fmt.Errorf("restore model request: %w", err)
	}
	req.Tools = make(map[string]tool.Tool, len(tools))
	for name, impl := range tools {
		req.Tools[name] = impl
	}
	// Tool implementations remain shared; model adapters must not mutate them.
	return &req, nil
}

func (m *retryModel) generateAttempt(ctx context.Context, req *model.Request) (*model.Response, error) {
	attemptCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	responses, err := m.base.GenerateContent(attemptCtx, req)
	if err != nil {
		return nil, err
	}
	if responses == nil {
		return nil, fmt.Errorf("model returned a nil response channel")
	}
	var complete *model.Response
	for {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case rsp, ok := <-responses:
			if !ok {
				return complete, ctx.Err()
			}
			if rsp == nil {
				continue
			}
			if rsp.Error != nil {
				return rsp, nil
			}
			if !rsp.IsPartial {
				complete = rsp
			}
		}
	}
}

func responseChannel(rsp *model.Response) <-chan *model.Response {
	responses := make(chan *model.Response, 1)
	responses <- rsp
	close(responses)
	return responses
}
