package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"
)

type SignalHandler struct {
	interruptChan chan os.Signal
	cleanupFunc   func() error
	once          sync.Once
	closed        bool
	mu            sync.RWMutex
}

func NewSignalHandler() *SignalHandler {
	return &SignalHandler{
		interruptChan: make(chan os.Signal, 1),
	}
}

func (sh *SignalHandler) Setup(cleanupFunc func() error) {
	sh.cleanupFunc = cleanupFunc
	signal.Notify(sh.interruptChan, os.Interrupt, syscall.SIGTERM)
}

func (sh *SignalHandler) InterruptChan() <-chan os.Signal {
	return sh.interruptChan
}

func (sh *SignalHandler) Stop() {
	sh.once.Do(func() {
		sh.mu.Lock()
		defer sh.mu.Unlock()

		if !sh.closed {
			signal.Stop(sh.interruptChan)
			close(sh.interruptChan)
			sh.closed = true
		}
	})
}
