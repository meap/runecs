// Copyright (c) Petr Reichl and affiliates. All rights reserved.
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

package utils

import (
	"fmt"
	"strconv"
	"strings"
)

const (
	// mibPerGB converts gigabytes to mebibytes (AWS uses MiB for ECS memory)
	mibPerGB = 1024
)

// ParseMemory converts a memory string to MiB.
// Accepts plain numbers (treated as MiB) or suffixed values like "1GB", "2gb".
func ParseMemory(value string) (string, error) {
	lower := strings.ToLower(strings.TrimSpace(value))

	if before, ok := strings.CutSuffix(lower, "gb"); ok {
		gb, err := strconv.Atoi(before)
		if err != nil {
			return "", fmt.Errorf("invalid memory value %q: number before 'GB' must be an integer", value)
		}
		return strconv.Itoa(gb * mibPerGB), nil
	}

	if _, err := strconv.Atoi(lower); err != nil {
		return "", fmt.Errorf("invalid memory value %q: must be a number in MiB or use GB suffix (e.g., 512, 1024, 1GB, 2GB)", value)
	}

	return value, nil
}
