package signal_test

import (
	"os"
	"os/signal"
	"syscall"
	"testing"

	sg "github.com/ofgp/ofgp-core/util/signal"
)

var isHandle = false

func testHandler() {
	isHandle = true
}

func TestSignal(t *testing.T) {
	ss := sg.NewSignalSet()
	ss.Register(syscall.SIGUSR1, testHandler)
	c := make(chan os.Signal)
	signal.Notify(c)
	defer signal.Stop(c)

	syscall.Kill(syscall.Getpid(), syscall.SIGUSR1)

	sig := <-c
	ss.Handle(sig)
	if !isHandle {
		t.Error("signal test failed")
	}
}
