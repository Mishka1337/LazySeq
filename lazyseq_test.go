package lazyseq

import (
	"fmt"
	"iter"
	"testing"
)

// countingSource is a controllable source of ints used to observe how many
// times the source is called and how many elements are pulled from it.
type countingSource struct {
	items  []int
	calls  int
	pulled int
}

func (c *countingSource) Iter() iter.Seq[int] {
	c.calls++
	return func(yield func(int) bool) {
		for _, v := range c.items {
			c.pulled++
			if !yield(v) {
				return
			}
		}
	}
}

func TestEmptySeq(t *testing.T) {
	var s LazySeq[int] = EmptySeq[int]()

	if got := s.Len(); got != 0 {
		t.Errorf("Len() = %d, want 0", got)
	}
	if got := s.ToSlice(); len(got) != 0 {
		t.Errorf("ToSlice() = %v, want empty slice", got)
	}
	if v, ok := s.First(); ok {
		t.Errorf("First() = %v, %v; want zero value and false", v, ok)
	}
	if s.Contains(42) {
		t.Error("Contains() = true for empty sequence, want false")
	}
	s.ForEach(func(int) { t.Error("ForEach must not invoke f on empty sequence") })
}

func TestFromSlice(t *testing.T) {
	seq := FromSlice([]int{1, 2, 3, 4, 5})

	if got := seq.ToSlice(); len(got) != 5 || got[0] != 1 || got[4] != 5 {
		t.Errorf("ToSlice() = %v, want [1 2 3 4 5]", got)
	}
	if got := seq.Len(); got != 5 {
		t.Errorf("Len() = %d, want 5", got)
	}
	if !seq.Contains(3) {
		t.Error("Contains(3) = false, want true")
	}
	if seq.Contains(99) {
		t.Error("Contains(99) = true, want false")
	}
	if v, ok := seq.First(); !ok || v != 1 {
		t.Errorf("First() = %v, %v; want 1, true", v, ok)
	}
}

func TestFromIter2(t *testing.T) {
	m := map[string]int{"a": 1, "b": 2}

	seq := FromIter2(func(yield func(string, int) bool) {
		for k, v := range m {
			if !yield(k, v) {
				return
			}
		}
	})

	if got := seq.ToSlice(); len(got) != 2 {
		t.Fatalf("ToSlice() = %v, want 2 entries", got)
	}
	m2 := seq.ToMap(func(e Entry[string, int]) string { return e.Key }, func(e Entry[string, int]) int { return e.Value })
	if len(m2) != 2 || m2["a"] != 1 || m2["b"] != 2 {
		t.Errorf("ToMap() over FromIter2 = %v, want map[a:1 b:2]", m2)
	}
}

func TestLaziness(t *testing.T) {
	src := &countingSource{items: []int{1, 2, 3}}

	seq := FromIter(func(yield func(int) bool) {
		src.calls++
		for _, v := range src.items {
			src.pulled++
			if !yield(v) {
				return
			}
		}
	}).Map(func(x int) int { return x * 2 }).
		Filter(func(x int) bool { return x > 2 })

	if src.calls != 0 {
		t.Fatalf("source was consumed during construction: %d calls, want 0", src.calls)
	}

	if got := seq.ToSlice(); len(got) != 2 || got[0] != 4 || got[1] != 6 {
		t.Errorf("ToSlice() = %v, want [4 6]", got)
	}
	if src.calls != 1 {
		t.Errorf("source calls = %d, want 1", src.calls)
	}
	if src.pulled != 3 {
		t.Errorf("source pulled %d elements, want 3", src.pulled)
	}
}

func TestTakeShortCircuit(t *testing.T) {
	src := &countingSource{items: []int{5, 6, 7, 8, 9}}

	if got := FromIter(src.Iter()).Take(2).ToSlice(); len(got) != 2 || got[0] != 5 || got[1] != 6 {
		t.Errorf("Take(2).ToSlice() = %v, want [5 6]", got)
	}
	if src.pulled != 2 {
		t.Errorf("source pulled %d elements, want 2 (short-circuit)", src.pulled)
	}
}

func TestTakeZeroAndNegative(t *testing.T) {
	if got := FromSlice([]int{1, 2, 3}).Take(0).ToSlice(); len(got) != 0 {
		t.Errorf("Take(0).ToSlice() = %v, want empty", got)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Take(-1) must panic, did not panic")
		}
	}()
	FromSlice([]int{1, 2, 3}).Take(-1)
}

func TestDrop(t *testing.T) {
	seq := FromSlice([]int{1, 2, 3, 4, 5})

	if got := seq.Drop(2).ToSlice(); len(got) != 3 || got[0] != 3 || got[2] != 5 {
		t.Errorf("Drop(2).ToSlice() = %v, want [3 4 5]", got)
	}
	if got := seq.Drop(0).ToSlice(); len(got) != 5 {
		t.Errorf("Drop(0).ToSlice() = %v, want [1 2 3 4 5]", got)
	}

	defer func() {
		if r := recover(); r == nil {
			t.Error("Drop(-1) must panic, did not panic")
		}
	}()
	seq.Drop(-1)
}

func TestFilterMap(t *testing.T) {
	seq := FromSlice([]int{1, 2, 3, 4, 5})
	f := func(x int) (int, bool) {
		if x%2 != 0 {
			return 0, false
		}
		return x * 2, true
	}

	if got := seq.FilterMap(f).ToSlice(); len(got) != 2 || got[0] != 4 || got[1] != 8 {
		t.Errorf("FilterMap(...).ToSlice() = %v, want [4 8]", got)
	}
}

func TestFlatMap(t *testing.T) {
	seq := FromSlice([]int{1, 2, 3})
	f := func(v int) LazySeq[int] {
		if v%2 != 0 {
			return EmptySeq[int]()
		}
		return FromSlice([]int{v, v * 10})
	}

	if got := seq.FlatMap(f).ToSlice(); len(got) != 2 || got[0] != 2 || got[1] != 20 {
		t.Errorf("FlatMap(...).ToSlice() = %v, want [2 20]", got)
	}
}

func TestToMap(t *testing.T) {
	seq := FromSlice([]int{1, 2, 3})
	m := seq.ToMap(func(i int) string { return "k" }, func(i int) string { return fmt.Sprint(i) })
	if len(m) != 1 {
		t.Fatalf("ToMap() returned %v, want a map with one key", m)
	}
	if v, ok := m["k"]; !ok || v != "3" {
		t.Errorf("ToMap() collision resolution: key %q = %q, want last value %q", "k", v, "3")
	}

	m2 := FromMap(map[string]string{"a": "1"}).ToMap(func(e Entry[string, string]) string { return e.Key }, func(e Entry[string, string]) string { return e.Value })
	if len(m2) != 1 || m2["a"] != "1" {
		t.Errorf("ToMap() over FromMap = %v, want map[a:1]", m2)
	}
}

func TestRangeOverIter(t *testing.T) {
	seq := FromSlice([]int{1, 2, 3})
	i := 0
	for v := range seq.Iter() {
		i++
		if v != i {
			t.Fatalf("range yielded %v, want %d", v, i)
		}
	}
	if i != 3 {
		t.Fatalf("range iterated %d values, want 3", i)
	}
}

func TestTransformationsAreImmutable(t *testing.T) {
	base := FromSlice([]int{1, 2, 3})

	first := base.Filter(func(x int) bool { return x%2 == 1 })
	second := base.Filter(func(x int) bool { return x%2 == 0 })

	if got := first.ToSlice(); len(got) != 2 || got[0] != 1 || got[1] != 3 {
		t.Errorf("first chain = %v, want [1 3]", got)
	}
	if got := second.ToSlice(); len(got) != 1 || got[0] != 2 {
		t.Errorf("second chain = %v, want [2]", got)
	}
}

func TestReduce(t *testing.T) {
	seq := FromSlice([]int{1, 2, 3, 4})

	if got := seq.Reduce(func(acc, v int) int { return acc + v }); got != 10 {
		t.Errorf("Reduce() = %d, want 10", got)
	}
	if got := EmptySeq[int]().Reduce(func(acc, v int) int { return acc + v }); got != 0 {
		t.Errorf("Reduce() over empty sequence = %d, want 0", got)
	}
}

func TestReduceRight(t *testing.T) {
	seq := FromSlice([]int{1, 2, 3})

	// right fold: f(f(f(3, 2), 1)) == 321, left fold would produce 123
	if got := seq.ReduceRight(func(acc, v int) int { return acc*10 + v }); got != 321 {
		t.Errorf("ReduceRight() = %d, want 321", got)
	}
	if got := EmptySeq[int]().ReduceRight(func(acc, v int) int { return acc*10 + v }); got != 0 {
		t.Errorf("ReduceRight() over empty sequence = %d, want 0", got)
	}
}

func TestUniq(t *testing.T) {
	got := FromSlice([]int{1, 2, 2, 3, 1, 3}).Uniq().ToSlice()
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("Uniq() = %v, want [1 2 3]", got)
	}
}

func TestUniqBy(t *testing.T) {
	got := FromSlice([]int{1, 2, 3, 4, 5}).UniqBy(func(i int) int { return i % 2 }).ToSlice()
	if len(got) != 2 || got[0] != 1 || got[1] != 2 {
		t.Errorf("UniqBy() = %v, want [1 2]", got)
	}
}

func TestIsUniq(t *testing.T) {
	if !FromSlice([]int{1, 2, 3}).IsUniq() {
		t.Error("IsUniq([1 2 3]) = false, want true")
	}
	if FromSlice([]int{1, 2, 2}).IsUniq() {
		t.Error("IsUniq([1 2 2]) = true, want false")
	}
}

func TestIsUniqBy(t *testing.T) {
	if !FromSlice([]int{10, 30, 50}).IsUniqBy(func(i int) int { return i }) {
		t.Error("IsUniqBy([10 30 50]) = false, want true")
	}
	if FromSlice([]int{10, 30, 50, 30}).IsUniqBy(func(i int) int { return i }) {
		t.Error("IsUniqBy([10 30 50 30]) = true, want false")
	}
}

func TestChunk(t *testing.T) {
	got := FromSlice([]int{1, 2, 3, 4, 5, 6}).Chunk(2)
	if len(got) != 3 {
		t.Fatalf("Chunk() returned %v chunks, want 3", got)
	}
	if c := got[0].ToSlice(); len(c) != 2 || c[0] != 1 || c[1] != 2 {
		t.Errorf("Chunk()[0] = %v, want [1 2]", c)
	}
	if c := got[2].ToSlice(); len(c) != 2 || c[0] != 5 || c[1] != 6 {
		t.Errorf("Chunk()[2] = %v, want [5 6]", c)
	}

	got = FromSlice([]int{1, 2, 3}).Chunk(2)
	if len(got) != 2 || len(got[1].ToSlice()) != 1 || got[1].ToSlice()[0] != 3 {
		t.Errorf("Chunk([1 2 3], 2) = %v, want [[1 2] [3]]", got)
	}
}

func TestWindow(t *testing.T) {
	got := FromSlice([]int{1, 2, 3, 4}).Window(2)
	if len(got) != 3 {
		t.Fatalf("Window() returned %v windows, want 3", got)
	}
	if w := got[0].ToSlice(); len(w) != 2 || w[0] != 1 || w[1] != 2 {
		t.Errorf("Window()[0] = %v, want [1 2]", w)
	}
	if w := got[2].ToSlice(); len(w) != 2 || w[0] != 3 || w[1] != 4 {
		t.Errorf("Window()[2] = %v, want [3 4]", w)
	}
}

func TestUniqOverGenerator(t *testing.T) {
	src := &countingSource{items: []int{1, 2, 2, 3, 1, 3}}

	seq := FromIter(src.Iter()).Uniq()
	if got := seq.ToSlice(); len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("Uniq() over generator = %v, want [1 2 3]", got)
	}
	if src.calls != 1 || src.pulled != 6 {
		t.Errorf("Uniq() pulled %d elements from %d sources, want 6 from 1", src.pulled, src.calls)
	}
}

func TestChunkEdgeCases(t *testing.T) {
	if got := FromSlice([]int{}).Chunk(2); got != nil {
		t.Errorf("Chunk() over empty sequence = %v, want nil", got)
	}
	if got := FromSlice([]int{1, 2, 3}).Chunk(0); got != nil {
		t.Errorf("Chunk(0) = %v, want nil", got)
	}
	if got := FromSlice([]int{1, 2, 3}).Chunk(-1); got != nil {
		t.Errorf("Chunk(-1) = %v, want nil", got)
	}

	got := FromSlice([]int{1, 2, 3, 4, 5}).Chunk(2)
	if len(got) != 3 || len(got[0].ToSlice()) != 2 || len(got[1].ToSlice()) != 2 || len(got[2].ToSlice()) != 1 {
		t.Errorf("Chunk(2) over [1 2 3 4 5] = %v, want 3 chunks: [1 2], [3 4], [5]", got)
	}
}

func TestWindowEdgeCases(t *testing.T) {
	if got := FromSlice([]int{1, 2, 3}).Window(5); got != nil {
		t.Errorf("Window(5) over 3 elements = %v, want nil", got)
	}
	if got := FromSlice([]int{1, 2, 3}).Window(0); got != nil {
		t.Errorf("Window(0) = %v, want nil", got)
	}

	got := FromSlice([]int{1, 2, 3, 4}).Window(3)
	if len(got) != 2 || len(got[0].ToSlice()) != 3 || len(got[1].ToSlice()) != 3 {
		t.Errorf("Window(3) over [1 2 3 4] = %v, want windows [1 2 3] and [2 3 4]", got)
	}
}

func TestReverse(t *testing.T) {
	got := FromSlice([]int{1, 2, 3}).Reverse().ToSlice()
	if len(got) != 3 || got[0] != 3 || got[1] != 2 || got[2] != 1 {
		t.Errorf("Reverse() = %v, want [3 2 1]", got)
	}
}

func TestShuffle(t *testing.T) {
	got := FromSlice([]int{1, 2, 3, 4}).Shuffle().ToSlice()
	if len(got) != 4 {
		t.Fatalf("Shuffle() returned %v, want 4 elements", got)
	}
	seen := make(map[int]struct{}, len(got))
	for _, v := range got {
		if _, ok := seen[v]; ok {
			t.Errorf("Shuffle() = %v, contains duplicate %d", got, v)
		}
		seen[v] = struct{}{}
	}
}

func TestConcat(t *testing.T) {
	got := FromSlice([]int{1, 2}).Concat(FromSlice([]int{3})).ToSlice()
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("Concat() = %v, want [1 2 3]", got)
	}
}

func TestConcatMultiple(t *testing.T) {
	got := FromSlice([]int{1}).Concat(FromSlice([]int{2}), FromSlice([]int{3})).ToSlice()
	if len(got) != 3 || got[0] != 1 || got[1] != 2 || got[2] != 3 {
		t.Errorf("Concat() over multiple sequences = %v, want [1 2 3]", got)
	}

	got = FromSlice([]int{1, 2}).Concat(FromSlice([]int{3}), FromSlice([]int{4, 5})).ToSlice()
	if len(got) != 5 || got[2] != 3 || got[3] != 4 || got[4] != 5 {
		t.Errorf("Concat() over mixed sequences = %v, want [1 2 3 4 5]", got)
	}
}

func TestConcatEmpty(t *testing.T) {
	if got := FromSlice([]int{1}).Concat().ToSlice(); len(got) != 1 || got[0] != 1 {
		t.Errorf("Concat() without args = %v, want [1]", got)
	}
}

func TestTakeWhile(t *testing.T) {
	seq := FromSlice([]int{1, 2, 3, 4, 5, 9})

	if got := seq.TakeWhile(func(x int) bool { return x < 5 }).ToSlice(); len(got) != 4 || got[3] != 4 {
		t.Errorf("TakeWhile(x < 5) = %v, want [1 2 3 4]", got)
	}
	if got := FromSlice([]int{5, 6, 7}).TakeWhile(func(x int) bool { return x < 5 }).ToSlice(); len(got) != 0 {
		t.Errorf("TakeWhile(false on first) = %v, want empty", got)
	}

	src := &countingSource{items: []int{1, 2, 3, 4}}
	if got := FromIter(src.Iter()).TakeWhile(func(x int) bool { return x < 3 }).ToSlice(); len(got) != 2 || got[1] != 2 {
		t.Errorf("TakeWhile over generator = %v, want [1 2]", got)
	}
	if src.pulled != 3 {
		t.Errorf("TakeWhile pulled %d elements, want 3 (stops at predicate failure)", src.pulled)
	}
}

func TestDropWhile(t *testing.T) {
	seq := FromSlice([]int{1, 2, 3, 4, 5, 9})

	if got := seq.DropWhile(func(x int) bool { return x < 3 }).ToSlice(); len(got) != 4 || got[0] != 3 || got[3] != 9 {
		t.Errorf("DropWhile(x < 3) = %v, want [3 4 5 9]", got)
	}
	if got := FromSlice([]int{1, 2, 3}).DropWhile(func(x int) bool { return x < 10 }).ToSlice(); len(got) != 0 {
		t.Errorf("DropWhile(true for all) = %v, want empty", got)
	}
}

func TestIndexOf(t *testing.T) {
	if got := FromSlice([]int{1, 2, 3, 2}).IndexOf(2); got != 1 {
		t.Errorf("IndexOf() = %d, want 1", got)
	}
	if got := FromSlice([]int{1, 2, 3}).IndexOf(99); got != -1 {
		t.Errorf("IndexOf() = %d, want -1", got)
	}
	if got := FromSlice([]string{"a", "b"}).IndexOf("b"); got != 1 {
		t.Errorf("IndexOf() over strings = %d, want 1", got)
	}
}

func TestLastIndexOf(t *testing.T) {
	if got := FromSlice([]int{1, 2, 3, 2}).LastIndexOf(2); got != 3 {
		t.Errorf("LastIndexOf() = %d, want 3", got)
	}
	if got := FromSlice([]int{1, 2, 3}).LastIndexOf(99); got != -1 {
		t.Errorf("LastIndexOf() = %d, want -1", got)
	}
}
