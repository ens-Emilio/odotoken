package cache

import (
	"testing"
	"time"
)

func TestSemanticCache_GetMiss(t *testing.T) {
	c := NewSemanticCache(0.9, time.Minute, 10)
	_, ok := c.Get([]float64{1, 0, 0})
	if ok {
		t.Error("expected miss on empty cache")
	}
}

func TestSemanticCache_SetAndGet(t *testing.T) {
	c := NewSemanticCache(0.9, time.Minute, 10)
	emb := []float64{1, 0, 0}
	resp := []byte("hello")
	c.Set(emb, resp)

	got, ok := c.Get(emb)
	if !ok {
		t.Fatal("expected hit")
	}
	if string(got) != "hello" {
		t.Errorf("got %q, want %q", got, "hello")
	}
}

func TestSemanticCache_BelowThreshold(t *testing.T) {
	c := NewSemanticCache(0.99, time.Minute, 10)
	c.Set([]float64{1, 0, 0}, []byte("a"))

	// Orthogonal vector should not match
	_, ok := c.Get([]float64{0, 1, 0})
	if ok {
		t.Error("expected miss for orthogonal vector below threshold")
	}
}

func TestSemanticCache_Expiry(t *testing.T) {
	c := NewSemanticCache(0.5, 1*time.Millisecond, 10)
	c.Set([]float64{1, 0, 0}, []byte("x"))
	time.Sleep(5 * time.Millisecond)

	_, ok := c.Get([]float64{1, 0, 0})
	if ok {
		t.Error("expected miss after TTL expiry")
	}
}

func TestSemanticCache_Eviction(t *testing.T) {
	c := NewSemanticCache(0.5, time.Hour, 2)
	c.Set([]float64{1, 0, 0}, []byte("first"))
	c.Set([]float64{0, 1, 0}, []byte("second"))
	c.Set([]float64{0, 0, 1}, []byte("third"))

	if c.Len() > 2 {
		t.Errorf("cache size %d exceeds max 2", c.Len())
	}
}

func TestCosineSimilarity_Identical(t *testing.T) {
	v := []float64{1, 2, 3}
	s := cosineSimilarity(v, v)
	if s < 0.999 || s > 1.001 {
		t.Errorf("identical vectors similarity = %f, want ~1.0", s)
	}
}

func TestCosineSimilarity_Orthogonal(t *testing.T) {
	a := []float64{1, 0}
	b := []float64{0, 1}
	s := cosineSimilarity(a, b)
	if s > 0.001 {
		t.Errorf("orthogonal vectors similarity = %f, want ~0.0", s)
	}
}

func TestCosineSimilarity_DifferentLengths(t *testing.T) {
	s := cosineSimilarity([]float64{1}, []float64{1, 2})
	if s != 0 {
		t.Errorf("different length similarity = %f, want 0", s)
	}
}