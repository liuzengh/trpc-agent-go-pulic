//
// Tencent is pleased to support the open source community by making trpc-agent-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//

// Package search provides internal retrieval adapters for session event search.
package search

import "trpc.group/trpc-go/trpc-agent-go/session"

// Request is the internal session event retrieval request.
type Request struct {
	Search session.EventSearchRequest
	Query  string
}
