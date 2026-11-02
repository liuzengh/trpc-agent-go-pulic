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
	"encoding/json"
	"errors"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/tool"
)

func TestWrapperRestoresRequestForEveryAttempt(t *testing.T) {
	maxTokens := 123
	echo := &echoTool{}
	req := &model.Request{
		Messages:         []model.Message{model.NewUserMessage("original")},
		GenerationConfig: model.GenerationConfig{MaxTokens: &maxTokens},
		Tools:            map[string]tool.Tool{"echo": echo},
		StructuredOutput: &model.StructuredOutput{Type: model.StructuredOutputJSONSchema, JSONSchema: &model.JSONSchemaConfig{
			Name: "original", Schema: map[string]any{"maximum": json.Number("9007199254740993")},
		}},
	}
	calls := 0
	base := modelFunc(func(_ context.Context, r *model.Request) (<-chan *model.Response, error) {
		calls++
		if r.Messages[0].Content != "original" || *r.MaxTokens != 123 || r.Tools["echo"] != echo || r.StructuredOutput.JSONSchema.Name != "original" {
			t.Error("attempt inherited provider mutation")
		}
		if r.StructuredOutput.JSONSchema.Schema["maximum"] != json.Number("9007199254740993") {
			t.Error("lost numeric precision")
		}
		r.Messages[0].Content = "mutated"
		*r.MaxTokens = 1
		delete(r.Tools, "echo")
		r.StructuredOutput.JSONSchema.Name = "mutated"
		if calls == 1 {
			return responses(completion("", "")), nil
		}
		return responses(completion("ok", "")), nil
	})
	wrapped := &retryModel{base: base, maxRetries: 1}
	ch, err := wrapped.GenerateContent(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if rsp := <-ch; !hasAnswer(rsp) {
		t.Fatal("missing successful result")
	}
	if calls != 2 || req.Messages[0].Content != "original" || maxTokens != 123 || req.Tools["echo"] != echo || req.StructuredOutput.JSONSchema.Name != "original" {
		t.Fatal("original request was changed")
	}
}

func TestWrapperDropsPartialsAndRetriesIncompleteStreams(t *testing.T) {
	calls := 0
	base := modelFunc(func(context.Context, *model.Request) (<-chan *model.Response, error) {
		calls++
		partial := &model.Response{IsPartial: true, Choices: []model.Choice{{Delta: model.Message{Content: "not committed"}}}}
		switch calls {
		case 1:
			return responses(), nil
		case 2:
			return responses(nil, partial), nil
		default:
			return responses(partial, completion("ok", "")), nil
		}
	})
	wrapped := &retryModel{base: base, maxRetries: 2}
	ch, err := wrapped.GenerateContent(context.Background(), &model.Request{})
	if err != nil {
		t.Fatal(err)
	}
	count := 0
	for rsp := range ch {
		count++
		if rsp.IsPartial || rsp.Choices[0].Message.Content != "ok" {
			t.Fatalf("uncommitted response escaped: %+v", rsp)
		}
	}
	if calls != 3 || count != 1 {
		t.Fatalf("attempts=%d responses=%d", calls, count)
	}
}

func TestWrapperCancellationCancelsUnderlyingStream(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	started, stopped := make(chan struct{}), make(chan struct{})
	base := modelFunc(func(ctx context.Context, _ *model.Request) (<-chan *model.Response, error) {
		ch := make(chan *model.Response)
		go func() {
			close(started)
			<-ctx.Done()
			close(ch)
			close(stopped)
		}()
		return ch, nil
	})
	wrapped := &retryModel{base: base, maxRetries: 2}
	done := make(chan error, 1)
	go func() { _, err := wrapped.GenerateContent(ctx, &model.Request{}); done <- err }()
	<-started
	cancel()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("got %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("wrapper ignored cancellation")
	}
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("underlying stream not canceled")
	}
}

func TestWrapperDoesNotRetryTransportErrors(t *testing.T) {
	failure := errors.New("transport failed")
	calls := 0
	wrapped := &retryModel{maxRetries: 2, base: modelFunc(func(context.Context, *model.Request) (<-chan *model.Response, error) { calls++; return nil, failure })}
	_, err := wrapped.GenerateContent(context.Background(), &model.Request{})
	if !errors.Is(err, failure) || calls != 1 {
		t.Fatalf("calls=%d err=%v", calls, err)
	}
}
