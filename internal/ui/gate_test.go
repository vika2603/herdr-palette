package ui

import (
	"testing"
	"time"
)

func TestClosingWaitsForAWriteInProgress(t *testing.T) {
	var g stateGate
	started, release, wrote := make(chan struct{}), make(chan struct{}), make(chan struct{})
	go g.write(func() {
		close(started)
		<-release
		close(wrote)
	})
	<-started

	closed := make(chan struct{})
	go func() {
		g.close()
		close(closed)
	}()
	select {
	case <-closed:
		t.Fatal("the gate closed while a write was still going")
	case <-time.After(50 * time.Millisecond):
	}

	close(release)
	<-closed
	select {
	case <-wrote:
	default:
		t.Error("the gate closed before the write finished")
	}
}

func TestAWriteAfterClosingIsTurnedAway(t *testing.T) {
	var g stateGate
	g.close()
	g.write(func() { t.Error("a write ran after the gate closed") })
}
