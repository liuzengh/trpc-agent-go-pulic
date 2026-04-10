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

	"trpc.group/trpc-go/trpc-agent-go/knowledge/embedder"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/query"
)

// EnhanceAndEmbedRewriter rewrites the query and prepares an embedding.
type EnhanceAndEmbedRewriter struct {
	Enhancer query.Enhancer
	Embedder embedder.Embedder
}

// Rewrite enhances the query and generates the embedding when available.
func (r EnhanceAndEmbedRewriter) Rewrite(
	ctx context.Context,
	req Request,
) (Request, error) {
	finalQuery := req.Text
	if r.Enhancer != nil {
		enhanced, err := r.Enhancer.EnhanceQuery(ctx, &query.Request{
			Query:     req.Text,
			History:   req.History,
			UserID:    req.UserID,
			SessionID: req.SessionID,
		})
		if err != nil {
			return Request{}, err
		}
		finalQuery = enhanced.Enhanced
	}
	req.FinalQuery = finalQuery

	if r.Embedder != nil && finalQuery != "" {
		embedding, err := r.Embedder.GetEmbedding(ctx, finalQuery)
		if err != nil {
			return Request{}, err
		}
		req.Embedding = embedding
	}
	return req, nil
}
