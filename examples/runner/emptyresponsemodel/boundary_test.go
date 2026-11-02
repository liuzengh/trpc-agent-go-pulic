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
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

func TestRetryBoundary(t *testing.T) {
	calls, before := 0, 0
	base := modelFunc(func(context.Context, *model.Request) (<-chan *model.Response, error) {
		calls++
		if calls == 1 {
			return responses(completion("", "")), nil
		}
		return responses(completion("ok", "")), nil
	})
	llm := &retryModel{base: base, maxRetries: 1}
	callbacks := model.NewCallbacks()
	callbacks.RegisterBeforeModel(func(_ context.Context, _ *model.BeforeModelArgs) (*model.BeforeModelResult, error) {
		before++
		return nil, nil
	})
	ag := llmagent.New("assistant", llmagent.WithModel(llm), llmagent.WithModelCallbacks(callbacks))
	svc := newSessionService()
	r := runner.NewRunner(appName, ag, runner.WithSessionService(svc))
	defer r.Close()
	_, sess := collectRun(t, r, svc, "boundary")
	checkHistory(t, sess, 1)
	if calls != 2 || before != 1 {
		t.Fatalf("provider calls=%d, BeforeModel calls=%d", calls, before)
	}
}

type echoTool struct{ calls int }

func (*echoTool) Declaration() *tool.Declaration {
	return &tool.Declaration{Name: "echo", Description: "Return ok", InputSchema: &tool.Schema{Type: "object"}}
}
func (e *echoTool) Call(context.Context, []byte) (any, error) { e.calls++; return "ok", nil }

func TestRetryAroundToolCallsDoesNotRepeatTheTool(t *testing.T) {
	calls := 0
	echo := &echoTool{}
	base := modelFunc(func(_ context.Context, req *model.Request) (<-chan *model.Response, error) {
		calls++
		switch calls {
		case 1, 3:
			return responses(completion("", "rejected-attempt")), nil
		case 2:
			return responses(&model.Response{Choices: []model.Choice{{Message: model.Message{
				Role: model.RoleAssistant, ToolCalls: []model.ToolCall{{ID: "call-1", Type: "function", Function: model.FunctionDefinitionParam{Name: "echo", Arguments: []byte(`{}`)}}},
			}}}}), nil
		case 4:
			results := 0
			for _, msg := range req.Messages {
				if msg.Role == model.RoleTool && msg.ToolID == "call-1" {
					results++
				}
			}
			if results != 1 {
				return nil, fmt.Errorf("tool results in retry request: %d", results)
			}
			return responses(completion("ok", "")), nil
		default:
			return nil, fmt.Errorf("unexpected attempt")
		}
	})
	llm := &retryModel{base: base, maxRetries: 1}
	callbacks := model.NewCallbacks()
	ag := llmagent.New("assistant", llmagent.WithModel(llm), llmagent.WithModelCallbacks(callbacks), llmagent.WithTools([]tool.Tool{echo}))
	svc := newSessionService()
	r := runner.NewRunner(appName, ag, runner.WithSessionService(svc))
	defer r.Close()
	got, sess := collectRun(t, r, svc, "tools")
	for _, rsp := range got {
		if rsp.Error != nil {
			t.Fatalf("unexpected error: %+v", rsp.Error)
		}
	}
	checkHistory(t, sess, 2)
	if calls != 4 || echo.calls != 1 {
		t.Fatalf("model calls=%d, tool calls=%d", calls, echo.calls)
	}
}
