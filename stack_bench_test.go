package gx_test

import (
	"testing"

	"github.com/shayanderson/gx"
)

func BenchmarkStackPushTryPop(b *testing.B) {
	stack := gx.NewStack[int](1)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		stack.Push(1)
		stack.TryPop()
	}
}

func BenchmarkStackPushPopChannel(b *testing.B) {
	ch := make(chan int, 1)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		ch <- 1
		<-ch
	}
}

func BenchmarkStackBatch(b *testing.B) {
	const size = 4096

	b.Run("stack", func(b *testing.B) {
		// Start small to include dynamic growth behavior.
		stack := gx.NewStack[int](1)

		b.ReportAllocs()
		b.ResetTimer()

		for b.Loop() {
			for i := range size {
				stack.Push(i)
			}
			for range size {
				stack.TryPop()
			}
		}
	})

	b.Run("channel", func(b *testing.B) {
		ch := make(chan int, size)

		b.ReportAllocs()
		b.ResetTimer()

		for b.Loop() {
			for i := range size {
				ch <- i
			}
			for range size {
				<-ch
			}
		}
	})
}

func BenchmarkStackGrowth(b *testing.B) {
	const size = 1024

	b.Run("stack_initial_1", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for b.Loop() {
			stack := gx.NewStack[int](1)
			for i := range size {
				stack.Push(i)
			}
			for range size {
				stack.TryPop()
			}
		}
	})

	b.Run("channel_size_1024", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for b.Loop() {
			ch := make(chan int, size)
			for i := range size {
				ch <- i
			}
			for range size {
				<-ch
			}
		}
	})
}
