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
	"math"
	"testing"
	"time"

	"trpc.group/trpc-go/trpc-agent-go/event"
	"trpc.group/trpc-go/trpc-agent-go/internal/retrieval"
	"trpc.group/trpc-go/trpc-agent-go/model"
	"trpc.group/trpc-go/trpc-agent-go/session"
)

func TestRRFFusionPreservesDenseAndSparseSignals(t *testing.T) {
	now := time.Date(2025, 1, 2, 3, 4, 5, 0, time.UTC)
	dense := session.EventSearchResult{
		SessionKey:     session.Key{AppName: "app", UserID: "user", SessionID: "sess"},
		EventCreatedAt: now,
		Event:          event.Event{ID: "evt1"},
		Role:           model.RoleUser,
		Text:           "dense",
		Score:          0.9,
		DenseScore:     0.9,
	}
	keyword := dense
	keyword.Text = "keyword"
	keyword.Score = 0.7
	keyword.DenseScore = 0
	keyword.SparseScore = 0.7

	hits, err := (RRFFusion{K: 60}).Fuse(context.Background(), []retrieval.NamedHits[session.EventSearchResult]{
		{Name: "dense", Hits: HitsFromResults([]session.EventSearchResult{dense})},
		{Name: "keyword", Hits: HitsFromResults([]session.EventSearchResult{keyword})},
	})
	if err != nil {
		t.Fatalf("Fuse() error = %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("len(hits) = %d, want 1", len(hits))
	}
	got := hits[0].Item
	if got.DenseScore != 0.9 || got.SparseScore != 0.7 {
		t.Fatalf("scores = dense:%v sparse:%v, want 0.9/0.7", got.DenseScore, got.SparseScore)
	}
	wantScore := 1.0/61.0 + 1.0/61.0
	if math.Abs(hits[0].Score-wantScore) > 1e-9 {
		t.Fatalf("fused score = %v, want %v", hits[0].Score, wantScore)
	}
}
