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

var errNilRecallChannel = errors.New("retrieval branch: recall channel is nil")

// Pipeline runs a retrieval pipeline.
type Pipeline[Req, T any] interface {
	Run(ctx context.Context, req Req) ([]Hit[T], error)
}

// Channel recalls candidate hits for a request.
type Channel[Req, T any] interface {
	Recall(ctx context.Context, req Req) ([]Hit[T], error)
}

// ChannelFunc adapts a function to Channel.
type ChannelFunc[Req, T any] func(ctx context.Context, req Req) ([]Hit[T], error)

// Recall executes the wrapped function.
func (f ChannelFunc[Req, T]) Recall(
	ctx context.Context,
	req Req,
) ([]Hit[T], error) {
	return f(ctx, req)
}

// Rewriter rewrites an incoming request before recall.
type Rewriter[Req any] interface {
	Rewrite(ctx context.Context, req Req) (Req, error)
}

// RewriterFunc adapts a function to Rewriter.
type RewriterFunc[Req any] func(ctx context.Context, req Req) (Req, error)

// Rewrite executes the wrapped function.
func (f RewriterFunc[Req]) Rewrite(ctx context.Context, req Req) (Req, error) {
	return f(ctx, req)
}

// Reranker reranks recalled hits.
type Reranker[Req, T any] interface {
	Rerank(ctx context.Context, req Req, hits []Hit[T]) ([]Hit[T], error)
}

// RerankerFunc adapts a function to Reranker.
type RerankerFunc[Req, T any] func(
	ctx context.Context,
	req Req,
	hits []Hit[T],
) ([]Hit[T], error)

// Rerank executes the wrapped function.
func (f RerankerFunc[Req, T]) Rerank(
	ctx context.Context,
	req Req,
	hits []Hit[T],
) ([]Hit[T], error) {
	return f(ctx, req, hits)
}

// Branch is the basic Rewrite -> Recall -> Rerank -> Post pipeline.
type Branch[Req, T any] struct {
	Rewrite Rewriter[Req]
	Recall  Channel[Req, T]
	Rerank  Reranker[Req, T]
	Post    []Postprocessor[Req, T]
}

// Run executes the branch sequentially.
func (b *Branch[Req, T]) Run(
	ctx context.Context,
	req Req,
) ([]Hit[T], error) {
	if b == nil || b.Recall == nil {
		return nil, errNilRecallChannel
	}
	var err error
	if b.Rewrite != nil {
		req, err = b.Rewrite.Rewrite(ctx, req)
		if err != nil {
			return nil, err
		}
	}
	hits, err := b.Recall.Recall(ctx, req)
	if err != nil {
		return nil, err
	}
	if b.Rerank != nil {
		hits, err = b.Rerank.Rerank(ctx, req, hits)
		if err != nil {
			return nil, err
		}
	}
	return applyPostprocessors(ctx, req, hits, b.Post)
}
