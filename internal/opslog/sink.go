package opslog

import (
	"context"

	"hvc/pkg/logx"
)

// Sink 把结构化日志异步落到数据库。
type Sink struct {
	repo *Repository
}

// NewSink 创建运行日志 sink。
func NewSink(repo *Repository) *Sink {
	return &Sink{repo: repo}
}

// SavePersistedEntry 实现 logx.PersistenceSink。
func (s *Sink) SavePersistedEntry(item logx.PersistedEntry) error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.SaveStructured(context.Background(), item)
}

// SavePersistedEntries 实现 logx.BatchPersistenceSink。
func (s *Sink) SavePersistedEntries(items []logx.PersistedEntry) error {
	if s == nil || s.repo == nil {
		return nil
	}
	return s.repo.SaveStructuredBatch(context.Background(), items)
}
