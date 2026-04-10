//
// Tencent is pleased to support the open source community by making trpc-agent-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//

// Package search assembles the knowledge retrieval pipeline between
// knowledge/retriever and knowledge/vectorstore. It owns knowledge-side request
// shaping, rewrite/rerank adapters, and vectorstore result adaptation, while
// leaving backend-specific SearchMode execution to vectorstore packages.
package search

import (
	"trpc.group/trpc-go/trpc-agent-go/knowledge/query"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/searchfilter"
	"trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"
)

// Request is the internal knowledge retrieval request.
type Request struct {
	Text       string
	History    []query.ConversationMessage
	UserID     string
	SessionID  string
	Limit      int
	MinScore   float64
	Filter     *vectorstore.SearchFilter
	SearchMode vectorstore.SearchMode

	FinalQuery string
	Embedding  []float64
}

// ResolveRequest builds the internal knowledge retrieval request.
func ResolveRequest(
	text string,
	history []query.ConversationMessage,
	userID string,
	sessionID string,
	limit int,
	minScore float64,
	filter *vectorstore.SearchFilter,
	searchMode vectorstore.SearchMode,
) Request {
	return Request{
		Text:       text,
		History:    history,
		UserID:     userID,
		SessionID:  sessionID,
		Limit:      limit,
		MinScore:   minScore,
		Filter:     filter,
		SearchMode: searchMode,
	}
}

// ResolveFilter builds the vectorstore search filter used by knowledge search.
func ResolveFilter(
	documentIDs []string,
	metadata map[string]any,
	filterCondition *searchfilter.UniversalFilterCondition,
) *vectorstore.SearchFilter {
	return &vectorstore.SearchFilter{
		IDs:             documentIDs,
		Metadata:        metadata,
		FilterCondition: filterCondition,
	}
}

// SearchQuery builds the vectorstore search query for the request.
func (r Request) SearchQuery() *vectorstore.SearchQuery {
	return &vectorstore.SearchQuery{
		Query:      r.FinalQuery,
		Vector:     r.Embedding,
		Limit:      r.Limit,
		MinScore:   r.MinScore,
		Filter:     r.Filter,
		SearchMode: r.SearchMode,
	}
}
