package lrucache

import (
	"sync"
	"sync/atomic"
)

type AssertRWLock struct {
	rwMutex sync.RWMutex
	writer  int32 // Tracks if the lock is currently held by a writer (0 = no writer, 1 = writer)
}

func (l *AssertRWLock) Lock() {
	l.rwMutex.Lock()
	if !atomic.CompareAndSwapInt32(&l.writer, 0, 1) {
		panic("Write lock already held!")
	}
}

func (l *AssertRWLock) Unlock() {
	if !atomic.CompareAndSwapInt32(&l.writer, 1, 0) {
		panic("Unlock called when no write lock is held!")
	}
	l.rwMutex.Unlock()
}

func (l *AssertRWLock) RLock() {
	l.rwMutex.RLock()
}

func (l *AssertRWLock) RUnlock() {
	l.rwMutex.RUnlock()
}

func (l *AssertRWLock) AssertLocked() {
	// Ensure a write lock is held
	if atomic.LoadInt32(&l.writer) != 1 {
		panic("Write lock is not held")
	}
}
