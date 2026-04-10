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
	"errors"
	"reflect"
	"testing"
)

func TestHybridPipelineRunOrder(t *testing.T) {
	ctx := context.Background()
	steps := make([]string, 0, 5)

	pipeline := &HybridPipeline[string, string]{
		Pipelines: []NamedPipeline[string, string]{
			{
				Name: "dense",
				Pipeline: &Branch[string, string]{
					Recall: ChannelFunc[string, string](func(
						_ context.Context,
						req string,
					) ([]Hit[string], error) {
						steps = append(steps, "branch:dense:"+req)
						return []Hit[string]{{ID: "1", Item: "dense"}}, nil
					}),
				},
			},
			{
				Name: "keyword",
				Pipeline: &Branch[string, string]{
					Recall: ChannelFunc[string, string](func(
						_ context.Context,
						req string,
					) ([]Hit[string], error) {
						steps = append(steps, "branch:keyword:"+req)
						return []Hit[string]{{ID: "2", Item: "keyword"}}, nil
					}),
				},
			},
		},
		Fusion: FusionFunc[string](func(
			_ context.Context,
			inputs []NamedHits[string],
		) ([]Hit[string], error) {
			steps = append(steps, "fusion:"+inputs[0].Name+"+"+inputs[1].Name)
			return []Hit[string]{
				{ID: "1", Item: inputs[0].Hits[0].Item},
				{ID: "2", Item: inputs[1].Hits[0].Item},
			}, nil
		}),
		Rerank: RerankerFunc[string, string](func(
			_ context.Context,
			req string,
			hits []Hit[string],
		) ([]Hit[string], error) {
			steps = append(steps, "rerank:"+req)
			return append(hits, Hit[string]{ID: "3", Item: "reranked"}), nil
		}),
		Post: []Postprocessor[string, string]{
			PostprocessorFunc[string, string](func(
				_ context.Context,
				req string,
				hits []Hit[string],
			) ([]Hit[string], error) {
				steps = append(steps, "post:"+req)
				return hits, nil
			}),
		},
	}

	got, err := pipeline.Run(ctx, "query")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if want := []string{
		"branch:dense:query",
		"branch:keyword:query",
		"fusion:dense+keyword",
		"rerank:query",
		"post:query",
	}; !reflect.DeepEqual(steps, want) {
		t.Fatalf("steps = %v, want %v", steps, want)
	}
	if len(got) != 3 || got[2].Item != "reranked" {
		t.Fatalf("unexpected hits: %#v", got)
	}
}

func TestFallbackPipelineRun(t *testing.T) {
	ctx := context.Background()
	steps := make([]string, 0, 4)

	pipeline := &FallbackPipeline[string, string]{
		Primary: &Branch[string, string]{
			Recall: ChannelFunc[string, string](func(
				_ context.Context,
				req string,
			) ([]Hit[string], error) {
				steps = append(steps, "primary:"+req)
				return []Hit[string]{{ID: "1", Item: "primary"}}, nil
			}),
		},
		Policy: FallbackPolicyFunc[string, string](func(
			_ context.Context,
			req string,
			primary []Hit[string],
		) (string, bool, error) {
			steps = append(steps, "policy:"+req)
			return req + "-fallback", true, nil
		}),
		Fallback: &Branch[string, string]{
			Recall: ChannelFunc[string, string](func(
				_ context.Context,
				req string,
			) ([]Hit[string], error) {
				steps = append(steps, "fallback:"+req)
				return []Hit[string]{{ID: "2", Item: "fallback"}}, nil
			}),
		},
		Merge: MergerFunc[string, string](func(
			_ context.Context,
			req string,
			primary []Hit[string],
			fallback []Hit[string],
		) ([]Hit[string], error) {
			steps = append(steps, "merge:"+req)
			return append(primary, fallback...), nil
		}),
	}

	got, err := pipeline.Run(ctx, "query")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}

	if want := []string{
		"primary:query",
		"policy:query",
		"fallback:query-fallback",
		"merge:query",
	}; !reflect.DeepEqual(steps, want) {
		t.Fatalf("steps = %v, want %v", steps, want)
	}
	if len(got) != 2 {
		t.Fatalf("unexpected hits: %#v", got)
	}
}

func TestFallbackPipelineNoFallback(t *testing.T) {
	ctx := context.Background()

	pipeline := &FallbackPipeline[string, string]{
		Primary: &Branch[string, string]{
			Recall: ChannelFunc[string, string](func(
				_ context.Context,
				req string,
			) ([]Hit[string], error) {
				return []Hit[string]{{ID: "1", Item: req}}, nil
			}),
		},
		Policy: FallbackPolicyFunc[string, string](func(
			_ context.Context,
			req string,
			primary []Hit[string],
		) (string, bool, error) {
			return req, false, nil
		}),
	}

	got, err := pipeline.Run(ctx, "query")
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if want := []Hit[string]{{ID: "1", Item: "query"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("Run() = %#v, want %#v", got, want)
	}
}

func TestFallbackPipelineErrors(t *testing.T) {
	ctx := context.Background()
	wantErr := errors.New("boom")

	pipeline := &FallbackPipeline[string, string]{
		Primary: &Branch[string, string]{
			Recall: ChannelFunc[string, string](func(
				_ context.Context,
				req string,
			) ([]Hit[string], error) {
				return nil, wantErr
			}),
		},
	}

	_, err := pipeline.Run(ctx, "query")
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}
