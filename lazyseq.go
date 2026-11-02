// Package lazyseq provides lazy sequences: any source of elements (slice,
// map, generator/iterator) can be wrapped into LazySeq[T], lazy chains of
// transformations can be built on top of it, and elements are computed only
// on explicit demand (materialization).
package lazyseq

import (
	"iter"
	"math/rand"
)

// Entry is a key-value pair produced when a map is converted into a lazy
// sequence.
type Entry[K comparable, V any] struct {
	Key   K
	Value V
}

// LazySeq is a lazily wrapable sequence of T.
//
// The source is not executed until Iter (or any materializing method) is
// called. Transformations are immutable: they return new LazySeq instances
// and never consume or mutate the original one.
type LazySeq[T any] struct {
	source func() iter.Seq[T]
}

// EmptySeq returns an empty lazy sequence.
func EmptySeq[T any]() LazySeq[T] {
	return LazySeq[T]{
		source: func() iter.Seq[T] {
			return func(yield func(T) bool) {}
		},
	}
}

// FromSlice returns a lazy sequence backed by the given slice.
func FromSlice[T any](data []T) LazySeq[T] {
	return LazySeq[T]{
		source: func() iter.Seq[T] {
			return func(yield func(T) bool) {
				for _, v := range data {
					if !yield(v) {
						return
					}
				}
			}
		},
	}
}

// FromMap returns a lazy sequence of key-value entries backed by the given
// map. Iteration order over Go maps is not deterministic — this is noted in
// the documentation.
func FromMap[K comparable, V any](m map[K]V) LazySeq[Entry[K, V]] {
	return LazySeq[Entry[K, V]]{
		source: func() iter.Seq[Entry[K, V]] {
			return func(yield func(Entry[K, V]) bool) {
				for k, v := range m {
					if !yield(Entry[K, V]{Key: k, Value: v}) {
						return
					}
				}
			}
		},
	}
}

// FromIter wraps a user-provided generator (possibly infinite) into a lazy
// sequence. The generator is not executed until the sequence is consumed.
func FromIter[T any](it iter.Seq[T]) LazySeq[T] {
	return LazySeq[T]{
		source: func() iter.Seq[T] { return it },
	}
}

// FromIter2 returns a lazy sequence of key-value entries backed by the given
// user-provided key-value generator (possibly infinite). The generator is not
// executed until the sequence is consumed.
func FromIter2[K comparable, V any](it iter.Seq2[K, V]) LazySeq[Entry[K, V]] {
	return LazySeq[Entry[K, V]]{
		source: func() iter.Seq[Entry[K, V]] {
			return func(yield func(Entry[K, V]) bool) {
				it(func(k K, v V) bool {
					return yield(Entry[K, V]{Key: k, Value: v})
				})
			}
		},
	}
}

// Iter returns the stdlib iterator of the sequence, usable with all the
// capabilities of iter/range (Go 1.23+). Can be used with `for range`.
func (s LazySeq[T]) Iter() iter.Seq[T] {
	return s.source()
}

// Filter returns a new lazy sequence that yields only the elements for which
// pred returns true. The source is not executed until materialization. The
// original sequence is not consumed.
func (s LazySeq[T]) Filter(pred func(T) bool) LazySeq[T] {
	return LazySeq[T]{
		source: func() iter.Seq[T] {
			return func(yield func(T) bool) {
				for v := range s.Iter() {
					if pred(v) && !yield(v) {
						return
					}
				}
			}
		},
	}
}

// Take returns a new lazy sequence that yields at most n elements. It is
// short-circuiting: the source is not consumed past n elements. It panics if
// n is negative.
func (s LazySeq[T]) Take(n int) LazySeq[T] {
	if n < 0 {
		panic("lazyseq: negative length requested")
	}
	if n == 0 {
		return EmptySeq[T]()
	}
	return LazySeq[T]{
		source: func() iter.Seq[T] {
			return func(yield func(T) bool) {
				count := 0
				for v := range s.Iter() {
					if !yield(v) {
						return
					}
					count++
					if count >= n {
						return
					}
				}
			}
		},
	}
}

// TakeWhile returns a new lazy sequence that yields elements of s for which
// pred returns true and stops at the first element for which pred returns
// false. The source is not consumed beyond the point where the predicate
// fails; if pred is false on the first element the result is empty.
func (s LazySeq[T]) TakeWhile(pred func(T) bool) LazySeq[T] {
	return LazySeq[T]{
		source: func() iter.Seq[T] {
			return func(yield func(T) bool) {
				for v := range s.Iter() {
					if !pred(v) {
						return
					}
					if !yield(v) {
						return
					}
				}
			}
		},
	}
}

// DropWhile returns a new lazy sequence that skips elements of s for which
// pred returns true and keeps the rest. Elements after the first one for which
// pred returns false are passed through. The source is not consumed before
// that point; if all elements satisfy pred the result is empty.
func (s LazySeq[T]) DropWhile(pred func(T) bool) LazySeq[T] {
	return LazySeq[T]{
		source: func() iter.Seq[T] {
			return func(yield func(T) bool) {
				skipping := true
				for v := range s.Iter() {
					if skipping {
						if pred(v) {
							continue
						}
						skipping = false
					}
					if !yield(v) {
						return
					}
				}
			}
		},
	}
}

// Drop returns a new lazy sequence that skips the first n elements. It is
// lazy: skipped elements are not passed to the consumer. It panics if n is
// negative.
func (s LazySeq[T]) Drop(n int) LazySeq[T] {
	if n < 0 {
		panic("lazyseq: negative length requested")
	}
	if n == 0 {
		return s
	}
	return LazySeq[T]{
		source: func() iter.Seq[T] {
			return func(yield func(T) bool) {
				skipped := 0
				for v := range s.Iter() {
					if skipped < n {
						skipped++
						continue
					}
					if !yield(v) {
						return
					}
				}
			}
		},
	}
}

// ForEach consumes the sequence, invoking f for each element. It is a
// materialization that does not return a value.
func (s LazySeq[T]) ForEach(f func(T)) {
	for v := range s.Iter() {
		f(v)
	}
}

// Contains reports whether the sequence contains at least one element equal
// to v.
func (s LazySeq[T]) Contains(v T) bool {
	for x := range s.Iter() {
		if any(x) == any(v) {
			return true
		}
	}
	return false
}

// First returns the first element of the sequence and true, or the zero
// value and false if the sequence is empty. It is short-circuiting: the source
// is not consumed beyond the first element.
func (s LazySeq[T]) First() (T, bool) {
	for v := range s.Iter() {
		return v, true
	}
	var zero T
	return zero, false
}

// Len returns the number of elements in the sequence. For sources without
// random access (generators) it is computed by enumeration.
func (s LazySeq[T]) Len() int {
	n := 0
	s.ForEach(func(T) { n++ })
	return n
}

// ToSlice materializes the sequence into a slice.
func (s LazySeq[T]) ToSlice() []T {
	out := make([]T, 0)
	s.ForEach(func(v T) { out = append(out, v) })
	return out
}

// Map returns a new lazy sequence that transforms each element of s with f.
// The source is not executed until materialization.
func (s LazySeq[T]) Map[U any](f func(T) U) LazySeq[U] {
	return LazySeq[U]{
		source: func() iter.Seq[U] {
			return func(yield func(U) bool) {
				for v := range s.Iter() {
					if !yield(f(v)) {
						return
					}
				}
			}
		},
	}
}

// FilterMap combines filter and map into a single step: it yields the result
// of f for elements where f's ok flag is true. The source is not executed
// until materialization.
func (s LazySeq[T]) FilterMap[U any](f func(T) (U, bool)) LazySeq[U] {
	return LazySeq[U]{
		source: func() iter.Seq[U] {
			return func(yield func(U) bool) {
				for v := range s.Iter() {
					u, ok := f(v)
					if ok && !yield(u) {
						return
					}
				}
			}
		},
	}
}

// FlatMap returns a new lazy sequence that flattens the sequences produced
// by f for each element of the source. Inner sequences are lazy too.
func (s LazySeq[T]) FlatMap[U any](f func(T) LazySeq[U]) LazySeq[U] {
	return LazySeq[U]{
		source: func() iter.Seq[U] {
			return func(yield func(U) bool) {
				for v := range s.Iter() {
					inner := f(v)
					stopped := false
					inner.Iter()(func(u U) bool {
						if stopped {
							return false
						}
						if !yield(u) {
							stopped = true
							return false
						}
						return true
					})
					if stopped {
						return
					}
				}
			}
		},
	}
}

// ToMap materializes the sequence into a map built with the given key and
// value functions. On key collisions the last written key wins (the semantics
// of a regular Go map).
func (s LazySeq[T]) ToMap[K comparable, V any](key func(T) K, value func(T) V) map[K]V {
	out := make(map[K]V)
	s.ForEach(func(v T) { out[key(v)] = value(v) })
	return out
}

// Reduce applies f to the first two elements of the sequence and then folds
// the accumulated value with the next element until the sequence is
// exhausted. It returns the final accumulated value; an empty sequence yields
// the zero value of T.
func (s LazySeq[T]) Reduce(f func(T, T) T) T {
	var acc T
	first := true
	s.ForEach(func(v T) {
		if first {
			acc = v
			first = false
			return
		}
		acc = f(acc, v)
	})
	return acc
}

// ReduceRight applies f to the sequence folding from the right: it starts
// with the last element and combines the accumulated value with each
// preceding element. It returns the final accumulated value; an empty sequence
// yields the zero value of T.
func (s LazySeq[T]) ReduceRight(f func(T, T) T) T {
	data := s.ToSlice()
	var acc T
	first := true
	for i := len(data) - 1; i >= 0; i-- {
		if first {
			acc = data[i]
			first = false
			continue
		}
		acc = f(acc, data[i])
	}
	return acc
}

// Uniq returns a new lazy sequence with the elements of the sequence without
// duplicates, keeping the first occurrence of each. The order of first
// occurrences is preserved. The source is not executed until materialization.
func (s LazySeq[T]) Uniq() LazySeq[T] {
	seen := make(map[any]struct{})
	return LazySeq[T]{
		source: func() iter.Seq[T] {
			return func(yield func(T) bool) {
				for v := range s.Iter() {
					if _, ok := seen[any(v)]; ok {
						continue
					}
					seen[any(v)] = struct{}{}
					if !yield(v) {
						return
					}
				}
			}
		},
	}
}

// UniqBy returns a new lazy sequence with the elements of the sequence whose
// keys returned by key are unique, keeping the first occurrence of each key.
// The source is not executed until materialization.
func (s LazySeq[T]) UniqBy[V comparable](key func(T) V) LazySeq[T] {
	seen := make(map[V]struct{})
	return LazySeq[T]{
		source: func() iter.Seq[T] {
			return func(yield func(T) bool) {
				for v := range s.Iter() {
					k := key(v)
					if _, ok := seen[k]; ok {
						continue
					}
					seen[k] = struct{}{}
					if !yield(v) {
						return
					}
				}
			}
		},
	}
}

// IsUniq reports whether the sequence contains no duplicates.
func (s LazySeq[T]) IsUniq() bool {
	seen := make(map[any]struct{})
	for _, v := range s.ToSlice() {
		if _, ok := seen[any(v)]; ok {
			return false
		}
		seen[any(v)] = struct{}{}
	}
	return true
}

// IsUniqBy reports whether all elements of the sequence have distinct keys
// returned by key.
func (s LazySeq[T]) IsUniqBy[V comparable](key func(T) V) bool {
	seen := make(map[V]struct{})
	for _, v := range s.ToSlice() {
		k := key(v)
		if _, ok := seen[k]; ok {
			return false
		}
		seen[k] = struct{}{}
	}
	return true
}

// Chunk splits the sequence into consecutive chunks of size elements. The
// last chunk may be shorter than size. If size is not positive, no chunks are
// produced. Each chunk is returned as a lazy sequence.
func (s LazySeq[T]) Chunk(size int) []LazySeq[T] {
	if size <= 0 {
		return nil
	}
	data := s.ToSlice()
	if len(data) == 0 {
		return nil
	}
	var out []LazySeq[T]
	for start := 0; start < len(data); start += size {
		end := start + size
		if end > len(data) {
			end = len(data)
		}
		out = append(out, FromSlice(append([]T(nil), data[start:end]...)))
	}
	return out
}

// Window returns all consecutive sub-slices of the sequence with exactly size
// elements. If size is not positive or greater than the length of the sequence,
// no windows are produced. Each window is returned as a lazy sequence.
func (s LazySeq[T]) Window(size int) []LazySeq[T] {
	data := s.ToSlice()
	if size <= 0 || size > len(data) {
		return nil
	}
	var out []LazySeq[T]
	for start := 0; start+size <= len(data); start++ {
		out = append(out, FromSlice(append([]T(nil), data[start:start+size]...)))
	}
	return out
}

// Reverse returns a new lazy sequence with the elements of the sequence in
// reverse order. Since reversing requires knowing all elements up front, it is
// materialized on demand.
func (s LazySeq[T]) Reverse() LazySeq[T] {
	return LazySeq[T]{
		source: func() iter.Seq[T] {
			data := s.ToSlice()
			out := make([]T, len(data))
			for i, v := range data {
				out[len(data)-1-i] = v
			}
			return func(yield func(T) bool) {
				for _, v := range out {
					if !yield(v) {
						return
					}
				}
			}
		},
	}
}

// Shuffle returns a new lazy sequence with the elements of the sequence in a
// randomly shuffled order. Since shuffling requires knowing all elements up
// front, it is materialized on demand.
func (s LazySeq[T]) Shuffle() LazySeq[T] {
	out := s.ToSlice()
	for i := len(out) - 1; i > 0; i-- {
		j := rand.Intn(i + 1)
		out[i], out[j] = out[j], out[i]
	}
	return FromSlice(out)
}

// Concat returns a new lazy sequence with the elements of the sequence
// followed by the elements of the given lazy sequences.
func (s LazySeq[T]) Concat(seqs ...LazySeq[T]) LazySeq[T] {
	return LazySeq[T]{
		source: func() iter.Seq[T] {
			return func(yield func(T) bool) {
				for v := range s.Iter() {
					if !yield(v) {
						return
					}
				}
				for _, seq := range seqs {
					for v := range seq.Iter() {
						if !yield(v) {
							return
						}
					}
				}
			}
		},
	}
}

// IndexOf returns the index of the first occurrence of v in the sequence, or
// -1 if the sequence does not contain v. Elements are compared with ==. It is
// short-circuiting: the source is not consumed past the first match.
func (s LazySeq[T]) IndexOf(v T) int {
	index := 0
	for x := range s.Iter() {
		if any(x) == any(v) {
			return index
		}
		index++
	}
	return -1
}

// LastIndexOf returns the index of the last occurrence of v in the sequence,
// or -1 if the sequence does not contain v. Elements are compared with ==.
func (s LazySeq[T]) LastIndexOf(v T) int {
	index := -1
	counter := 0
	for x := range s.Iter() {
		if any(x) == any(v) {
			index = counter
		}
		counter++
	}
	return index
}
