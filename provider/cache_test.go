package provider

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestCache_GetMiss(t *testing.T) {
	c := newCache(time.Minute)
	if got := c.get("missing"); got != nil {
		t.Errorf("get(missing) = %v, want nil", got)
	}
}

func TestCache_PutAndGet(t *testing.T) {
	c := newCache(time.Minute)
	c.put("key1", "value1")

	got := c.get("key1")
	if got != "value1" {
		t.Errorf("get(key1) = %v, want %q", got, "value1")
	}
}

func TestCache_Expiry(t *testing.T) {
	c := newCache(10 * time.Millisecond)
	c.put("key1", "value1")

	// Should be available immediately
	if got := c.get("key1"); got != "value1" {
		t.Errorf("get(key1) immediately = %v, want %q", got, "value1")
	}

	// Wait for expiry
	time.Sleep(20 * time.Millisecond)

	if got := c.get("key1"); got != nil {
		t.Errorf("get(key1) after expiry = %v, want nil", got)
	}
}

func TestCache_Invalidate(t *testing.T) {
	c := newCache(time.Minute)
	c.put("key1", "value1")
	c.put("key2", "value2")

	c.invalidate("key1")

	if got := c.get("key1"); got != nil {
		t.Errorf("get(key1) after invalidate = %v, want nil", got)
	}
	if got := c.get("key2"); got != "value2" {
		t.Errorf("get(key2) = %v, want %q", got, "value2")
	}
}

func TestCache_InvalidatePrefix(t *testing.T) {
	c := newCache(time.Minute)
	c.put("APP.policy.read", "p1")
	c.put("APP.policy.write", "p2")
	c.put("APP.binding.admin", "b1")
	c.put("OTHER.policy.read", "p3")

	c.invalidatePrefix("APP.policy.")

	if got := c.get("APP.policy.read"); got != nil {
		t.Errorf("get(APP.policy.read) after prefix invalidate = %v, want nil", got)
	}
	if got := c.get("APP.policy.write"); got != nil {
		t.Errorf("get(APP.policy.write) after prefix invalidate = %v, want nil", got)
	}
	if got := c.get("APP.binding.admin"); got != "b1" {
		t.Errorf("get(APP.binding.admin) = %v, want %q", got, "b1")
	}
	if got := c.get("OTHER.policy.read"); got != "p3" {
		t.Errorf("get(OTHER.policy.read) = %v, want %q", got, "p3")
	}
}

func TestCache_Clear(t *testing.T) {
	c := newCache(time.Minute)
	c.put("key1", "value1")
	c.put("key2", "value2")

	c.clear()

	if got := c.get("key1"); got != nil {
		t.Errorf("get(key1) after clear = %v, want nil", got)
	}
	if got := c.get("key2"); got != nil {
		t.Errorf("get(key2) after clear = %v, want nil", got)
	}
}

func TestCache_Concurrency(t *testing.T) {
	c := newCache(time.Minute)
	var wg sync.WaitGroup

	// Concurrent writers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			c.put(key, i)
		}(i)
	}

	// Concurrent readers
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			c.get(key)
		}(i)
	}

	// Concurrent invalidators
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("key-%d", i)
			c.invalidate(key)
		}(i)
	}

	wg.Wait()
}

func TestCache_OverwriteValue(t *testing.T) {
	c := newCache(time.Minute)
	c.put("key1", "old")
	c.put("key1", "new")

	if got := c.get("key1"); got != "new" {
		t.Errorf("get(key1) after overwrite = %v, want %q", got, "new")
	}
}
