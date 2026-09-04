package gx_test

import (
	"slices"
	"testing"

	"github.com/shayanderson/gx"
	"github.com/shayanderson/gx/test"
)

func TestNewSet(t *testing.T) {
	t.Parallel()

	s := gx.NewSet("a", "b", "a")

	test.Equal(t, 2, s.Len())
	test.True(t, s.Has("a"))
	test.True(t, s.Has("b"))
	test.False(t, s.Has("c"))
}

func TestSetAdd(t *testing.T) {
	t.Parallel()

	s := gx.NewSet[int]()

	test.True(t, s.Add(1))
	test.False(t, s.Add(1))
	test.True(t, s.Has(1))
	test.Equal(t, 1, s.Len())
}

func TestSetRemove(t *testing.T) {
	t.Parallel()

	s := gx.NewSet(1, 2)

	test.True(t, s.Remove(1))
	test.False(t, s.Remove(1))
	test.False(t, s.Has(1))
	test.True(t, s.Has(2))
	test.Equal(t, 1, s.Len())
}

func TestSetClear(t *testing.T) {
	t.Parallel()

	s := gx.NewSet(1, 2, 3)

	s.Clear()

	test.Equal(t, 0, s.Len())
	test.False(t, s.Has(1))
	test.False(t, s.Has(2))
	test.False(t, s.Has(3))
}

func TestSetClone(t *testing.T) {
	t.Parallel()

	s := gx.NewSet("a", "b")

	clone := s.Clone()
	test.Equal(t, 2, clone.Len())

	test.True(t, clone.Remove("a"))
	test.True(t, clone.Add("c"))

	test.True(t, s.Has("a"))
	test.True(t, s.Has("b"))
	test.False(t, s.Has("c"))

	test.True(t, s.Add("d"))
	test.False(t, clone.Has("d"))
}

func TestSetCloneShallow(t *testing.T) {
	t.Parallel()

	value := 1

	s := gx.NewSet(&value)
	clone := s.Clone()

	values := clone.Values()

	test.Equal(t, &value, values[0])

	*values[0] = 2

	original := s.Values()
	test.Equal(t, 2, *original[0])
}

func TestSetValues(t *testing.T) {
	t.Parallel()

	s := gx.NewSet("b", "a", "c", "a")

	values := s.Values()
	slices.Sort(values)

	test.Equal(t, []string{"a", "b", "c"}, values)
}
