// Copyright 2021 PingCAP, Inc.
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

package tidbproto

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDecodeBytes(t *testing.T) {
	key := "abcdefghijklmnopqrstuvwxyz"
	for i := 0; i < len(key); i++ {
		_, k, err := decodeBytes(encodeBytes([]byte(key[:i])), nil)
		assert.Nil(t, err)
		assert.Equal(t, string(k), key[:i])
	}
}

func TestTiDBInfo(t *testing.T) {
	buf := new(KeyInfoBuffer)

	// no encode
	_, err := buf.DecodeKey([]byte("t\x80\x00\x00\x00\x00\x00\x00\xff"))
	assert.NotNil(t, err)

	testcases := []struct {
		Key            string
		IsMeta         bool
		TableID        int64
		IsCommonHandle bool
		RowID          int64
		IndexID        int64
	}{
		{
			"T\x00\x00\x00\x00\x00\x00\x00\xff",
			false,
			0,
			false,
			0,
			0,
		},
		{
			"t\x80\x00\x00\x00\x00\x00\xff",
			false,
			0,
			false,
			0,
			0,
		},
		{
			"t\x80\x00\x00\x00\x00\x00\x00\xff",
			false,
			0xff,
			false,
			0,
			0,
		},
		{
			"t\x80\x00\x00\x00\x00\x00\x00\xff_i\x01\x02",
			false,
			0xff,
			false,
			0,
			0,
		},
		{
			"t\x80\x00\x00\x00\x00\x00\x00\xff_i\x80\x00\x00\x00\x00\x00\x00\x02",
			false,
			0xff,
			false,
			0,
			2,
		},
		{
			"t\x80\x00\x00\x00\x00\x00\x00\xff_r\x80\x00\x00\x00\x00\x00\x00\x02",
			false,
			0xff,
			false,
			2,
			0,
		},
		{
			"t\x80\x00\x00\x00\x00\x00\x00\xff_r\x03\x80\x00\x00\x00\x00\x02\r\xaf\x03\x80\x00\x00\x00\x00\x00\x00\x03\x03\x80\x00\x00\x00\x00\x00\b%",
			false,
			0xff,
			true,
			0,
			0,
		},
	}

	for _, testcase := range testcases {
		key := encodeBytes([]byte(testcase.Key))
		_, err := buf.DecodeKey(key)
		assert.Nil(t, err)
		isMeta, tableID := buf.MetaOrTable()
		assert.Equal(t, testcase.IsMeta, isMeta)
		assert.Equal(t, testcase.TableID, tableID)
		isCommonHandle, rowID := buf.RowInfo()
		assert.Equal(t, testcase.IsCommonHandle, isCommonHandle)
		assert.Equal(t, testcase.RowID, rowID)
		indexID := buf.IndexInfo()
		assert.Equal(t, testcase.IndexID, indexID)
	}
}
