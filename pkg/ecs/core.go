// Copyright 2021 Petr Reichl. All rights reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package ecs

import "errors"

// Generic helper functions for safe slice access
// These functions provide bounds checking to prevent panics when accessing slice elements

// safeGetFirstPtr returns a pointer to the first element of a slice.
// Returns an error if the slice is empty to prevent panic on index access.
func safeGetFirstPtr[T any](slice []T, errorMsg string) (*T, error) {
	if len(slice) == 0 {
		return nil, errors.New(errorMsg)
	}
	return &slice[0], nil
}

// safeGetFirst returns the first element of a slice by value.
// Returns the zero value of type T and an error if the slice is empty.
func safeGetFirst[T any](slice []T, errorMsg string) (T, error) {
	var zero T
	if len(slice) == 0 {
		return zero, errors.New(errorMsg)
	}
	return slice[0], nil
}
