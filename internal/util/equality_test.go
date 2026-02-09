/*
Copyright 2025.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

type Example struct {
	S  string
	I  int
	M  map[string]string
	Sl []string
}

type NestedExample struct {
	S string
	N Example
}

var m1 = map[string]string{"a": "b"}
var m2 = map[string]string{"c": "d"}
var sl1 = []string{"a"}
var sl2 = []string{"b"}

func TestNonZeroDeepEqual(t *testing.T) {
	tests := []struct {
		name             string
		desired, current any
		expected         bool
	}{
		{"desired empty, current non-empty", Example{"", 0, nil, nil}, Example{"a", 1, m1, sl1}, true},
		{"desired non-empty, current empty", Example{"a", 1, m1, sl1}, Example{"", 0, nil, nil}, false},
		{"all equal", Example{"a", 1, m1, sl1}, Example{"a", 1, m1, sl1}, true},
		{"string non-equal", Example{"a", 1, m1, sl1}, Example{"b", 1, m1, sl1}, false},
		{"int non-equal", Example{"a", 1, m1, sl1}, Example{"a", 2, m1, sl1}, false},
		{"map non-equal", Example{"a", 1, m1, sl1}, Example{"a", 1, m2, sl1}, false},
		{"slice non-equal", Example{"a", 1, m1, sl1}, Example{"a", 1, m1, sl2}, false},
		{"nested equal, string empty", NestedExample{"", Example{"", 1, m1, sl1}}, NestedExample{"a", Example{"a", 1, m1, sl1}}, false},
		{"nested non-equal", NestedExample{"a", Example{"a", 1, m1, sl1}}, NestedExample{"a", Example{"b", 1, m1, sl1}}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := NonZeroDeepEqual(tt.desired, tt.current)
			assert.Equal(t, tt.expected, result)
		})
	}
}
