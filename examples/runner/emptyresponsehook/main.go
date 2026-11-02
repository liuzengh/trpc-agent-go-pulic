//
// Tencent is pleased to support the open source community by making trpc-agent-go available.
//
// Copyright (C) 2026 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//
//

// Package main demonstrates empty-response retries with Runner and LLMAgent.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	openaiopt "github.com/openai/openai-go/option"
	"trpc.group/trpc-go/trpc-agent-go/agent/llmagent"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/model/openai"
	"trpc.group/trpc-go/trpc-agent-go/runner"
	"trpc.group/trpc-go/trpc-agent-go/session"
	"trpc.group/trpc-go/trpc-agent-go/session/inmemory"
)

const appName = "empty-response-hook"

func main() {
	modelName := flag.String("model", os.Getenv("MODEL_NAME"), "Model name (defaults to MODEL_NAME)")
	prompt := flag.String("prompt", "Reply with OK.", "User message")
	streaming := flag.Bool("streaming", true, "Request streaming from the provider")
	retries := flag.Int("max-retries", 2, "Additional attempts after an empty completion")
	delay := flag.Duration("retry-delay", 250*time.Millisecond, "Delay between empty-response retries")
	timeout := flag.Duration("timeout", 2*time.Minute, "Deadline for the entire run, including retries")
	flag.Parse()
	if *modelName == "" || *retries < 0 || *delay < 0 || *timeout <= 0 {
		log.Fatal("set -model or MODEL_NAME, non-negative retries/delay, and a positive timeout")
	}
	ctx, cancel := context.WithTimeout(context.Background(), *timeout)
	defer cancel()
	if err := run(ctx, *modelName, *prompt, *streaming, *retries, *delay); err != nil {
		log.Fatal(err)
	}
}

func newExampleRunner(base model.Model, svc session.Service, streaming bool, maxRetries int, delay time.Duration) runner.Runner {
	llm := base
	ag := llmagent.New("assistant",
		llmagent.WithModel(llm),
		llmagent.WithInstruction("Answer briefly. Use a tool when needed."),
		llmagent.WithGenerationConfig(model.GenerationConfig{Stream: streaming}),
		llmagent.WithModelCallbacks(newRetryCallbacks(maxRetries, delay)),
	)
	return runner.NewRunner(appName, ag, runner.WithSessionService(svc))
}

func newSessionService() *inmemory.SessionService {
	return inmemory.NewSessionService(inmemory.WithAppendEventHook(skipEmptyResponseError))
}

func run(ctx context.Context, modelName, prompt string, streaming bool, maxRetries int, delay time.Duration) error {
	// Disable SDK transport retries so application-level attempt counts are clear.
	base := openai.New(modelName, openai.WithOpenAIOptions(openaiopt.WithMaxRetries(0)))
	svc := newSessionService()
	r := newExampleRunner(base, svc, streaming, maxRetries, delay)
	defer r.Close()
	events, err := r.Run(ctx, "demo-user", "demo-session", model.NewUserMessage(prompt))
	if err != nil {
		return err
	}
	var runErr error
	for e := range events {
		if e == nil || e.Response == nil {
			continue
		}
		if e.Error != nil {
			runErr = fmt.Errorf("%s: %s", e.Error.Type, e.Error.Message)
			continue
		}
		if e.Object == model.ObjectTypeRunnerCompletion {
			continue
		}
		if e.IsPartial {
			for _, c := range e.Choices {
				fmt.Print(c.Delta.Content)
			}
		} else if hasAnswer(e.Response) {
			fmt.Printf("\nFinal answer: %s\n", e.Choices[0].Message.Content)
		}
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	sess, err := svc.GetSession(ctx, session.Key{AppName: appName, UserID: "demo-user", SessionID: "demo-session"})
	if err != nil {
		return err
	}
	fmt.Println("Stored conversation:")
	for _, e := range sess.Events {
		if e.Response == nil {
			continue
		}
		for _, c := range e.Choices {
			fmt.Printf("  %s: %q (tool_calls=%d)\n", c.Message.Role, c.Message.Content, len(c.Message.ToolCalls))
		}
	}
	return runErr
}
