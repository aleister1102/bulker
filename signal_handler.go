package main

import (
	"os"
	"os/signal"
	"syscall"
)

type SignalHandler struct {
	interruptChan chan os.Signal
	cleanupFunc   func() error
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
	signal.Stop(sh.interruptChan)
	close(sh.interruptChan)
}
