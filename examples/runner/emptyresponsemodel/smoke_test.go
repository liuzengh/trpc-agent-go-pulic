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
	"os"
	"reflect"
	"testing"
	"time"

	openaiopt "github.com/openai/openai-go/option"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

// TestLiveSmoke is opt-in. It makes two real provider requests per streaming
// mode and deterministically replaces the first completed attempt with an empty
// answer. This exercises retries without relying on a provider malfunction.
func TestLiveSmoke(t *testing.T) {
	if os.Getenv("TRPC_EMPTY_RESPONSE_SMOKE") != "1" {
		t.Skip("set TRPC_EMPTY_RESPONSE_SMOKE=1 to call a real provider")
	}
	name := os.Getenv("MODEL_NAME")
	if name == "" || os.Getenv("OPENAI_API_KEY") == "" {
		t.Fatal("MODEL_NAME and OPENAI_API_KEY are required")
	}
	for _, streaming := range []bool{false, true} {
		t.Run(fmt.Sprintf("stream=%t", streaming), func(t *testing.T) {
			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
			defer cancel()
			realModel := openai.New(name, openai.WithOpenAIOptions(openaiopt.WithMaxRetries(0)))
			calls := 0
			var requests [][]model.Message
			base := modelFunc(func(ctx context.Context, req *model.Request) (<-chan *model.Response, error) {
				calls++
				requests = append(requests, append([]model.Message(nil), req.Messages...))
				ch, err := realModel.GenerateContent(ctx, req)
				if err != nil {
					return nil, err
				}
				var received []*model.Response
				for {
					select {
					case <-ctx.Done():
						return nil, ctx.Err()
					case rsp, ok := <-ch:
						if !ok {
							if calls == 1 {
								return responses(completion("", "rejected-attempt")), nil
							}
							return responses(received...), nil
						}
						if rsp != nil && rsp.Error != nil {
							return responses(rsp), nil
						}
						received = append(received, rsp)
					}
				}
			})
			svc := newSessionService()
			r := newExampleRunner(base, svc, streaming, 1, 10*time.Millisecond)
			defer r.Close()
			ch, err := r.Run(ctx, "user", "smoke", model.NewUserMessage("Reply exactly OK. Do not explain."))
			if err != nil {
				t.Fatal(err)
			}
			answer := ""
			for e := range ch {
				if e == nil || e.Response == nil {
					continue
				}
				if e.Error != nil {
					t.Fatalf("model call failed (%s)", e.Error.Type)
				}
				if !e.IsPartial && hasAnswer(e.Response) {
					answer = e.Choices[0].Message.Content
				}
			}
			if err := ctx.Err(); err != nil {
				t.Fatal(err)
			}
			if calls != 2 || answer == "" {
				t.Fatalf("calls=%d, answer present=%t", calls, answer != "")
			}
			if !reflect.DeepEqual(requests[0], requests[1]) {
				t.Fatal("failed attempt polluted retry prompt")
			}
			sess, err := svc.GetSession(ctx, session.Key{AppName: appName, UserID: "user", SessionID: "smoke"})
			if err != nil {
				t.Fatal(err)
			}
			checkHistory(t, sess, 1)
			t.Logf("live provider attempts=%d; rejected attempt absent from retry prompt and session; answer=%q", calls, answer)
		})
	}
}
