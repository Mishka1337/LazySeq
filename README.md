# LazySeq

**Lazy sequences in Go** — a library for working with `LazySeq[T]`.

The library provides lazy sequences: any source of elements (slice, map, generator/iterator) can be wrapped into `LazySeq[T]`. Lazy transformation chains (`Filter`, `Map`, `Take`, etc.) can be built on top of it **without executing the source**. Elements are computed only on explicit demand (materialization).

## Features

- **Sources:**
  - `EmptySeq[T]()` — empty sequence;
  - `FromSlice[T](data []T)` — slice → lazy sequence;
  - `FromMap[K, V](m map[K]V)` — map → sequence of `Entry` pairs;
  - `FromIter[T](it iter.Seq[T])` — user generator (possibly infinite). Any source (slice, map, generator) is just an `iter.Seq[T]` passed to this constructor;
  - `FromIter2[K, V](it iter.Seq2[K, V])` — user key-value generator (possibly infinite) → lazy sequence of `Entry` pairs.
- **Transformations** (immutable, no source execution):
  - `Filter(pred func(T) bool) LazySeq[T]`;
  - `Map(f func(T) U) LazySeq[U]`;
  - `FilterMap(f func(T) (U, bool)) LazySeq[U]`;
  - `FlatMap(f func(T) LazySeq[U]) LazySeq[U]`;
  - `Take(n int) LazySeq[T]` – lazy limit, short-circuit;
  - `Drop(n int) LazySeq[T]` – lazy skip, panics on `n < 0`;
  - `TakeWhile(pred func(T) bool) LazySeq[T]` – lazy take, stops at first `false`;
  - `DropWhile(pred func(T) bool) LazySeq[T]` – lazy drop, skips while `pred` is true.
- **Helpers:**
  - `Uniq() LazySeq[T]` – unique elements, first occurrence kept;
  - `UniqBy(key func(T) V) LazySeq[T]` – unique elements by key, first occurrence kept;
  - `IsUniq() bool` – whether the sequence has no duplicates;
  - `IsUniqBy(key func(T) V) bool` – whether all elements have distinct keys;
  - `Chunk(size int) []LazySeq[T]` – consecutive chunks of size `size`, last chunk may be shorter; `nil` on non-positive `size` or empty sequence;
  - `Window(size int) []LazySeq[T]` – all consecutive windows of exactly `size` elements; `nil` if `size` is not positive or greater than the sequence length;
  - `Reverse() LazySeq[T]` – reversed sequence;
  - `Shuffle() LazySeq[T]` – randomly shuffled sequence;
  - `Concat(seqs ...LazySeq[T]) LazySeq[T]` – sequence elements followed by the elements of the given sequences;
  - `Reduce(f func(T, T) T) T` – left fold over the sequence;
  - `ReduceRight(f func(T, T) T) T` – right fold over the sequence.
- **Materialization:**
  - `ToSlice() []T`;
  - `ToMap(key func(T) K, value func(T) V) map[K]V`;
  - `Iter() iter.Seq[T]` – stdlib iterator, `for v := range seq.Iter()`;
  - `ForEach(f func(T))`, `Contains(v T) bool`, `First() (T, bool)`, `Len() int`.

## Example

```go
nums := lazyseq.FromSlice([]int{1, 2, 3, 4, 5, 6})

evens := nums.
    Filter(func(x int) bool { return x%2 == 0 }).
    Map(func(x int) int { return x * 10 })

result := evens.ToSlice() // []int{20, 40, 60}
```

## Installation

```bash
go get github.com/mkostylev/lazyseq
```

## Tests

```bash
go test ./...
```