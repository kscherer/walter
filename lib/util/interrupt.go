package util

import (
	"context"
	"os"
	"os/signal"
	"sync/atomic"
)

// Create channel to listen to the completion of cleanup
var cleanupListener = make(chan struct{})

// Create channel to listen to interrupt signal and setup interrupt handler
var interruptListener = make(chan os.Signal)

// Atomic value indicating if the program is interrupted
var interrupted atomic.Value

// forceQuitChan will be closed if the second SIGINT is received.
// This channel does not pass any data
var forceQuitChan chan bool

// listenInterrupt listens for SIGINT, atomically sets the value
// for interrupted and run the handler function
func listenInterrupt(handleInterrupt func(), handleForceQuit func()) {
	<-interruptListener
	interrupted.Store(true)
	go waitForceQuit(handleForceQuit)

	handleInterrupt()
}

// waitForceQuit listens for the second SIGINT and close the forceQuitChan
func waitForceQuit(handleForceQuit func()) {
	<-interruptListener
	handleForceQuit()
}

// StartForceQuit closes the forceQuitChan channel to signal a force quit to
// running tasks
func StartForceQuit() {
	close(forceQuitChan)
}

// PrepareInterrupt initializes interrupted value and sets up
// the interrupt listener
func PrepareInterrupt(handleInterrupt func(), handleForceQuit func()) {
	interrupted.Store(false)

	forceQuitChan = make(chan bool)

	signal.Notify(interruptListener, os.Interrupt)
	go listenInterrupt(handleInterrupt, handleForceQuit)
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

// ForceQuit returns the forceQuitChan channel
func ForceQuit() <-chan bool {
	return forceQuitChan
}

// WaitClean blocks until cleanupListener channel receives
// data. Right now no data is expected to be sent through
// the channel and it will block forever.
func WaitClean() {
	<-cleanupListener
}
