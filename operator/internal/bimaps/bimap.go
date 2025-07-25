// /*
// Copyright 2025 The Grove Authors.
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
// */

package bimaps

// BiMap provides a bidirectional mapping between keys and values.
// Both K and V must be comparable types.
type BiMap[K, V comparable] struct {
	forward  map[K]V // K → V mapping
	backward map[V]K // V → K mapping
}

// New creates a new empty BiMap.
func New[K, V comparable]() *BiMap[K, V] {
	return &BiMap[K, V]{
		forward:  make(map[K]V),
		backward: make(map[V]K),
	}
}

// Set establishes a bidirectional mapping between key and value.
// If the key or value already exists, the old mappings are removed first.
func (b *BiMap[K, V]) Set(key K, value V) {
	// Remove existing mappings if they exist
	if existingValue, exists := b.forward[key]; exists {
		delete(b.backward, existingValue)
	}
	if existingKey, exists := b.backward[value]; exists {
		delete(b.forward, existingKey)
	}

	// Set new bidirectional mapping
	b.forward[key] = value
	b.backward[value] = key
}

// GetByKey returns the value associated with the given key.
func (b *BiMap[K, V]) GetByKey(key K) (V, bool) {
	value, exists := b.forward[key]
	return value, exists
}

// HasValue returns true if the given value exists in the map.
func (b *BiMap[K, V]) HasValue(value V) bool {
	_, exists := b.backward[value]
	return exists
}

// Values returns a slice containing all values in the map.
func (b *BiMap[K, V]) Values() []V {
	values := make([]V, 0, len(b.forward))
	for _, value := range b.forward {
		values = append(values, value)
	}
	return values
}
