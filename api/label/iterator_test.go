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

package label_test

import (
	"fmt"
	"testing"

	"github.com/Ch1f/otel/api/kv"

	"github.com/stretchr/testify/require"

	"github.com/Ch1f/otel/api/label"
)

func TestIterator(t *testing.T) {
	one := kv.String("one", "1")
	two := kv.Int("two", 2)
	lbl := label.NewSet(one, two)
	iter := lbl.Iter()
	require.Equal(t, 2, iter.Len())

	require.True(t, iter.Next())
	require.Equal(t, one, iter.Label())
	idx, attr := iter.IndexedLabel()
	require.Equal(t, 0, idx)
	require.Equal(t, one, attr)
	require.Equal(t, 2, iter.Len())

	require.True(t, iter.Next())
	require.Equal(t, two, iter.Label())
	idx, attr = iter.IndexedLabel()
	require.Equal(t, 1, idx)
	require.Equal(t, two, attr)
	require.Equal(t, 2, iter.Len())

	require.False(t, iter.Next())
	require.Equal(t, 2, iter.Len())
}

func TestEmptyIterator(t *testing.T) {
	lbl := label.NewSet()
	iter := lbl.Iter()
	require.Equal(t, 0, iter.Len())
	require.False(t, iter.Next())
}

func TestMergedIterator(t *testing.T) {

	type inputs struct {
		name   string
		keys1  []string
		keys2  []string
		expect []string
	}

	makeLabels := func(keys []string, num int) (result []kv.KeyValue) {
		for _, k := range keys {
			result = append(result, kv.Int(k, num))
		}
		return
	}

	for _, input := range []inputs{
		{
			name:   "one overlap",
			keys1:  []string{"A", "B"},
			keys2:  []string{"B", "C"},
			expect: []string{"A/1", "B/1", "C/2"},
		},
		{
			name:   "reversed one overlap",
			keys1:  []string{"B", "A"},
			keys2:  []string{"C", "B"},
			expect: []string{"A/1", "B/1", "C/2"},
		},
		{
			name:   "one empty",
			keys1:  nil,
			keys2:  []string{"C", "B"},
			expect: []string{"B/2", "C/2"},
		},
		{
			name:   "two empty",
			keys1:  []string{"C", "B"},
			keys2:  nil,
			expect: []string{"B/1", "C/1"},
		},
		{
			name:   "no overlap both",
			keys1:  []string{"C"},
			keys2:  []string{"B"},
			expect: []string{"B/2", "C/1"},
		},
		{
			name:   "one empty single two",
			keys1:  nil,
			keys2:  []string{"B"},
			expect: []string{"B/2"},
		},
		{
			name:   "two empty single one",
			keys1:  []string{"A"},
			keys2:  nil,
			expect: []string{"A/1"},
		},
		{
			name:   "all empty",
			keys1:  nil,
			keys2:  nil,
			expect: nil,
		},
		{
			name:   "full overlap",
			keys1:  []string{"A", "B", "C", "D"},
			keys2:  []string{"A", "B", "C", "D"},
			expect: []string{"A/1", "B/1", "C/1", "D/1"},
		},
	} {
		t.Run(input.name, func(t *testing.T) {
			labels1 := makeLabels(input.keys1, 1)
			labels2 := makeLabels(input.keys2, 2)

			set1 := label.NewSet(labels1...)
			set2 := label.NewSet(labels2...)

			merge := label.NewMergeIterator(&set1, &set2)

			var result []string

			for merge.Next() {
				label := merge.Label()
				result = append(result, fmt.Sprint(label.Key, "/", label.Value.Emit()))
			}

			require.Equal(t, input.expect, result)
		})
	}
}
