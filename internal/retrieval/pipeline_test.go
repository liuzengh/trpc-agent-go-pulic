//
// Tencent is pleased to support the open source community by making trpc-agent-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//

package retrieval

import (
	"context"
	"reflect"
	"testing"
)

func TestBranchRunOrder(t *testing.T) {
	ctx := context.Background()
	steps := make([]string, 0, 4)

	branch := &Branch[string, string]{
		Rewrite: RewriterFunc[string](func(_ context.Context, req string) (string, error) {
			steps = append(steps, "rewrite")
			return req + "-rewrite", nil
		}),
		Recall: ChannelFunc[string, string](func(_ context.Context, req string) ([]Hit[string], error) {
			steps = append(steps, "recall:"+req)
			return []Hit[string]{{ID: "1", Item: req}}, nil
		}),
		Rerank: RerankerFunc[string, string](func(
			_ context.Context,
			req string,
			hits []Hit[string],
		) ([]Hit[string], error) {
			steps = append(steps, "rerank:"+req)
			hits[0].Rank = 7
			return hits, nil
		}),
		Post: []Postprocessor[string, string]{
			PostprocessorFunc[string, string](func(
				_ context.Context,
				req string,
				hits []Hit[string],
			) ([]Hit[string], error) {
				steps = append(steps, "post:"+req)
				hits[0].Item = req + "-post"
				return hits, nil
			}),
		},
	}

	got, err := branch.Run(ctx, "start")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if want := []string{
		"rewrite",
		"recall:start-rewrite",
		"rerank:start-rewrite",
		"post:start-rewrite",
	}; !reflect.DeepEqual(steps, want) {
		t.Fatalf("steps = %v, want %v", steps, want)
	}
	if len(got) != 1 || got[0].Item != "start-rewrite-post" || got[0].Rank != 7 {
		t.Fatalf("unexpected hits: %#v", got)
	}
}

func TestBranchRunSkipsNilStages(t *testing.T) {
	ctx := context.Background()

	branch := &Branch[string, string]{
		Recall: ChannelFunc[string, string](func(
			_ context.Context,
			req string,
		) ([]Hit[string], error) {
			return []Hit[string]{{ID: "1", Item: req}}, nil
		}),
	}

	got, err := branch.Run(ctx, "query")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if want := []Hit[string]{{ID: "1", Item: "query"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Run() = %#v, want %#v", got, want)
	}
}

func TestBranchRunRequiresRecall(t *testing.T) {
	ctx := context.Background()

	_, err := (&Branch[string, string]{}).Run(ctx, "query")
	if err == nil || err.Error() != errNilRecallChannel.Error() {
		t.Fatalf("Run() error = %v, want %v", err, errNilRecallChannel)
	}
}
