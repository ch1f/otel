// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package kv_test

import (
	"encoding/json"
	"testing"

	"github.com/Ch1f/otel/api/kv"

	"github.com/stretchr/testify/require"

	"github.com/Ch1f/otel/api/kv/value"
)

func TestDefined(t *testing.T) {
	for _, testcase := range []struct {
		name string
		k    kv.Key
		want bool
	}{
		{
			name: "Key.Defined() returns true when len(v.Name) != 0",
			k:    kv.Key("foo"),
			want: true,
		},
		{
			name: "Key.Defined() returns false when len(v.Name) == 0",
			k:    kv.Key(""),
			want: false,
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			//func (k kv.Key) Defined() bool {
			have := testcase.k.Defined()
			if have != testcase.want {
				t.Errorf("Want: %v, but have: %v", testcase.want, have)
			}
		})
	}
}

func TestJSONValue(t *testing.T) {
	var kvs interface{} = [2]kv.KeyValue{
		kv.String("A", "B"),
		kv.Int64("C", 1),
	}

	data, err := json.Marshal(kvs)
	require.NoError(t, err)
	require.Equal(t,
		`[{"Key":"A","Value":{"Type":"STRING","Value":"B"}},{"Key":"C","Value":{"Type":"INT64","Value":1}}]`,
		string(data))
}

func TestEmit(t *testing.T) {
	for _, testcase := range []struct {
		name string
		v    value.Value
		want string
	}{
		{
			name: `test Key.Emit() can emit a string representing self.BOOL`,
			v:    value.Bool(true),
			want: "true",
		},
		{
			name: `test Key.Emit() can emit a string representing self.INT32`,
			v:    value.Int32(42),
			want: "42",
		},
		{
			name: `test Key.Emit() can emit a string representing self.INT64`,
			v:    value.Int64(42),
			want: "42",
		},
		{
			name: `test Key.Emit() can emit a string representing self.UINT32`,
			v:    value.Uint32(42),
			want: "42",
		},
		{
			name: `test Key.Emit() can emit a string representing self.UINT64`,
			v:    value.Uint64(42),
			want: "42",
		},
		{
			name: `test Key.Emit() can emit a string representing self.FLOAT32`,
			v:    value.Float32(42.1),
			want: "42.1",
		},
		{
			name: `test Key.Emit() can emit a string representing self.FLOAT64`,
			v:    value.Float64(42.1),
			want: "42.1",
		},
		{
			name: `test Key.Emit() can emit a string representing self.STRING`,
			v:    value.String("foo"),
			want: "foo",
		},
	} {
		t.Run(testcase.name, func(t *testing.T) {
			//proto: func (v kv.Value) Emit() string {
			have := testcase.v.Emit()
			if have != testcase.want {
				t.Errorf("Want: %s, but have: %s", testcase.want, have)
			}
		})
	}
}

func BenchmarkEmitBool(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n := value.Bool(i%2 == 0)
		_ = n.Emit()
	}
}

func BenchmarkEmitInt64(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n := value.Int64(int64(i))
		_ = n.Emit()
	}
}

func BenchmarkEmitUInt64(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n := value.Uint64(uint64(i))
		_ = n.Emit()
	}
}

func BenchmarkEmitFloat64(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n := value.Float64(float64(i))
		_ = n.Emit()
	}
}

func BenchmarkEmitFloat32(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		n := value.Float32(float32(i))
		_ = n.Emit()
	}
}
