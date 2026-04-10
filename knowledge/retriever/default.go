//
// Tencent is pleased to support the open source community by making trpc-agent-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//
//

package retriever

import (
	"context"

	"trpc.group/trpc-go/trpc-agent-go/internal/retrieval"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/document"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/embedder"
	isearch "trpc.group/trpc-go/trpc-agent-go/knowledge/internal/search"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/query"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/reranker"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"
)

// DefaultRetriever implements the complete RAG pipeline.
type DefaultRetriever struct {
	embedder      embedder.Embedder
	vectorStore   vectorstore.VectorStore
	queryEnhancer query.Enhancer
	reranker      reranker.Reranker
}

// Option represents a functional option for configuring DefaultRetriever.
type Option func(*DefaultRetriever)

// WithEmbedder sets the embedder for the retriever.
func WithEmbedder(e embedder.Embedder) Option {
	return func(dr *DefaultRetriever) {
		dr.embedder = e
	}
}

// WithVectorStore sets the vector store for the retriever.
func WithVectorStore(vs vectorstore.VectorStore) Option {
	return func(dr *DefaultRetriever) {
		dr.vectorStore = vs
	}
}

// WithQueryEnhancer sets the query enhancer for the retriever.
func WithQueryEnhancer(qe query.Enhancer) Option {
	return func(dr *DefaultRetriever) {
		dr.queryEnhancer = qe
	}
}

// WithReranker sets the reranker for the retriever.
func WithReranker(r reranker.Reranker) Option {
	return func(dr *DefaultRetriever) {
		dr.reranker = r
	}
}

// New creates a new default retriever with the given options.
func New(opts ...Option) *DefaultRetriever {
	dr := &DefaultRetriever{}

	for _, opt := range opts {
		opt(dr)
	}

	return dr
}

// Retrieve implements the Retriever interface by executing the complete RAG pipeline.
func (dr *DefaultRetriever) Retrieve(ctx context.Context, q *Query) (*Result, error) {
	var filter *vectorstore.SearchFilter
	if q.Filter != nil {
		filter = isearch.ResolveFilter(q.Filter.DocumentIDs, q.Filter.Metadata, q.Filter.FilterCondition)
	}
	req := isearch.ResolveRequest(
		q.Text,
		q.History,
		q.UserID,
		q.SessionID,
		q.Limit,
		q.MinScore,
		filter,
		vectorstore.SearchMode(q.SearchMode),
	)

	var rerankerAdapter retrieval.Reranker[isearch.Request, *document.Document]
	if dr.reranker != nil {
		rerankerAdapter = isearch.RerankerAdapter{Reranker: dr.reranker}
	}

	branch := isearch.NewBranch(
		isearch.EnhanceAndEmbedRewriter{
			Enhancer: dr.queryEnhancer,
			Embedder: dr.embedder,
		},
		isearch.VectorStoreChannel{VectorStore: dr.vectorStore},
		rerankerAdapter,
		isearch.TopKPostprocessor{},
	)
	results, err := isearch.Run(ctx, branch, req)
	if err != nil {
		return nil, err
	}

	finalResults := make([]*RelevantDocument, 0, len(results))
	for _, result := range results {
		if result == nil {
			continue
		}
		finalResults = append(finalResults, &RelevantDocument{
			Document: result.Document,
			Score:    result.Score,
		})
	}
	return &Result{Documents: finalResults}, nil
}

// Close implements the Retriever interface.
func (dr *DefaultRetriever) Close() error {
	// Close components if they support closing.
	return nil
}
