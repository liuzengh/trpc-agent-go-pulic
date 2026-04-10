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
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/knowledge/document"
	q "trpc.group/trpc-go/trpc-agent-go/knowledge/query"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/searchfilter"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"
)

type stubEmbedder struct{}

func (stubEmbedder) GetEmbedding(ctx context.Context, text string) ([]float64, error) {
	return []float64{1, 2, 3}, nil
}
func (stubEmbedder) GetEmbeddingWithUsage(ctx context.Context, text string) ([]float64, map[string]any, error) {
	return []float64{1, 2, 3}, nil, nil
}
func (stubEmbedder) GetDimensions() int { return 3 }

type stubEnhancer struct{}

func (stubEnhancer) EnhanceQuery(ctx context.Context, req *q.Request) (*q.Enhanced, error) {
	return &q.Enhanced{Enhanced: req.Query + " enhanced"}, nil
}

type stubVectorStore struct {
	query  *vectorstore.SearchQuery
	result *vectorstore.SearchResult
}

func (s *stubVectorStore) Add(context.Context, *document.Document, []float64) error { return nil }
func (s *stubVectorStore) Get(context.Context, string) (*document.Document, []float64, error) {
	return nil, nil, nil
}
func (s *stubVectorStore) Update(context.Context, *document.Document, []float64) error { return nil }
func (s *stubVectorStore) Delete(context.Context, string) error                        { return nil }
func (s *stubVectorStore) Search(_ context.Context, query *vectorstore.SearchQuery) (*vectorstore.SearchResult, error) {
	s.query = query
	return s.result, nil
}
func (s *stubVectorStore) DeleteByFilter(context.Context, ...vectorstore.DeleteOption) error {
	return nil
}
func (s *stubVectorStore) UpdateByFilter(context.Context, ...vectorstore.UpdateByFilterOption) (int64, error) {
	return 0, nil
}
func (s *stubVectorStore) Count(context.Context, ...vectorstore.CountOption) (int, error) {
	return 0, nil
}
func (s *stubVectorStore) GetMetadata(context.Context, ...vectorstore.GetMetadataOption) (map[string]vectorstore.DocumentMetadata, error) {
	return nil, nil
}
func (s *stubVectorStore) Close() error { return nil }

func TestEnhanceAndEmbedRewriter(t *testing.T) {
	rewriter := EnhanceAndEmbedRewriter{
		Enhancer: stubEnhancer{},
		Embedder: stubEmbedder{},
	}
	req, err := rewriter.Rewrite(context.Background(), Request{Text: "query"})
	if err != nil {
		t.Fatalf("Rewrite() error = %v", err)
	}
	if req.FinalQuery != "query enhanced" {
		t.Fatalf("FinalQuery = %q, want %q", req.FinalQuery, "query enhanced")
	}
	if len(req.Embedding) != 3 {
		t.Fatalf("len(Embedding) = %d, want 3", len(req.Embedding))
	}
}

func TestTopKPostprocessor(t *testing.T) {
	post := TopKPostprocessor{}
	hits, err := post.Postprocess(context.Background(), Request{Limit: 1}, HitsFromResults([]*Result{
		{Document: &document.Document{ID: "doc1"}, Score: 0.9},
		{Document: &document.Document{ID: "doc2"}, Score: 0.8},
	}))
	if err != nil {
		t.Fatalf("Postprocess() error = %v", err)
	}
	if len(hits) != 1 || hits[0].ID != "doc1" {
		t.Fatalf("hits = %#v, want only doc1", hits)
	}
}

func TestResolveRequestAndSearchQuery(t *testing.T) {
	filter := ResolveFilter(
		[]string{"doc1"},
		map[string]any{"tenant": "t1"},
		&searchfilter.UniversalFilterCondition{Operator: searchfilter.OperatorAnd},
	)
	req := ResolveRequest("query", nil, "u1", "s1", 3, 0.4, filter, vectorstore.SearchModeHybrid)
	req.FinalQuery = "query enhanced"
	req.Embedding = []float64{1, 2, 3}

	searchQuery := req.SearchQuery()
	if searchQuery.Query != "query enhanced" {
		t.Fatalf("SearchQuery().Query = %q, want %q", searchQuery.Query, "query enhanced")
	}
	if searchQuery.SearchMode != vectorstore.SearchModeHybrid {
		t.Fatalf("SearchQuery().SearchMode = %v, want %v", searchQuery.SearchMode, vectorstore.SearchModeHybrid)
	}
	if searchQuery.Filter == nil || len(searchQuery.Filter.IDs) != 1 {
		t.Fatalf("SearchQuery().Filter = %#v, want populated filter", searchQuery.Filter)
	}
}

func TestHitsFromSearchResult(t *testing.T) {
	hits := HitsFromSearchResult(&vectorstore.SearchResult{
		Results: []*vectorstore.ScoredDocument{
			{Document: &document.Document{ID: "doc1"}, Score: 0.9},
			nil,
			{Document: &document.Document{ID: "doc2"}, Score: 0.8},
		},
	})
	if len(hits) != 2 {
		t.Fatalf("len(hits) = %d, want 2", len(hits))
	}
	if hits[0].ID != "doc1" || hits[1].ID != "doc2" {
		t.Fatalf("hits = %#v, want doc1/doc2", hits)
	}
}

func TestVectorStoreChannelUsesRequestAdapters(t *testing.T) {
	store := &stubVectorStore{
		result: &vectorstore.SearchResult{
			Results: []*vectorstore.ScoredDocument{
				{Document: &document.Document{ID: "doc1"}, Score: 0.7},
			},
		},
	}
	channel := VectorStoreChannel{VectorStore: store}
	req := ResolveRequest("query", nil, "u1", "s1", 2, 0.5, ResolveFilter(
		[]string{"doc1"},
		map[string]any{"tenant": "t1"},
		nil,
	), vectorstore.SearchModeVector)
	req.FinalQuery = "query enhanced"
	req.Embedding = []float64{1, 2, 3}

	hits, err := channel.Recall(context.Background(), req)
	if err != nil {
		t.Fatalf("Recall() error = %v", err)
	}
	if store.query == nil || store.query.Query != "query enhanced" {
		t.Fatalf("store.query = %#v, want query enhanced", store.query)
	}
	if store.query.SearchMode != vectorstore.SearchModeVector {
		t.Fatalf("store.query.SearchMode = %v, want %v", store.query.SearchMode, vectorstore.SearchModeVector)
	}
	if len(hits) != 1 || hits[0].ID != "doc1" {
		t.Fatalf("hits = %#v, want doc1", hits)
	}
}
