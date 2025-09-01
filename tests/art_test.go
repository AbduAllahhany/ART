package tests

import (
	"art"
	"fmt"
	"math/rand"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Test basic insertion and search functionality
func TestBasicInsertAndSearch(t *testing.T) {
	tree := art.NewART()

	// Test single insertion
	tree.Insert("hello", "world")
	val, found := tree.Search("hello")
	if !found {
		t.Error("Expected to find 'hello' key")
	}
	if val != "world" {
		t.Errorf("Expected 'world', got %v", val)
	}

	// Test non-existent key
	_, found = tree.Search("goodbye")
	if found {
		t.Error("Should not find non-existent key")
	}
}

func TestMultipleInsertions(t *testing.T) {
	tree := art.NewART()

	testData := []string{
		"apple",
		"app",
		"apply",
		"apt",
		"banana",
		"band",
		"can",
		"cat",
		"car",
	}

	// Insert all data
	for val, key := range testData {
		tree.Insert(key, val)
	}

	// Verify all data can be found
	for expectedVal, key := range testData {
		val, found := tree.Search(key)
		if !found {
			t.Errorf("Expected to find key '%s'", key)
		}
		if val != expectedVal {
			t.Errorf("For key '%s', expected %v, got %v", key, expectedVal, val)
		}
	}
}

func TestPrefixHandling(t *testing.T) {
	tree := art.NewART()

	// Test keys with common prefixes
	tree.Insert("test", 1)
	tree.Insert("testing", 2)
	tree.Insert("tester", 3)
	tree.Insert("tea", 4)
	tree.Insert("team", 5)

	// Verify all keys
	expected := map[string]interface{}{
		"test":    1,
		"testing": 2,
		"tester":  3,
		"tea":     4,
		"team":    5,
	}

	for key, expectedVal := range expected {
		val, found := tree.Search(key)
		if !found {
			t.Errorf("Expected to find key '%s'", key)
		}
		if val != expectedVal {
			t.Errorf("For key '%s', expected %v, got %v", key, expectedVal, val)
		}
	}
}

func TestEmptyString(t *testing.T) {
	tree := art.NewART()

	tree.Insert("", "empty")
	val, found := tree.Search("")
	if !found {
		t.Error("Expected to find empty string key")
	}
	if val != "empty" {
		t.Errorf("Expected 'empty', got %v", val)
	}
}

func TestSingleCharacterKeys(t *testing.T) {
	tree := art.NewART()

	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		tree.Insert(key, i)
	}

	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		val, found := tree.Search(key)
		if !found {
			t.Errorf("Expected to find key '%s'", key)
		}
		if val != i {
			t.Errorf("For key '%s', expected %d, got %v", key, i, val)
		}
	}
}

func TestNodeGrowth(t *testing.T) {
	tree := art.NewART()

	// Test growth from node4 to node16
	keys := make([]string, 20)
	for i := 0; i < 20; i++ {
		keys[i] = fmt.Sprintf("key_%02d", i)
		tree.Insert(keys[i], i)
	}

	// Verify all keys are still accessible
	for i, key := range keys {
		val, found := tree.Search(key)
		if !found {
			t.Errorf("Expected to find key '%s' after node growth", key)
		}
		if val != i {
			t.Errorf("For key '%s', expected %d, got %v", key, i, val)
		}
	}
}

func TestLargeNodeGrowth(t *testing.T) {
	tree := art.NewART()

	// Test growth to node48 and node256
	keys := make([]string, 100)
	for i := 0; i < 100; i++ {
		keys[i] = fmt.Sprintf("large_key_%03d", i)
		tree.Insert(keys[i], i*10)
	}

	// Verify all keys
	for i, key := range keys {
		val, found := tree.Search(key)
		if !found {
			t.Errorf("Expected to find key '%s' after large node growth", key)
		}
		if val != i*10 {
			t.Errorf("For key '%s', expected %d, got %v", key, i*10, val)
		}
	}
}

func TestOverwriteValue(t *testing.T) {
	tree := art.NewART()

	// Insert initial value
	tree.Insert("key", "value1")
	val, found := tree.Search("key")
	if !found || val != "value1" {
		t.Error("Initial insertion failed")
	}

	// Overwrite with new value
	tree.Insert("key", "value2")
	val, found = tree.Search("key")
	if !found {
		t.Error("Key not found after overwrite")
	}
	if val != "value2" {
		t.Errorf("Expected 'value2', got %v", val)
	}
}

func TestSpecialCharacters(t *testing.T) {
	tree := art.NewART()

	specialKeys := []string{
		"key with spaces",
		"key-with-dashes",
		"key_with_underscores",
		"key.with.dots",
		"key@with@symbols",
		"key123with456numbers",
		"UPPERCASE_KEY",
		"MiXeD_cAsE_kEy",
	}

	for i, key := range specialKeys {
		tree.Insert(key, i)
	}

	for i, key := range specialKeys {
		val, found := tree.Search(key)
		if !found {
			t.Errorf("Expected to find special key '%s'", key)
		}
		if val != i {
			t.Errorf("For key '%s', expected %d, got %v", key, i, val)
		}
	}
}

func TestLongKeys(t *testing.T) {
	tree := art.NewART()

	// Test with very long keys
	longKey1 := strings.Repeat("a", 1000)
	longKey2 := strings.Repeat("b", 1000)
	longKey3 := strings.Repeat("a", 999) + "b" // Similar but different

	tree.Insert(longKey1, "long1")
	tree.Insert(longKey2, "long2")
	tree.Insert(longKey3, "long3")

	val, found := tree.Search(longKey1)
	if !found || val != "long1" {
		t.Error("Failed to find long key 1")
	}

	val, found = tree.Search(longKey2)
	if !found || val != "long2" {
		t.Error("Failed to find long key 2")
	}

	val, found = tree.Search(longKey3)
	if !found || val != "long3" {
		t.Error("Failed to find long key 3")
	}
}

func TestRandomInsertions(t *testing.T) {
	tree := art.NewART()
	rand.Seed(42) // For reproducible tests

	const numKeys = 1000
	keys := make(map[string]int)

	// Generate random keys
	for i := 0; i < numKeys; i++ {
		keyLength := rand.Intn(20) + 1
		key := make([]byte, keyLength)
		for j := range key {
			key[j] = byte(rand.Intn(95) + 32) // Printable ASCII
		}
		keyStr := string(key)
		keys[keyStr] = i
		tree.Insert(keyStr, i)
	}

	// Verify all keys
	for key, expectedVal := range keys {
		val, found := tree.Search(key)
		if !found {
			t.Errorf("Expected to find random key '%s'", key)
		}
		if val != expectedVal {
			t.Errorf("For key '%s', expected %d, got %v", key, expectedVal, val)
		}
	}
}

// Benchmarks
func BenchmarkInsertSequential(b *testing.B) {
	tree := art.NewART()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%010d", i)
		tree.Insert(key, i)
	}
}

func BenchmarkInsertRandom(b *testing.B) {
	tree := art.NewART()
	rand.Seed(42)
	keys := make([]string, b.N)

	// Pre-generate keys
	for i := 0; i < b.N; i++ {
		keys[i] = fmt.Sprintf("key_%010d", rand.Intn(1000000))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Insert(keys[i], i)
	}
}

func BenchmarkSearchExisting(b *testing.B) {
	tree := art.NewART()
	const numKeys = 100000

	// Pre-populate tree
	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = fmt.Sprintf("key_%010d", i)
		tree.Insert(keys[i], i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i%numKeys]
		tree.Search(key)
	}
}

func BenchmarkSearchNonExisting(b *testing.B) {
	tree := art.NewART()
	const numKeys = 100000

	// Pre-populate tree
	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key_%010d", i)
		tree.Insert(key, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("nonexistent_%010d", i)
		tree.Search(key)
	}
}

func BenchmarkSearchRandomExisting(b *testing.B) {
	tree := art.NewART()
	const numKeys = 100000
	rand.Seed(42)

	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = fmt.Sprintf("key_%010d", rand.Intn(1000000))
		tree.Insert(keys[i], i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[rand.Intn(numKeys)]
		tree.Search(key)
	}
}

func BenchmarkInsertShortKeys(b *testing.B) {
	tree := art.NewART()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("k%d", i)
		tree.Insert(key, i)
	}
}

func BenchmarkInsertLongKeys(b *testing.B) {
	tree := art.NewART()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("very_long_key_with_many_characters_%010d", i)
		tree.Insert(key, i)
	}
}

func BenchmarkInsertCommonPrefix(b *testing.B) {
	tree := art.NewART()
	prefix := "common_prefix_for_all_keys_"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := prefix + strconv.Itoa(i)
		tree.Insert(key, i)
	}
}

func BenchmarkSearchCommonPrefix(b *testing.B) {
	tree := art.NewART()
	prefix := "common_prefix_for_all_keys_"
	const numKeys = 100000

	// Pre-populate
	for i := 0; i < numKeys; i++ {
		key := prefix + strconv.Itoa(i)
		tree.Insert(key, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := prefix + strconv.Itoa(i%numKeys)
		tree.Search(key)
	}
}

func BenchmarkMixedOperations(b *testing.B) {
	tree := art.NewART()
	rand.Seed(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if rand.Float32() < 0.7 { // 70% inserts
			key := fmt.Sprintf("key_%010d", rand.Intn(1000000))
			tree.Insert(key, i)
		} else { // 30% searches
			key := fmt.Sprintf("key_%010d", rand.Intn(1000000))
			tree.Search(key)
		}
	}
}

// Memory usage benchmark
func BenchmarkMemoryUsage(b *testing.B) {
	for n := 1000; n <= 1000000; n *= 10 {
		b.Run(fmt.Sprintf("Keys_%d", n), func(b *testing.B) {
			tree := art.NewART()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				// Create fresh tree for each iteration
				tree = art.NewART()

				b.StartTimer()
				for j := 0; j < n; j++ {
					key := fmt.Sprintf("key_%010d", j)
					tree.Insert(key, j)
				}
			}
		})
	}
}

// Comparative benchmark against Go's built-in map
func BenchmarkCompareWithMap_Insert(b *testing.B) {
	b.Run("ART", func(b *testing.B) {
		tree := art.NewART()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key_%010d", i)
			tree.Insert(key, i)
		}
	})

	b.Run("Map", func(b *testing.B) {
		m := make(map[string]int)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key_%010d", i)
			m[key] = i
		}
	})
}

func BenchmarkCompareWithMap_Search(b *testing.B) {
	const numKeys = 100000

	// Setup ART
	tree := art.NewART()
	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = fmt.Sprintf("key_%010d", i)
		tree.Insert(keys[i], i)
	}

	// Setup Map
	m := make(map[string]int)
	for i := 0; i < numKeys; i++ {
		m[keys[i]] = i
	}

	b.Run("ART", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := keys[i%numKeys]
			tree.Search(key)
		}
	})

	b.Run("Map", func(b *testing.B) {
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := keys[i%numKeys]
			_ = m[key]
		}
	})
}

// Stress test with concurrent access patterns
func BenchmarkStressTest(b *testing.B) {
	tree := art.NewART()
	rand.Seed(time.Now().UnixNano())

	// Pre-populate with some data
	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("initial_%010d", i)
		tree.Insert(key, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		switch rand.Intn(3) {
		case 0: // Insert new key
			key := fmt.Sprintf("stress_%010d_%010d", i, rand.Intn(1000000))
			tree.Insert(key, i)
		case 1: // Search existing key
			key := fmt.Sprintf("initial_%010d", rand.Intn(10000))
			tree.Search(key)
		case 2: // Search non-existing key
			key := fmt.Sprintf("nonexist_%010d", rand.Intn(1000000))
			tree.Search(key)
		}
	}
}
