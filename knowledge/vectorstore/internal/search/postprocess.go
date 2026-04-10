//
// Tencent is pleased to support the open source community by making trpc-agent-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//

package search

import (
	"context"

	"trpc.group/trpc-go/trpc-agent-go/internal/retrieval"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"
)

// TopKPostprocessor truncates vectorstore results to the requested limit.
type TopKPostprocessor struct{}

// Postprocess keeps only the top K results.
func (TopKPostprocessor) Postprocess(
	_ context.Context,
	req Request,
	hits []retrieval.Hit[*vectorstore.ScoredDocument],
) ([]retrieval.Hit[*vectorstore.ScoredDocument], error) {
	if req.Query == nil || req.Query.Limit <= 0 || len(hits) <= req.Query.Limit {
		return hits, nil
	}
	return append([]retrieval.Hit[*vectorstore.ScoredDocument](nil), hits[:req.Query.Limit]...), nil
}
