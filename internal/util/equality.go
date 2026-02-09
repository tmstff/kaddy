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
	"errors"
	"reflect"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
)

func ignoreZeroFields(desired any) cmp.Option {
	t := reflect.TypeOf(desired)
	if t.Kind() == reflect.Ptr {
		t = t.Elem()
	}

	var fields []string
	v := reflect.ValueOf(desired)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	for i := 0; i < t.NumField(); i++ {
		fv := v.Field(i)
		if fv.IsZero() {
			fields = append(fields, t.Field(i).Name)
		}
	}

	return cmpopts.IgnoreFields(desired, fields...)
}

// NonZeroDeepEqual performs a deep equal on two structs, ignoring fields that are zero in "desired" and unexported fields.
// It return "true" when  they are equal this way, or "false" if not.
// An error is returned, when any of the inpt parameters is no struct.
func NonZeroDeepEqual(desired, current any) (bool, error) {
	t := reflect.TypeOf(desired)
	if t.Kind() != reflect.Struct {
		return false, errors.New("desired must be a struct")
	}

	return cmp.Equal(desired, current, cmpopts.IgnoreUnexported(), ignoreZeroFields(desired)), nil
}
