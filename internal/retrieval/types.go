//
// Tencent is pleased to support the open source community by making trpc-agent-go available.
//
// Copyright (C) 2025 Tencent.  All rights reserved.
//
// trpc-agent-go is licensed under the Apache License Version 2.0.
//

// Package retrieval provides generic internal retrieval orchestration
// primitives. It owns terminology such as Hit, Branch, Pipeline, Channel,
// Rewriter, Reranker, Fusion, and Postprocessor, but it does not own
// memory/session/knowledge-specific request shaping, scoring, SQL, or backend
// execution details.
package retrieval

// Hit is a ranked retrieval result.
type Hit[T any] struct {
	ID      string
	Item    T
	Score   float64
	Rank    int
	Signals map[string]float64
	Meta    map[string]any
}
