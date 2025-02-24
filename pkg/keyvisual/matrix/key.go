// Copyright 2019 PingCAP, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// See the License for the specific language governing permissions and
// limitations under the License.

package matrix

import (
	"reflect"
	"sync"
	"unsafe"
)

// KeyMap is used for string intern
type KeyMap struct {
	sync.RWMutex
	sync.Map
}

// SaveKey interns a string.
func (km *KeyMap) SaveKey(key *string) {
	uniqueKey, _ := km.LoadOrStore(*key, *key)
	*key = uniqueKey.(string)
}

// SaveKeys interns all strings without using mutex.
func (km *KeyMap) SaveKeys(keys []string) {
	for i, key := range keys {
		uniqueKey, _ := km.LoadOrStore(key, key)
		keys[i] = uniqueKey.(string)
	}
}

func equal(keyA, keyB string) bool {
	pA := (*reflect.StringHeader)(unsafe.Pointer(&keyA)) // #nosec
	pB := (*reflect.StringHeader)(unsafe.Pointer(&keyB)) // #nosec
	return pA.Data == pB.Data && pA.Len == pB.Len
}
