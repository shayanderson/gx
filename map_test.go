package gx_test

import (
	"slices"
	"testing"

	"github.com/shayanderson/gx"
	"github.com/shayanderson/gx/test"
)

func TestNewMap(t *testing.T) {
	m := gx.NewMap[string, int]()

	test.Equal(t, 0, m.Len())
	test.False(t, m.Has("a"))
}

func TestMapSetAndGet(t *testing.T) {
	m := gx.NewMap[string, int]()

	m.Set("a", 1)
	m.Set("b", 2)
	m.Set("a", 3)

	v, ok := m.Get("a")
	test.True(t, ok)
	test.Equal(t, 3, v)

	v, ok = m.Get("b")
	test.True(t, ok)
	test.Equal(t, 2, v)

	v, ok = m.Get("c")
	test.False(t, ok)
	test.Equal(t, 0, v)
	test.Equal(t, 2, m.Len())
}

func TestMapGetOrSet(t *testing.T) {
	m := gx.NewMap[string, int]()

	v, ok := m.GetOrSet("a", 1)
	test.False(t, ok)
	test.Equal(t, 1, v)

	v, ok = m.GetOrSet("a", 2)
	test.True(t, ok)
	test.Equal(t, 1, v)
	test.Equal(t, 1, m.Len())
}

func TestMapRemove(t *testing.T) {
	m := gx.NewMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)

	test.True(t, m.Remove("a"))
	test.False(t, m.Remove("a"))
	test.False(t, m.Has("a"))
	test.True(t, m.Has("b"))
	test.Equal(t, 1, m.Len())
}

func TestMapGetAndRemove(t *testing.T) {
	m := gx.NewMap[string, int]()
	m.Set("a", 1)

	v, ok := m.GetAndRemove("a")
	test.True(t, ok)
	test.Equal(t, 1, v)
	test.False(t, m.Has("a"))

	v, ok = m.GetAndRemove("a")
	test.False(t, ok)
	test.Equal(t, 0, v)
}

func TestMapClear(t *testing.T) {
	m := gx.NewMap[string, int]()
	m.Set("a", 1)
	m.Set("b", 2)

	m.Clear()

	test.Equal(t, 0, m.Len())
	test.False(t, m.Has("a"))
	test.False(t, m.Has("b"))
}

func TestMapKeys(t *testing.T) {
	m := gx.NewMap[string, int]()
	m.Set("b", 2)
	m.Set("a", 1)
	m.Set("c", 3)

	keys := m.Keys()
	slices.Sort(keys)

	test.Equal(t, []string{"a", "b", "c"}, keys)
}

func TestMapValues(t *testing.T) {
	m := gx.NewMap[string, int]()
	m.Set("a", 2)
	m.Set("b", 1)
	m.Set("c", 3)

	values := m.Values()
	slices.Sort(values)

	test.Equal(t, []int{1, 2, 3}, values)
}
