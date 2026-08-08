package gx_test

import (
	"slices"
	"testing"

	"github.com/shayanderson/gx"
	"github.com/shayanderson/gx/test"
)

func TestNewSet(t *testing.T) {
	s := gx.NewSet("a", "b", "a")

	test.Equal(t, 2, s.Len())
	test.True(t, s.Has("a"))
	test.True(t, s.Has("b"))
	test.False(t, s.Has("c"))
}

func TestSetAdd(t *testing.T) {
	s := gx.NewSet[int]()

	test.True(t, s.Add(1))
	test.False(t, s.Add(1))
	test.True(t, s.Has(1))
	test.Equal(t, 1, s.Len())
}

func TestSetRemove(t *testing.T) {
	s := gx.NewSet(1, 2)

	test.True(t, s.Remove(1))
	test.False(t, s.Remove(1))
	test.False(t, s.Has(1))
	test.True(t, s.Has(2))
	test.Equal(t, 1, s.Len())
}

func TestSetClear(t *testing.T) {
	s := gx.NewSet(1, 2, 3)

	s.Clear()

	test.Equal(t, 0, s.Len())
	test.False(t, s.Has(1))
	test.False(t, s.Has(2))
	test.False(t, s.Has(3))
}

func TestSetValues(t *testing.T) {
	s := gx.NewSet("b", "a", "c", "a")

	values := s.Values()
	slices.Sort(values)

	test.Equal(t, []string{"a", "b", "c"}, values)
}
