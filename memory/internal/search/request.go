//
// Tencent is pleased to support the open source community by making trpc-agent-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//

// Package search assembles memory retrieval pipelines on top of the generic
// retrieval core.
package search

import "trpc.group/trpc-go/trpc-agent-go/memory"

// Request is the memory retrieval request envelope.
type Request struct {
	UserKey memory.UserKey
	Options memory.SearchOptions
}

// ResolveRequest builds a Request from user key, query, and search options.
func ResolveRequest(
	userKey memory.UserKey,
	query string,
	opts ...memory.SearchOption,
) Request {
	return Request{
		UserKey: userKey,
		Options: memory.ResolveSearchOptions(query, opts),
	}
}

// WithOptions returns a copy with updated options.
func (r Request) WithOptions(options memory.SearchOptions) Request {
	r.Options = options
	return r
}
