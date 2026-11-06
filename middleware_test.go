package nethttplibrary

import (
	"testing"
	"unsafe"

	"github.com/stretchr/testify/assert"
)

type arrayOption [3]int

func TestIsNil(t *testing.T) {
	assert.True(t, isNil(nil))

	var ptr *string
	assert.True(t, isNil(ptr))

	var m map[string]string
	assert.True(t, isNil(m))

	var fn func()
	assert.True(t, isNil(fn))

	var ch chan int
	assert.True(t, isNil(ch))

	var s []string
	assert.True(t, isNil(s))

	var iface any
	assert.True(t, isNil(iface))

	var uptr unsafe.Pointer
	assert.True(t, isNil(uptr))

	// Arrays cannot be nil and must not panic
	var arr [3]int
	assert.False(t, isNil(arr))
	assert.False(t, isNil(arrayOption{1, 2, 3}))
	assert.False(t, isNil([0]int{}))

	// Non-nil values
	strVal := "hello"
	assert.False(t, isNil(&strVal))
	assert.False(t, isNil("hello"))
	assert.False(t, isNil(123))
	assert.False(t, isNil(map[string]string{"k": "v"}))
	assert.False(t, isNil([]string{"a"}))
	assert.False(t, isNil(func() {}))
}

