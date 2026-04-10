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
	"trpc.group/trpc-go/trpc-agent-go/session"
)

// TopKPostprocessor truncates session event search results to the configured limit.
type TopKPostprocessor struct {
	MaxResults int
}

// Postprocess keeps only the top K session event results.
func (p TopKPostprocessor) Postprocess(
	_ context.Context,
	_ Request,
	hits []retrieval.Hit[session.EventSearchResult],
) ([]retrieval.Hit[session.EventSearchResult], error) {
	if p.MaxResults <= 0 || len(hits) <= p.MaxResults {
		return hits, nil
	}
	return append([]retrieval.Hit[session.EventSearchResult](nil), hits[:p.MaxResults]...), nil
}
