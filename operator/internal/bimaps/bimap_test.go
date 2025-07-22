package bimaps

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestBiMap_New(t *testing.T) {
	bm := New[string, int]()
	assert.NotNil(t, bm)
	assert.Empty(t, bm.forward)
	assert.Empty(t, bm.backward)
}

func TestBiMap_Set(t *testing.T) {
	bm := New[string, int]()

	// Test basic set
	bm.Set("pod-a", 0)
	value, exists := bm.GetByKey("pod-a")
	assert.True(t, exists)
	assert.Equal(t, 0, value)
	assert.True(t, bm.HasValue(0))
}

func TestBiMap_GetByKey(t *testing.T) {
	bm := New[string, int]()
	bm.Set("pod-a", 0)
	bm.Set("pod-b", 1)

	// Test existing key
	value, exists := bm.GetByKey("pod-a")
	assert.True(t, exists)
	assert.Equal(t, 0, value)

	// Test non-existing key
	_, exists = bm.GetByKey("pod-c")
	assert.False(t, exists)
}

func TestBiMap_HasValue(t *testing.T) {
	bm := New[string, int]()
	bm.Set("pod-a", 0)
	bm.Set("pod-b", 1)

	assert.True(t, bm.HasValue(0))
	assert.True(t, bm.HasValue(1))
	assert.False(t, bm.HasValue(2))
}

func TestBiMap_Values(t *testing.T) {
	bm := New[string, int]()
	bm.Set("pod-a", 0)
	bm.Set("pod-b", 2)
	bm.Set("pod-c", 4)

	values := bm.Values()
	assert.Len(t, values, 3)
	assert.ElementsMatch(t, []int{0, 2, 4}, values)
}

func TestBiMap_SetOverwrite(t *testing.T) {
	bm := New[string, int]()

	// Set initial mapping
	bm.Set("pod-a", 0)
	bm.Set("pod-b", 1)

	// Overwrite key with new value
	bm.Set("pod-a", 2)

	// Old value should not exist
	assert.False(t, bm.HasValue(0))
	// New value should exist
	value, exists := bm.GetByKey("pod-a")
	assert.True(t, exists)
	assert.Equal(t, 2, value)
	assert.True(t, bm.HasValue(2))
}
