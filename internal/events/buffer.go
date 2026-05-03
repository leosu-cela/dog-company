package events

import (
	"context"
	"encoding/json"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	defaultFlushTimeout    = 10 * time.Second
	defaultFlushConcurrency = 4
)

// Buffer 是一個 in-memory 事件緩衝。Append 累積到 flushAt 筆數時整批 insert。
// 並起一個 goroutine 在 idleFlushInterval 過期時 flush 殘餘事件，避免低流量時段事件
// 久存 RAM。Close() 在 server 關機時 flush 所有尚未寫入的事件。
//
// 注意：Buffer 不保證 100% 持久化。Server 強制中斷（kill -9 / OOM / panic 未捕捉）時，
// pending 事件會遺失。屬於 best-effort 的審計 log，不適合用作金流憑證。
type Buffer struct {
	mu        sync.Mutex
	pending   []Event
	repo      IEventRepository
	db        *gorm.DB
	flushAt   int
	idleEvery time.Duration

	flushTimeout time.Duration
	flushSem     chan struct{}
	wg           sync.WaitGroup

	stopCh chan struct{}
	doneCh chan struct{}
}

// NewBuffer 建立一個事件緩衝。
//   - flushAt：累積到幾筆就批次 insert（建議 100）
//   - idleEvery：每隔多久強制 flush 殘餘事件（建議 5 * time.Minute）
func NewBuffer(db *gorm.DB, repo IEventRepository, flushAt int, idleEvery time.Duration) *Buffer {
	b := &Buffer{
		repo:         repo,
		db:           db,
		flushAt:      flushAt,
		idleEvery:    idleEvery,
		flushTimeout: defaultFlushTimeout,
		flushSem:     make(chan struct{}, defaultFlushConcurrency),
		stopCh:       make(chan struct{}),
		doneCh:       make(chan struct{}),
	}
	go b.idleLoop()
	return b
}

// Append 紀錄一筆事件。userUID 取自 JWT，無需查 DB。
// payload 為任意 marshal 過的 JSON；nil 代表無 payload。
func (b *Buffer) Append(userUID uuid.UUID, eventType string, payload json.RawMessage) {
	e := Event{
		UserUID:   userUID,
		EventType: eventType,
		Payload:   payload,
		CreatedAt: time.Now(),
	}

	b.mu.Lock()
	b.pending = append(b.pending, e)
	shouldFlush := len(b.pending) >= b.flushAt
	var batch []Event
	if shouldFlush {
		batch = b.pending
		b.pending = nil
	}
	b.mu.Unlock()

	if shouldFlush {
		b.flushBatchAsync(batch)
	}
}

// flushBatchAsync 試圖搶 semaphore 後丟出 goroutine 寫 DB。
// Semaphore 滿表示 DB 慢、in-flight goroutine 已達上限，直接丟棄這批避免 goroutine 暴衝。
// best-effort log 接受丟資料優於 OOM。
func (b *Buffer) flushBatchAsync(batch []Event) {
	if len(batch) == 0 {
		return
	}
	select {
	case b.flushSem <- struct{}{}:
	default:
		log.Printf("[EventBuffer] flush concurrency exhausted, dropping %d events", len(batch))
		return
	}
	b.wg.Go(func() {
		defer func() { <-b.flushSem }()
		b.flushBatch(batch)
	})
}

// flushBatch 真正寫 DB，失敗只 log，事件丟棄（避免 batch 卡死後續批次）。
// 用 timeout context 防止 DB hang 時 goroutine 永不返回。
func (b *Buffer) flushBatch(batch []Event) {
	if len(batch) == 0 {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), b.flushTimeout)
	defer cancel()
	tx := b.db.WithContext(ctx)
	if err := b.repo.BulkInsert(tx, batch); err != nil {
		log.Printf("[EventBuffer] flush %d events failed: %v", len(batch), err)
		return
	}
}

// idleLoop 定期把殘餘事件 flush，避免低流量時段累積太久。
func (b *Buffer) idleLoop() {
	defer close(b.doneCh)
	ticker := time.NewTicker(b.idleEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			b.flushPending()
		case <-b.stopCh:
			b.flushPending()
			return
		}
	}
}

// flushPending 取出所有 pending 並同步 flush（idleLoop 與 Close 路徑使用）。
func (b *Buffer) flushPending() {
	b.mu.Lock()
	if len(b.pending) == 0 {
		b.mu.Unlock()
		return
	}
	batch := b.pending
	b.pending = nil
	b.mu.Unlock()
	b.flushBatch(batch)
}

// Close 通知 idle goroutine 停止、flush 殘餘事件，並等待所有 in-flight flush goroutine 完成。
// 應該在 server graceful shutdown 時呼叫。
func (b *Buffer) Close() {
	close(b.stopCh)
	<-b.doneCh
	b.wg.Wait()
}
