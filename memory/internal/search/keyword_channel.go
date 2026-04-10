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

	"trpc.group/trpc-go/trpc-agent-go/internal/retrieval"
	"trpc.group/trpc-go/trpc-agent-go/memory"
)

var (
	errNilKeywordEntries = errors.New("memory search keyword channel: entries provider is nil")
	errNilKeywordSearch  = errors.New("memory search keyword channel: search function is nil")
)

// KeywordSearchFunc performs keyword recall over in-memory entries.
type KeywordSearchFunc func(
	entries []*memory.Entry,
	opts memory.SearchOptions,
	minScore float64,
	defaultMaxResults int,
) []*memory.Entry

// KeywordChannel adapts in-memory keyword search into a retrieval channel.
type KeywordChannel struct {
	Entries           func(ctx context.Context, req Request) ([]*memory.Entry, error)
	SearchFunc        KeywordSearchFunc
	MinScore          float64
	DefaultMaxResults int
	PrepareOptions    func(req Request) memory.SearchOptions
}

// Recall executes keyword search over the provided entry set.
func (c KeywordChannel) Recall(
	ctx context.Context,
	req Request,
) ([]retrieval.Hit[*memory.Entry], error) {
	if c.Entries == nil {
		return nil, errNilKeywordEntries
	}
	if c.SearchFunc == nil {
		return nil, errNilKeywordSearch
	}
	entries, err := c.Entries(ctx, req)
	if err != nil {
		return nil, err
	}
	opts := req.Options
	if c.PrepareOptions != nil {
		opts = c.PrepareOptions(req)
	}
	results := c.SearchFunc(entries, opts, c.MinScore, c.DefaultMaxResults)
	return HitsFromEntries(results), nil
}
