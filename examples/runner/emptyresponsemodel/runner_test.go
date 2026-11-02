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
	"reflect"
	"sync"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

type funcModel struct {
	generate func(context.Context, *model.Request) (<-chan *model.Response, error)
}

func modelFunc(fn func(context.Context, *model.Request) (<-chan *model.Response, error)) *funcModel {
	return &funcModel{generate: fn}
}
func (m *funcModel) Info() model.Info { return model.Info{Name: "test-model"} }
func (m *funcModel) GenerateContent(ctx context.Context, req *model.Request) (<-chan *model.Response, error) {
	return m.generate(ctx, req)
}

func responses(items ...*model.Response) <-chan *model.Response {
	ch := make(chan *model.Response, len(items))
	for _, item := range items {
		ch <- item
	}
	close(ch)
	return ch
}

func completion(content, reasoning string) *model.Response {
	return &model.Response{Done: true, Choices: []model.Choice{{Message: model.Message{
		Role: model.RoleAssistant, Content: content, ReasoningContent: reasoning,
	}}}}
}

func collectRun(t *testing.T, r runner.Runner, svc session.Service, id string) ([]*model.Response, *session.Session) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	ch, err := r.Run(ctx, "user", id, model.NewUserMessage("hello"))
	if err != nil {
		t.Fatal(err)
	}
	var got []*model.Response
	for e := range ch {
		if e != nil && e.Response != nil && e.Object != model.ObjectTypeRunnerCompletion {
			got = append(got, e.Response)
		}
	}
	if err := ctx.Err(); err != nil {
		t.Fatal(err)
	}
	sess, err := svc.GetSession(ctx, session.Key{AppName: appName, UserID: "user", SessionID: id})
	if err != nil {
		t.Fatal(err)
	}
	return got, sess
}

func checkHistory(t *testing.T, sess *session.Session, wantAssistant int) {
	t.Helper()
	users, assistants := 0, 0
	for _, e := range sess.Events {
		if e.Response == nil {
			continue
		}
		if e.Error != nil {
			t.Fatalf("unexpected persisted error: %+v", e.Error)
		}
		for _, c := range e.Choices {
			switch c.Message.Role {
			case model.RoleUser:
				users++
			case model.RoleAssistant:
				assistants++
				if c.Message.Content == "" && len(c.Message.ContentParts) == 0 && len(c.Message.ToolCalls) == 0 {
					t.Fatalf("rejected assistant response entered history: %+v", c.Message)
				}
				if c.Message.ReasoningContent == "rejected-attempt" {
					t.Fatal("rejected reasoning entered history")
				}
			}
		}
	}
	if users != 1 || assistants != wantAssistant {
		t.Fatalf("history: users=%d assistants=%d, want 1/%d", users, assistants, wantAssistant)
	}
}

func TestRunnerRetriesWithoutHistoryPollution(t *testing.T) {
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", streaming), func(t *testing.T) {
			partText := "multimodal answer"
			parts := completion("", "")
			parts.Choices[0].Message.ContentParts = []model.ContentPart{{Type: model.ContentTypeText, Text: &partText}}
			cases := []struct {
				name      string
				first     *model.Response
				failures  int
				retries   int
				wantCalls int
				exhausted bool
			}{
				{"valid", completion("ok", ""), 0, 2, 1, false},
				{"content_parts", parts, 1, 2, 1, false},
				{"empty", completion("", ""), 1, 2, 2, false},
				{"reasoning_only", completion("", "rejected-attempt"), 1, 2, 2, false},
				{"no_choices", &model.Response{Done: true}, 1, 2, 2, false},
				{"non_final_empty", &model.Response{Done: false}, 1, 2, 2, false},
				{"exhausted", completion("", "rejected-attempt"), 3, 2, 3, true},
				{"zero_retries", completion("", ""), 1, 0, 1, true},
			}
			for _, tc := range cases {
				t.Run(tc.name, func(t *testing.T) {
					calls := 0
					var requests [][]model.Message
					base := modelFunc(func(_ context.Context, req *model.Request) (<-chan *model.Response, error) {
						calls++
						requests = append(requests, append([]model.Message(nil), req.Messages...))
						if calls > tc.wantCalls {
							return nil, fmt.Errorf("unexpected extra model call")
						}
						rsp := completion("ok", "")
						if calls <= tc.failures {
							rsp = tc.first
						}
						if streaming {
							return responses(&model.Response{IsPartial: true},
								&model.Response{IsPartial: true, Choices: []model.Choice{{Delta: model.Message{ReasoningContent: "partial-reasoning"}}}}, rsp), nil
						}
						return responses(rsp), nil
					})
					svc := newSessionService()
					r := newExampleRunner(base, svc, streaming, tc.retries, 0)
					defer r.Close()
					got, sess := collectRun(t, r, svc, "test")
					if calls != tc.wantCalls {
						t.Fatalf("calls=%d, want %d", calls, tc.wantCalls)
					}
					partials := 0
					errorSeen := false
					for _, rsp := range got {
						if rsp.IsPartial {
							partials++
						}
						if rsp.Error != nil {
							if rsp.Error.Type != emptyResponseErrorType {
								t.Fatalf("unexpected error: %+v", rsp.Error)
							}
							errorSeen = true
						}
					}
					if (partials > 0) != (false) {
						t.Fatalf("unexpected partial visibility: partials=%d", partials)
					}
					if errorSeen != tc.exhausted {
						t.Fatalf("terminal error=%t, want %t", errorSeen, tc.exhausted)
					}
					wantAssistant := 1
					if tc.exhausted {
						wantAssistant = 0
					}
					checkHistory(t, sess, wantAssistant)
					for _, req := range requests[1:] {
						if !reflect.DeepEqual(req, requests[0]) {
							t.Fatalf("retry prompt changed:\n%+v\n%+v", requests[0], req)
						}
					}
				})
			}
		})
	}
}

func TestProviderErrorsAreNotRetriedOrFiltered(t *testing.T) {
	calls := 0
	base := modelFunc(func(context.Context, *model.Request) (<-chan *model.Response, error) {
		calls++
		return responses(&model.Response{Done: true, Error: &model.ResponseError{Type: model.ErrorTypeAPIError, Message: "provider unavailable"}}), nil
	})
	svc := newSessionService()
	r := newExampleRunner(base, svc, false, 2, 0)
	defer r.Close()
	got, sess := collectRun(t, r, svc, "provider-error")
	if calls != 1 {
		t.Fatalf("retried provider error %d times", calls)
	}
	visible, stored := false, false
	for _, rsp := range got {
		if rsp.Error != nil && rsp.Error.Type == model.ErrorTypeAPIError {
			visible = true
		}
	}
	for _, e := range sess.Events {
		if e.Response != nil && e.Error != nil && e.Error.Type == model.ErrorTypeAPIError {
			stored = true
		}
	}
	if !visible || !stored {
		t.Fatalf("provider error: visible=%t stored=%t", visible, stored)
	}
}

func TestConcurrentRunsHaveIndependentBudgets(t *testing.T) {
	// Every attempt is invalid: each invocation must get its own full budget.
	var mu sync.Mutex
	calls := 0
	base := modelFunc(func(context.Context, *model.Request) (<-chan *model.Response, error) {
		mu.Lock()
		calls++
		mu.Unlock()
		return responses(completion("", "")), nil
	})
	svc := newSessionService()
	r := newExampleRunner(base, svc, false, 2, 0)
	defer r.Close()
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, sess := collectRun(t, r, svc, fmt.Sprintf("concurrent-%d", i))
			checkHistory(t, sess, 0)
		}(i)
	}
	wg.Wait()
	if calls != 12 {
		t.Fatalf("calls=%d, want 12", calls)
	}
}

func TestWaitRetryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := waitRetry(ctx, time.Hour); err != context.Canceled {
		t.Fatalf("got %v", err)
	}
}
