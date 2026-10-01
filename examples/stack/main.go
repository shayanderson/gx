// Stack demonstrates an undo history built with gx.Stack.
//
// Run with:
//
//	go run ./examples/stack
package main

import (
	"fmt"

	"github.com/shayanderson/gx"
)

type Edit struct {
	Description string
}

func main() {
	// The initial capacity is only a sizing hint; Stack grows automatically.
	history := gx.NewStack[Edit](2)
	for _, edit := range []Edit{
		{Description: "typed h"},
		{Description: "typed i"},
		{Description: "added !"},
	} {
		history.Push(edit)
	}

	// TryPop is useful for a non-blocking undo command. A stack returns the
	// most recently pushed edit first.
	if edit, ok := history.TryPop(); ok {
		fmt.Printf("undo: %s\n", edit.Description)
	}

	// Close prevents new history entries but preserves entries already pushed.
	// Pop drains them in LIFO order and returns ok == false once the closed
	// stack is empty.
	history.Close()
	for {
		edit, ok := history.Pop()
		if !ok {
			break
		}
		fmt.Printf("undo: %s\n", edit.Description)
	}

	fmt.Printf("push after close accepted: %t\n", history.Push(Edit{Description: "typed again"}))
}
