// Map demonstrates concurrent key-value storage with gx.Map.
//
// Run with:
//
//	go run ./examples/map
package main

import (
	"fmt"

	"github.com/shayanderson/gx"
)

func main() {
	sessions := gx.NewMap[string, string]()

	// Set returns true for a new key, making it convenient to distinguish a
	// first registration from an update.
	if sessions.Set("user-123", "active") {
		fmt.Println("registered user-123")
	}
	sessions.Set("user-123", "idle")

	if status, ok := sessions.Get("user-123"); ok {
		fmt.Printf("current status: %s\n", status)
	}

	// GetAndDelete performs a lookup and deletion under one lock. Use it when a
	// value must be consumed exactly once, such as a session.
	if status, ok := sessions.GetAndDelete("user-123"); ok {
		fmt.Printf("consumed session with status: %s\n", status)
	}

	// Delete reports whether it removed an entry, which is useful when cleanup
	// should only happen for a session that still exists.
	if !sessions.Delete("user-123") {
		fmt.Println("session was already removed")
	}
}
