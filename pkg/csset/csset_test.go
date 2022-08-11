package csset

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestCSSetEmptyInit(t *testing.T) {
	set := NewCSSet("")
	assert.Equal(t, "", set.String())
}

func TestCSSetContentsInit(t *testing.T) {
	set := NewCSSet("one,three,two")
	n := struct{}{}
	assert.ObjectsAreEqual(map[string]struct{}{
		"one": n, "two": n, "three": n},
		set.set)
}

func TestCSSetAdd(t *testing.T) {
	set := NewCSSet("")
	set.Add("one")
	assert.Equal(t, "one", set.String())
}

func TestCSSetTwo(t *testing.T) {
	set := NewCSSet("")
	set.Add("one")
	set.Add("two")
	assert.Equal(t, "one,two", set.String())
}

func TestCSSetDel(t *testing.T) {
	set := NewCSSet("one,two")
	set.Del("one")
	assert.Equal(t, "two", set.String())
}

func TestCSSetHas(t *testing.T) {
	set := NewCSSet("one,two")
	assert.True(t, set.Has("one"))
	assert.True(t, set.Has("two"))
	assert.False(t, set.Has("three"))
}

func TestCSSetDelNotExists(t *testing.T) {
	set := NewCSSet("one,two")
	assert.NotPanics(t, func() { set.Del("three") })
}

func TestCSSetLenZero(t *testing.T) {
	set := NewCSSet("")
	assert.Zero(t, set.Size())
}

func TestCSSetLenTwo(t *testing.T) {
	set := NewCSSet("one,two")
	assert.Equal(t, 2, set.Size())
}
