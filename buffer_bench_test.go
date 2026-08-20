package gx_test

import (
	"testing"

	"github.com/shayanderson/gx"
)

func BenchmarkBufferPushTryNext(b *testing.B) {
	buf := gx.NewBuffer[int](1)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		buf.Push(1)
		buf.TryNext()
	}
}

func BenchmarkBufferChannelPushReceive(b *testing.B) {
	ch := make(chan int, 1)

	b.ReportAllocs()
	b.ResetTimer()

	for b.Loop() {
		ch <- 1
		<-ch
	}
}

func BenchmarkBufferBatch(b *testing.B) {
	const size = 4096

	b.Run("buffer", func(b *testing.B) {
		// Start small to include dynamic growth behavior.
		buf := gx.NewBuffer[int](1)

		b.ReportAllocs()
		b.ResetTimer()

		for b.Loop() {
			for i := range size {
				buf.Push(i)
			}
			for range size {
				buf.TryNext()
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

func BenchmarkBufferGrowth(b *testing.B) {
	const size = 1024

	b.Run("buffer_initial_1", func(b *testing.B) {
		b.ReportAllocs()
		b.ResetTimer()

		for b.Loop() {
			buf := gx.NewBuffer[int](1)
			for i := range size {
				buf.Push(i)
			}
			for range size {
				buf.TryNext()
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
