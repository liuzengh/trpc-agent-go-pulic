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
)

var (
	errNilFusion           = errors.New("retrieval hybrid pipeline: fusion is nil")
	errNilPrimaryPipeline  = errors.New("retrieval fallback pipeline: primary pipeline is nil")
	errNilFallbackPipeline = errors.New("retrieval fallback pipeline: fallback pipeline is nil")
	errNilMerger           = errors.New("retrieval fallback pipeline: merger is nil")
	errEmptyHybridPipeline = errors.New("retrieval hybrid pipeline: no pipelines configured")
	errNilNamedPipeline    = errors.New("retrieval hybrid pipeline: named pipeline is nil")
)

// NamedPipeline binds a stable name to a pipeline.
type NamedPipeline[Req, T any] struct {
	Name     string
	Pipeline Pipeline[Req, T]
}

// NamedHits carries a named pipeline result into fusion.
type NamedHits[T any] struct {
	Name string
	Hits []Hit[T]
}

// Fusion combines ranked lists into one.
type Fusion[T any] interface {
	Fuse(ctx context.Context, inputs []NamedHits[T]) ([]Hit[T], error)
}

// FusionFunc adapts a function to Fusion.
type FusionFunc[T any] func(
	ctx context.Context,
	inputs []NamedHits[T],
) ([]Hit[T], error)

// Fuse executes the wrapped function.
func (f FusionFunc[T]) Fuse(
	ctx context.Context,
	inputs []NamedHits[T],
) ([]Hit[T], error) {
	return f(ctx, inputs)
}

// FallbackPolicy decides whether a fallback pipeline should run.
type FallbackPolicy[Req, T any] interface {
	ShouldFallback(
		ctx context.Context,
		req Req,
		primary []Hit[T],
	) (fallbackReq Req, ok bool, err error)
}

// FallbackPolicyFunc adapts a function to FallbackPolicy.
type FallbackPolicyFunc[Req, T any] func(
	ctx context.Context,
	req Req,
	primary []Hit[T],
) (Req, bool, error)

// ShouldFallback executes the wrapped function.
func (f FallbackPolicyFunc[Req, T]) ShouldFallback(
	ctx context.Context,
	req Req,
	primary []Hit[T],
) (Req, bool, error) {
	return f(ctx, req, primary)
}

// Merger merges primary and fallback hits.
type Merger[Req, T any] interface {
	Merge(
		ctx context.Context,
		req Req,
		primary []Hit[T],
		fallback []Hit[T],
	) ([]Hit[T], error)
}

// MergerFunc adapts a function to Merger.
type MergerFunc[Req, T any] func(
	ctx context.Context,
	req Req,
	primary []Hit[T],
	fallback []Hit[T],
) ([]Hit[T], error)

// Merge executes the wrapped function.
func (f MergerFunc[Req, T]) Merge(
	ctx context.Context,
	req Req,
	primary []Hit[T],
	fallback []Hit[T],
) ([]Hit[T], error) {
	return f(ctx, req, primary, fallback)
}

// HybridPipeline runs named pipelines in order and fuses their results.
type HybridPipeline[Req, T any] struct {
	Pipelines []NamedPipeline[Req, T]
	Fusion    Fusion[T]
	Rerank    Reranker[Req, T]
	Post      []Postprocessor[Req, T]
}

// Run executes all pipelines sequentially, fuses once, then reranks/posts.
func (p *HybridPipeline[Req, T]) Run(
	ctx context.Context,
	req Req,
) ([]Hit[T], error) {
	if p == nil || len(p.Pipelines) == 0 {
		return nil, errEmptyHybridPipeline
	}

	inputs := make([]NamedHits[T], 0, len(p.Pipelines))
	for _, named := range p.Pipelines {
		if named.Pipeline == nil {
			return nil, errNilNamedPipeline
		}
		hits, err := named.Pipeline.Run(ctx, req)
		if err != nil {
			return nil, err
		}
		inputs = append(inputs, NamedHits[T]{
			Name: named.Name,
			Hits: hits,
		})
	}

	var (
		hits []Hit[T]
		err  error
	)
	switch {
	case p.Fusion != nil:
		hits, err = p.Fusion.Fuse(ctx, inputs)
	case len(inputs) == 1:
		hits = append([]Hit[T](nil), inputs[0].Hits...)
	default:
		err = errNilFusion
	}
	if err != nil {
		return nil, err
	}

	if p.Rerank != nil {
		hits, err = p.Rerank.Rerank(ctx, req, hits)
		if err != nil {
			return nil, err
		}
	}
	return applyPostprocessors(ctx, req, hits, p.Post)
}

// FallbackPipeline runs the primary pipeline, then optionally fallback+merge.
type FallbackPipeline[Req, T any] struct {
	Primary  Pipeline[Req, T]
	Policy   FallbackPolicy[Req, T]
	Fallback Pipeline[Req, T]
	Merge    Merger[Req, T]
	Post     []Postprocessor[Req, T]
}

// Run executes the fallback pipeline sequentially.
func (p *FallbackPipeline[Req, T]) Run(
	ctx context.Context,
	req Req,
) ([]Hit[T], error) {
	if p == nil || p.Primary == nil {
		return nil, errNilPrimaryPipeline
	}

	hits, err := p.Primary.Run(ctx, req)
	if err != nil {
		return nil, err
	}

	if p.Policy != nil {
		fallbackReq, ok, policyErr := p.Policy.ShouldFallback(ctx, req, hits)
		if policyErr != nil {
			return nil, policyErr
		}
		if ok {
			if p.Fallback == nil {
				return nil, errNilFallbackPipeline
			}
			if p.Merge == nil {
				return nil, errNilMerger
			}
			fallbackHits, fallbackErr := p.Fallback.Run(ctx, fallbackReq)
			if fallbackErr != nil {
				return nil, fallbackErr
			}
			hits, err = p.Merge.Merge(ctx, req, hits, fallbackHits)
			if err != nil {
				return nil, err
			}
		}
	}

	return applyPostprocessors(ctx, req, hits, p.Post)
}
