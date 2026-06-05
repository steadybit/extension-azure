/*
 * Copyright 2025 steadybit GmbH. All rights reserved.
 */

package common

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetStringValue(t *testing.T) {
	v := "hi"
	assert.Equal(t, "hi", GetStringValue(&v))
	assert.Equal(t, "", GetStringValue(nil))
}

func TestGetMapValue(t *testing.T) {
	t.Run("map value at key", func(t *testing.T) {
		m := map[string]any{"props": map[string]any{"x": 1}}
		got := GetMapValue(m, "props")
		assert.Equal(t, 1, got["x"])
	})

	t.Run("first element of []any when it's a map", func(t *testing.T) {
		m := map[string]any{"props": []any{map[string]any{"y": 2}, "ignored"}}
		got := GetMapValue(m, "props")
		assert.Equal(t, 2, got["y"])
	})

	t.Run("absent key returns empty map", func(t *testing.T) {
		assert.Equal(t, map[string]any{}, GetMapValue(map[string]any{}, "missing"))
	})

	t.Run("scalar value returns empty map", func(t *testing.T) {
		assert.Equal(t, map[string]any{}, GetMapValue(map[string]any{"k": "scalar"}, "k"))
	})

	t.Run("empty slice returns empty map", func(t *testing.T) {
		assert.Equal(t, map[string]any{}, GetMapValue(map[string]any{"k": []any{}}, "k"))
	})
}

func TestAddAttribute(t *testing.T) {
	a := map[string][]string{}
	AddAttribute(a, "key", "v1")
	AddAttribute(a, "key", "v2")
	assert.Equal(t, []string{"v1", "v2"}, a["key"])
}
