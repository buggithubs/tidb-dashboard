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

package statement

import (
	"fmt"
	"reflect"
	"strings"

	"github.com/thoas/go-funk"

	"github.com/pingcap/tidb-dashboard/pkg/apiserver/utils"
)

// incoming configuration field should have the gorm tag `column` used to specify global variables
// sql will be built like this,
// struct { FieldName `gorm:"column:some_global_var"` } -> @@GLOBAL.some_global_var AS some_global_var
func buildGlobalConfigProjectionSelectSQL(config interface{}) string {
	str := buildStringByStructField(config, func(f reflect.StructField) (string, bool) {
		gormTag, ok := f.Tag.Lookup("gorm")
		if !ok {
			return "", false
		}
		column := utils.GetGormColumnName(gormTag)
		return fmt.Sprintf("@@GLOBAL.%s AS %s", column, column), true
	}, ", ")
	return "SELECT " + str // #nosec
}

// sql will be built like this,
// struct { FieldName `gorm:"column:some_global_var"` } -> @@GLOBAL.some_global_var = @FieldName
// `allowedFields` means only allowed fields can be kept in built SQL.
func buildGlobalConfigNamedArgsUpdateSQL(config interface{}, allowedFields ...string) string {
	str := buildStringByStructField(config, func(f reflect.StructField) (string, bool) {
		// extract fields on demand
		if len(allowedFields) != 0 && !funk.ContainsString(allowedFields, f.Name) {
			return "", false
		}

		gormTag, ok := f.Tag.Lookup("gorm")
		if !ok {
			return "", false
		}
		column := utils.GetGormColumnName(gormTag)
		return fmt.Sprintf("@@GLOBAL.%s = @%s", column, f.Name), true
	}, ", ")
	return "SET " + str // #nosec
}

func buildStringByStructField(i interface{}, buildFunc func(f reflect.StructField) (string, bool), sep string) string {
	var t reflect.Type
	if reflect.ValueOf(i).Kind() == reflect.Ptr {
		t = reflect.TypeOf(i).Elem()
	} else {
		t = reflect.TypeOf(i)
	}

	strs := []string{}
	fNum := t.NumField()
	for i := 0; i < fNum; i++ {
		str, ok := buildFunc(t.Field(i))
		if !ok {
			continue
		}
		strs = append(strs, str)
	}
	return strings.Join(strs, sep)
}
