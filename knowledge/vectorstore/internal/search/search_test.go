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
	"errors"
	"testing"

	"trpc.group/trpc-go/trpc-agent-go/internal/retrieval"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/document"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"
)

func TestModePipelineSelectsBranch(t *testing.T) {
	pipeline := &ModePipeline{
		Vector: NewVectorBranch(
			retrieval.ChannelFunc[Request, *vectorstore.ScoredDocument](func(
				_ context.Context,
				req Request,
			) ([]retrieval.Hit[*vectorstore.ScoredDocument], error) {
				return HitsFromSearchResult(&vectorstore.SearchResult{
					Results: []*vectorstore.ScoredDocument{{
						Document: &document.Document{ID: "vector"},
						Score:    0.9,
					}},
				}), nil
			}),
		),
	}

	result, err := Run(context.Background(), pipeline, &vectorstore.SearchQuery{
		SearchMode: vectorstore.SearchModeVector,
	})
	if err != nil {
		t.Fatalf("Run() error = %v", err)
	}
	if len(result.Results) != 1 || result.Results[0].Document.ID != "vector" {
		t.Fatalf("result = %#v, want vector hit", result)
	}
}

func TestModePipelineInvalidMode(t *testing.T) {
	wantErr := errors.New("invalid mode")
	pipeline := &ModePipeline{
		InvalidMode: func(mode vectorstore.SearchMode) error { return wantErr },
	}

	_, err := Run(context.Background(), pipeline, &vectorstore.SearchQuery{
		SearchMode: vectorstore.SearchMode(99),
	})
	if !errors.Is(err, wantErr) {
		t.Fatalf("Run() error = %v, want %v", err, wantErr)
	}
}

func TestTopKPostprocessor(t *testing.T) {
	hits, err := (TopKPostprocessor{}).Postprocess(
		context.Background(),
		Request{Query: &vectorstore.SearchQuery{Limit: 1}},
		HitsFromSearchResult(&vectorstore.SearchResult{
			Results: []*vectorstore.ScoredDocument{
				{Document: &document.Document{ID: "doc1"}, Score: 0.9},
				{Document: &document.Document{ID: "doc2"}, Score: 0.8},
			},
		}),
	)
	if err != nil {
		t.Fatalf("Postprocess() error = %v", err)
	}
	if len(hits) != 1 || hits[0].Item.Document.ID != "doc1" {
		t.Fatalf("hits = %#v, want only doc1", hits)
	}
}
