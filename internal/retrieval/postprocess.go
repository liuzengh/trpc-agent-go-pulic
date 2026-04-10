//
// Tencent is pleased to support the open source community by making trpc-agent-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//

package retrieval

import "context"

// Postprocessor transforms ranked hits after recall/rerank stages.
type Postprocessor[Req, T any] interface {
	Postprocess(ctx context.Context, req Req, hits []Hit[T]) ([]Hit[T], error)
}

// PostprocessorFunc adapts a function to Postprocessor.
type PostprocessorFunc[Req, T any] func(
	ctx context.Context,
	req Req,
	hits []Hit[T],
) ([]Hit[T], error)

// Postprocess executes the wrapped function.
func (f PostprocessorFunc[Req, T]) Postprocess(
	ctx context.Context,
	req Req,
	hits []Hit[T],
) ([]Hit[T], error) {
	return f(ctx, req, hits)
}

func applyPostprocessors[Req, T any](
	ctx context.Context,
	req Req,
	hits []Hit[T],
	post []Postprocessor[Req, T],
) ([]Hit[T], error) {
	var err error
	for _, processor := range post {
		if processor == nil {
			continue
		}
		hits, err = processor.Postprocess(ctx, req, hits)
		if err != nil {
			return nil, err
		}
	}
	return hits, nil
}
