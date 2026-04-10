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
	"trpc.group/trpc-go/trpc-agent-go/knowledge/document"
)

// TopKPostprocessor truncates knowledge results to the configured limit.
type TopKPostprocessor struct{}

// Postprocess keeps only the top K knowledge results.
func (TopKPostprocessor) Postprocess(
	_ context.Context,
	req Request,
	hits []retrieval.Hit[*document.Document],
) ([]retrieval.Hit[*document.Document], error) {
	if req.Limit <= 0 || len(hits) <= req.Limit {
		return hits, nil
	}
	return append([]retrieval.Hit[*document.Document](nil), hits[:req.Limit]...), nil
}
