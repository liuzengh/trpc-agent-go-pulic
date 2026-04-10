//
// Tencent is pleased to support the open source community by making trpc-agent-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//

// Package search provides shared internal retrieval orchestration for
// knowledge vectorstore SearchMode dispatch. It owns mode-level routing and
// result adaptation, while each backend still owns its searchByVector,
// searchByKeyword, searchByHybrid, and searchByFilter implementations.
package search

import "trpc.group/trpc-go/trpc-agent-go/knowledge/vectorstore"

// Request is the internal knowledge vectorstore search request.
type Request struct {
	Query *vectorstore.SearchQuery
}
