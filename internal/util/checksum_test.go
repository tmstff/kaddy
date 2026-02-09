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

func TestChecksum_sameHash(t *testing.T) {
	m1 := map[string]string{"a": "b"}
	m2 := map[string]string{"a": "b"}
	assert.NotEmpty(t, CheckSumOf(m1))
	assert.Equal(t, CheckSumOf(m1), CheckSumOf(m2))
}

func TestChecksum_differentHash(t *testing.T) {
	m1 := map[string]string{"a": "b"}
	m2 := map[string]string{"c": "d"}
	assert.NotEqual(t, CheckSumOf(m1), CheckSumOf(m2))
}

func TestChecksum_nil(t *testing.T) {
	assert.NotEmpty(t, CheckSumOf(nil))
	assert.Equal(t, CheckSumOf(nil), CheckSumOf(nil))
}
