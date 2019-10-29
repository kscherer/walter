package util

import (
	"context"
	"os"
	"os/signal"
	"sync/atomic"
)

type InterruptHandler func() bool

// Create channel to listen to the completion of cleanup
var cleanupListener = make(chan struct{})

// Create channel to listen to interrupt signal and setup interrupt handler
var interruptListener = make(chan os.Signal, 1)

// Atomic value indicating if the program is interrupted
var interrupted atomic.Value

// listenInterrupt listens for SIGINT, atomically sets the value
// for interrupted and run the handler function
func listenInterrupt(handleInterrupt func()) {
	<-interruptListener
	interrupted.Store(true)

	handleInterrupt()
}

// PrepareInterrupt initializes interrupted value and sets up
// the interrupt listener
func PrepareInterrupt(handleInterrupt func()) {
	interrupted.Store(false)

	signal.Notify(interruptListener, os.Interrupt)
	go listenInterrupt(handleInterrupt)
}

// Interrupted returns true when a SIGINT is received for the current context
func Interrupted(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return interrupted.Load().(bool)
	default:
		return false
	}
}

// WaitInterrupt blocks until an interrupt happens
func WaitInterrupt() {
	<-interruptListener
}

// WaitClean blocks until cleanupListener channel receives
// data. Right now no data is expected to be sent through
// the channel and it will block forever.
func WaitClean() {
	<-cleanupListener
}
