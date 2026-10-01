// Buffer demonstrates a concurrent FIFO queue built with gx.Buffer.
//
// Run with:
//
//	go run ./examples/buffer
package main

import (
	"fmt"

	"github.com/shayanderson/gx"
)

func main() {
	pollWithoutBlocking()
	drainAfterClose()
	waitForNextValue()
}

func pollWithoutBlocking() {
	fmt.Println("non-blocking read:")

	b := gx.NewBuffer[string](1)
	if _, ok := b.TryNext(); !ok {
		fmt.Println("  buffer is empty; TryNext returned immediately")
	}
}

func drainAfterClose() {
	fmt.Println("\nFIFO delivery and close:")

	// The initial capacity is only a sizing hint. Buffer grows automatically
	// when producers outpace consumers.
	b := gx.NewBuffer[string](2)
	for _, message := range []string{"first", "second", "third"} {
		b.Push(message)
	}
	fmt.Printf("  queued %d messages\n", b.Len())

	// Close rejects future Push calls but does not discard queued messages.
	// Next keeps returning them in FIFO order, then returns ok == false once
	// the closed buffer is empty.
	b.Close()
	for {
		message, ok := b.Next()
		if !ok {
			break
		}
		fmt.Printf("  received %q\n", message)
	}

	fmt.Printf("  push after close accepted: %t\n", b.Push("fourth"))
}

func waitForNextValue() {
	fmt.Println("\nblocking read:")

	b := gx.NewBuffer[string](1)
	received := make(chan string, 1)

	go func() {
		// Next blocks while the buffer is empty, making it useful for a
		// consumer goroutine that should wait for work without polling.
		message, ok := b.Next()
		if ok {
			received <- message
		}
	}()

	b.Push("a value from the producer")
	fmt.Printf("  consumer received %q\n", <-received)
	b.Close()
}
