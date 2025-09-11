package art

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

const (
	numKeys       = 1000
	maxKeyLen     = 64
	testDuration  = 15 * time.Second
	maxGoroutines = 100
	batchSize     = 1000
)

type TestStats struct {
	searches     int64
	inserts      int64
	deletes      int64
	updates      int64
	restarts     int64
	errors       int64
	totalLatency int64
	maxLatency   int64
	searchHits   int64
	searchMisses int64
}

func (s *TestStats) String() string {
	total := s.searches + s.inserts + s.deletes + s.updates
	avgLatency := float64(s.totalLatency) / float64(total)

	return fmt.Sprintf(`
Performance Stats:
  Operations: %d total
  - Searches: %d (%.1f%%, hits: %d, misses: %d)
  - Inserts:  %d (%.1f%%)
  - Updates:  %d (%.1f%%)
  - Deletes:  %d (%.1f%%)
  Restarts: %d
  Errors: %d
  Latency: avg=%.2fns, max=%dns
  Throughput: %.0f ops/sec
`,
		total,
		s.searches, float64(s.searches)/float64(total)*100, s.searchHits, s.searchMisses,
		s.inserts, float64(s.inserts)/float64(total)*100,
		s.updates, float64(s.updates)/float64(total)*100,
		s.deletes, float64(s.deletes)/float64(total)*100,
		s.restarts, s.errors,
		avgLatency, s.maxLatency,
		float64(total)/testDuration.Seconds(),
	)
}

type ContentionStats struct {
	TotalOps  int64
	Restarts  int64
	LockWaits int64
}

var globalStats ContentionStats

func TestBasicInsertAndSearch(t *testing.T) {
	tree := NewART()

	tree.Insert([]byte("hello"), "world")
	val, found := tree.Search([]byte("hello"))
	if !found {
		t.Error("Expected to find 'hello' key")
	}
	if val != "world" {
		t.Errorf("Expected 'world', got %v", val)
	}

	// Test non-existent key
	_, found = tree.Search([]byte("goodbye"))
	if found {
		t.Error("Should not find non-existent key")
	}
}

func TestMultipleInsertions(t *testing.T) {
	tree := NewART()

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

	for val, key := range testData {
		tree.Insert([]byte(key), val)
	}

	for expectedVal, key := range testData {
		val, found := tree.Search([]byte(key))
		if !found {
			t.Errorf("Expected to find key '%s'", key)
		}
		if val != expectedVal {
			t.Errorf("For key '%s', expected %v, got %v", key, expectedVal, val)
		}
	}
}

func TestPrefixHandling(t *testing.T) {
	tree := NewART()

	tree.Insert([]byte("test"), 1)
	tree.Insert([]byte("testing"), 2)
	tree.Insert([]byte("tester"), 3)
	tree.Insert([]byte("tea"), 4)
	tree.Insert([]byte("team"), 5)

	expected := map[string]interface{}{
		"test":    1,
		"testing": 2,
		"tester":  3,
		"tea":     4,
		"team":    5,
	}

	for key, expectedVal := range expected {
		val, found := tree.Search([]byte(key))
		if !found {
			t.Errorf("Expected to find key '%s'", key)
		}
		if val != expectedVal {
			t.Errorf("For key '%s', expected %v, got %v", key, expectedVal, val)
		}
	}
}

func TestEmptyString(t *testing.T) {
	tree := NewART()

	tree.Insert([]byte(""), "empty")
	val, found := tree.Search([]byte(""))
	if !found {
		t.Error("Expected to find empty string key")
	}
	if val != "empty" {
		t.Errorf("Expected 'empty', got %v", val)
	}
}

func TestSingleCharacterKeys(t *testing.T) {
	tree := NewART()

	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		tree.Insert([]byte(key), i)
	}

	for i := 0; i < 10; i++ {
		key := string(rune('a' + i))
		val, found := tree.Search([]byte(key))
		if !found {
			t.Errorf("Expected to find key '%s'", key)
		}
		if val != i {
			t.Errorf("For key '%s', expected %d, got %v", key, i, val)
		}
	}
}

func TestNodeGrowth(t *testing.T) {
	tree := NewART()

	// Test growth from node4 to node16
	keys := make([]string, 20)
	for i := 0; i < 20; i++ {
		keys[i] = fmt.Sprintf("key_%02d", i)
		tree.Insert([]byte(keys[i]), i)
	}

	// Verify all keys are still accessible
	for i, key := range keys {
		val, found := tree.Search([]byte(key))
		if !found {
			t.Errorf("Expected to find key '%s' after node growth", key)
		}
		if val != i {
			t.Errorf("For key '%s', expected %d, got %v", key, i, val)
		}
	}
}

func TestLargeNodeGrowth(t *testing.T) {
	tree := NewART()

	// Test growth to node48 and node256
	keys := make([]string, 100)
	for i := 0; i < 100; i++ {
		keys[i] = fmt.Sprintf("large_key_%03d", i)
		tree.Insert([]byte(keys[i]), i*10)
	}

	// Verify all keys
	for i, key := range keys {
		val, found := tree.Search([]byte(key))
		if !found {
			t.Errorf("Expected to find key '%s' after large node growth", key)
		}
		if val != i*10 {
			t.Errorf("For key '%s', expected %d, got %v", key, i*10, val)
		}
	}
}

func TestOverwriteValue(t *testing.T) {
	tree := NewART()

	// Insert initial value
	tree.Insert([]byte("key"), "value1")
	val, found := tree.Search([]byte("key"))
	if !found || val != "value1" {
		t.Error("Initial insertion failed")
	}

	// Overwrite with new value
	tree.Insert([]byte("key"), "value2")
	val, found = tree.Search([]byte("key"))
	if !found {
		t.Error("Key not found after overwrite")
	}
	if val != "value2" {
		t.Errorf("Expected 'value2', got %v", val)
	}
}

func TestSpecialCharacters(t *testing.T) {
	tree := NewART()

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
		tree.Insert([]byte(key), i)
	}

	for i, key := range specialKeys {
		val, found := tree.Search([]byte(key))
		if !found {
			t.Errorf("Expected to find special key '%s'", key)
		}
		if val != i {
			t.Errorf("For key '%s', expected %d, got %v", key, i, val)
		}
	}
}

func TestLongKeys(t *testing.T) {
	tree := NewART()

	longKey1 := strings.Repeat("a", 1000)
	longKey2 := strings.Repeat("b", 1000)
	longKey3 := strings.Repeat("a", 999) + "b"

	tree.Insert([]byte(longKey1), "long1")
	tree.Insert([]byte(longKey2), "long2")
	tree.Insert([]byte(longKey3), "long3")

	val, found := tree.Search([]byte(longKey1))
	if !found || val != "long1" {
		t.Error("Failed to find long key 1")
	}

	val, found = tree.Search([]byte(longKey2))
	if !found || val != "long2" {
		t.Error("Failed to find long key 2")
	}

	val, found = tree.Search([]byte(longKey3))
	if !found || val != "long3" {
		t.Error("Failed to find long key 3")
	}
}

func TestRandomInsertions(t *testing.T) {
	tree := NewART()
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
		tree.Insert([]byte(keyStr), i)
	}

	// Verify all keys
	for key, expectedVal := range keys {
		val, found := tree.Search([]byte(key))
		if !found {
			t.Errorf("Expected to find random key '%s'", key)
		}
		if val != expectedVal {
			t.Errorf("For key '%s', expected %d, got %v", key, expectedVal, val)
		}
	}
}

func TestMaxConcurrencyStress(t *testing.T) {
	tree := NewART() // Assuming your tree constructor
	keys := generateRandomKeys(numKeys)
	stats := &TestStats{}

	// Pre-populate tree with ALL keys to ensure searches always hit
	fmt.Printf("Pre-populating tree with %d keys...\n", numKeys)
	for i := 0; i < numKeys; i++ {
		tree.Insert(keys[i], i)
	}

	fmt.Printf("Starting max concurrency test with %d goroutines...\n", maxGoroutines)

	var wg sync.WaitGroup
	start := time.Now()

	for i := 0; i < maxGoroutines; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			localRand := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))
			workerStats := &TestStats{}

			for time.Since(start) < testDuration {
				keyIdx := localRand.Intn(numKeys)
				key := keys[keyIdx]
				op := localRand.Intn(100)

				opStart := time.Now()

				switch {
				case op < 70: // 70% searches
					val, found := tree.Search(key)
					atomic.AddInt64(&workerStats.searches, 1)
					if found {
						atomic.AddInt64(&workerStats.searchHits, 1)
						if val == nil {
							atomic.AddInt64(&workerStats.errors, 1)
						}
					} else {
						// This should never happen since all keys are pre-populated
						atomic.AddInt64(&workerStats.searchMisses, 1)
						fmt.Println(string(key), "the issue here")
						atomic.AddInt64(&workerStats.errors, 1) // Count as error since unexpected
					}

				default: // 30% inserts/updates (keys already exist, so these are updates)
					tree.Insert(key, keyIdx) // This will update existing key
					atomic.AddInt64(&workerStats.updates, 1)
				}

				latency := time.Since(opStart).Nanoseconds()
				atomic.AddInt64(&workerStats.totalLatency, latency)

				if latency > atomic.LoadInt64(&workerStats.maxLatency) {
					atomic.StoreInt64(&workerStats.maxLatency, latency)
				}
			}

			// Merge worker stats
			atomic.AddInt64(&stats.searches, workerStats.searches)
			atomic.AddInt64(&stats.inserts, workerStats.inserts)
			atomic.AddInt64(&stats.deletes, workerStats.deletes)
			atomic.AddInt64(&stats.updates, workerStats.updates)
			atomic.AddInt64(&stats.searchHits, workerStats.searchHits)
			atomic.AddInt64(&stats.searchMisses, workerStats.searchMisses)
			atomic.AddInt64(&stats.totalLatency, workerStats.totalLatency)
			atomic.AddInt64(&stats.errors, workerStats.errors)

			if workerStats.maxLatency > atomic.LoadInt64(&stats.maxLatency) {
				atomic.StoreInt64(&stats.maxLatency, workerStats.maxLatency)
			}
		}(i)
	}

	wg.Wait()

	// Verify no misses occurred
	if stats.searchMisses > 0 {
		t.Errorf("Unexpected search misses: %d", stats.searchMisses)
	}

	fmt.Print(stats.String())
}

func TestReadHeavyWorkload(t *testing.T) {
	tree := NewART()
	keys := generateRandomKeys(numKeys)
	stats := &TestStats{}

	// Pre-populate
	for i, key := range keys {
		tree.Insert(key, i)
	}

	fmt.Printf("Starting read-heavy test (95%% reads)...\n")

	var wg sync.WaitGroup
	start := time.Now()
	numReaders := runtime.NumCPU() * 20

	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			localRand := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			for time.Since(start) < testDuration {
				keyIdx := localRand.Intn(numKeys)
				key := keys[keyIdx]
				op := localRand.Intn(100)

				if op < 95 { // 95% reads
					val, found := tree.Search(key)
					atomic.AddInt64(&stats.searches, 1)
					if found {
						atomic.AddInt64(&stats.searchHits, 1)
						if val.(int) != keyIdx {
							atomic.AddInt64(&stats.errors, 1)
						}
					} else {
						atomic.AddInt64(&stats.searchMisses, 1)
					}
				} else { // 5% writes
					tree.Insert(generateRandomKey(32), localRand.Intn(1000000))
					atomic.AddInt64(&stats.inserts, 1)
				}
			}
		}(i)
	}

	wg.Wait()
	fmt.Print(stats.String())
}

func TestWriteHeavyWorkload(t *testing.T) {
	tree := NewART()
	stats := &TestStats{}

	fmt.Printf("Starting write-heavy test (80%% writes)...\n")

	var wg sync.WaitGroup
	start := time.Now()
	numWriters := runtime.NumCPU()
	keyCounter := int64(0)

	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			localRand := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))
			localKeys := make([][]byte, 0, numKeys)

			for time.Since(start) < testDuration {
				op := localRand.Intn(100)

				switch {
				case op < 80: // 80% inserts
					key := generateRandomKey(16)
					keyId := atomic.AddInt64(&keyCounter, 1)
					tree.Insert(key, int(keyId))
					localKeys = append(localKeys, key)
					atomic.AddInt64(&stats.inserts, 1)
				default:
					if len(localKeys) > 0 {
						key := localKeys[localRand.Intn(len(localKeys))]
						_, found := tree.Search(key)
						atomic.AddInt64(&stats.searches, 1)
						if found {
							atomic.AddInt64(&stats.searchHits, 1)
						} else {
							atomic.AddInt64(&stats.searchMisses, 1)
						}
					}
				}
			}
		}(i)
	}

	wg.Wait()
	fmt.Print(stats.String())
}

func TestHotspotContention(t *testing.T) {
	tree := NewART()
	stats := &TestStats{}

	// Create hotspot keys (common prefixes)
	hotKeys := [][]byte{
		[]byte("user:"),
		[]byte("session:"),
		[]byte("cache:"),
		[]byte("temp:"),
		[]byte("data:"),
	}

	allKeys := make([][]byte, 0, numKeys)
	for _, prefix := range hotKeys {
		keys := generatePrefixKeys(prefix, numKeys/len(hotKeys))
		allKeys = append(allKeys, keys...)
	}

	// Pre-populate
	for i, key := range allKeys {
		tree.Insert(key, i)
	}

	fmt.Printf("Starting hotspot contention test...\n")

	var wg sync.WaitGroup
	start := time.Now()
	numWorkers := 4

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func(workerID int) {
			defer wg.Done()

			localRand := rand.New(rand.NewSource(time.Now().UnixNano() + int64(workerID)))

			for time.Since(start) < testDuration {
				// 80% operations on hotspot keys
				var key []byte
				if localRand.Intn(100) < 80 {
					prefix := hotKeys[localRand.Intn(len(hotKeys))]
					suffix := generateRandomKey(32)
					key = make([]byte, len(prefix)+len(suffix))
					copy(key, prefix)
					copy(key[len(prefix):], suffix)
				} else {
					key = allKeys[localRand.Intn(len(allKeys))]
				}

				op := localRand.Intn(100)
				switch {
				case op < 60: // 60% searches
					_, found := tree.Search(key)
					atomic.AddInt64(&stats.searches, 1)
					if found {
						atomic.AddInt64(&stats.searchHits, 1)
					} else {
						atomic.AddInt64(&stats.searchMisses, 1)
					}

				default: // 40% inserts
					tree.Insert(key, localRand.Intn(1000000))
					atomic.AddInt64(&stats.inserts, 1)

				}
			}
		}(i)
	}

	wg.Wait()
	fmt.Print(stats.String())
}

func TestBurstLoad(t *testing.T) {
	tree := NewART()
	stats := &TestStats{}

	fmt.Printf("Starting burst load test...\n")

	start := time.Now()

	for burst := 0; burst < 10 && time.Since(start) < testDuration; burst++ {
		fmt.Printf("Burst %d...\n", burst+1)

		var wg sync.WaitGroup
		burstWorkers := 200 + burst*50 // Escalating load

		for i := 0; i < burstWorkers; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				for j := 0; j < batchSize; j++ {
					key := generateRandomKey(rand.Intn(maxKeyLen-1) + 1)

					// Rapid fire operations
					tree.Insert(key, j)
					atomic.AddInt64(&stats.inserts, 1)

					_, found := tree.Search(key)
					atomic.AddInt64(&stats.searches, 1)
					if found {
						atomic.AddInt64(&stats.searchHits, 1)
					}
				}
			}()
		}

		wg.Wait()
		time.Sleep(100 * time.Millisecond) // Brief pause between bursts
	}

	fmt.Print(stats.String())
}

func TestPathologicalCases(t *testing.T) {
	tree := NewART()
	stats := &TestStats{}

	fmt.Printf("Starting pathological cases test...\n")

	// Generate pathological keys
	pathologicalKeys := make([][]byte, 0, numKeys)

	// Long common prefixes
	commonPrefix := bytes.Repeat([]byte("a"), 50)
	for i := 0; i < numKeys/4; i++ {
		key := make([]byte, len(commonPrefix)+4)
		copy(key, commonPrefix)
		binary.BigEndian.PutUint32(key[len(commonPrefix):], uint32(i))
		pathologicalKeys = append(pathologicalKeys, key)
	}

	// Very long keys
	for i := 0; i < numKeys/4; i++ {
		key := bytes.Repeat([]byte("b"), 200+i%100)
		pathologicalKeys = append(pathologicalKeys, key)
	}

	// Sequential keys
	for i := 0; i < numKeys/4; i++ {
		key := make([]byte, 8)
		binary.BigEndian.PutUint64(key, uint64(i))
		pathologicalKeys = append(pathologicalKeys, key)
	}

	// Random remaining
	for i := len(pathologicalKeys); i < numKeys; i++ {
		pathologicalKeys = append(pathologicalKeys, generateRandomKey(64))
	}

	// Pre-populate
	for i, key := range pathologicalKeys {
		tree.Insert(key, i)
	}

	var wg sync.WaitGroup
	start := time.Now()
	numWorkers := runtime.NumCPU() * 12

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			localRand := rand.New(rand.NewSource(time.Now().UnixNano()))

			for time.Since(start) < testDuration {
				key := pathologicalKeys[localRand.Intn(len(pathologicalKeys))]

				_, found := tree.Search(key)
				atomic.AddInt64(&stats.searches, 1)
				if found {
					atomic.AddInt64(&stats.searchHits, 1)
				} else {
					atomic.AddInt64(&stats.searchMisses, 1)
				}
			}
		}()
	}

	wg.Wait()
	fmt.Print(stats.String())
}

func TestConcurrentInsertSearch(t *testing.T) {
	tree := NewART()
	numGoroutines := 1000
	numOperationsPerGoroutine := 10000

	var wg sync.WaitGroup
	var insertCount, searchCount int64

	// Start concurrent insert operations
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numOperationsPerGoroutine; j++ {
				key := fmt.Sprintf("key_%d_%d", goroutineID, j)
				value := fmt.Sprintf("value_%d_%d", goroutineID, j)

				tree.Insert([]byte(key), value)
				atomic.AddInt64(&insertCount, 1)

				// Immediately search for the inserted key
				if val, found := tree.Search([]byte(key)); found {
					if val.(string) == value {
						atomic.AddInt64(&searchCount, 1)
					} else {
						t.Errorf("Wrong value for key %s: expected %s, got %s", key, value, val.(string))
					}
				}
			}
		}(i)
	}

	wg.Wait()

	expectedOps := int64(numGoroutines * numOperationsPerGoroutine)
	t.Logf("Completed %d inserts and %d successful searches", insertCount, searchCount)

	if insertCount != expectedOps {
		t.Errorf("Expected %d inserts, got %d", expectedOps, insertCount)
	}
}

func TestConcurrentUpdateOperations(t *testing.T) {
	tree := NewART()
	numGoroutines := 10
	numUpdates := 100
	sharedKey := []byte("shared_key")

	var wg sync.WaitGroup
	var updateCount int64

	// Initialize the key
	tree.Insert(sharedKey, "initial_value")

	// Multiple goroutines updating the same key
	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			for j := 0; j < numUpdates; j++ {
				value := fmt.Sprintf("value_%d_%d", goroutineID, j)
				tree.Insert(sharedKey, value)
				atomic.AddInt64(&updateCount, 1)

				// Verify we can read the key (value might be from any goroutine)
				if _, found := tree.Search(sharedKey); !found {
					t.Errorf("Key disappeared after update by goroutine %d", goroutineID)
				}
			}
		}(i)
	}

	wg.Wait()

	// Final verification
	if val, found := tree.Search(sharedKey); !found {
		t.Error("Shared key not found after concurrent updates")
	} else {
		t.Logf("Final value: %s after %d updates", val.(string), updateCount)
	}
}

func TestConcurrentMixedOperations(t *testing.T) {
	tree := NewART()
	duration := 2 * time.Second
	numReaders := 5
	numWriters := 3

	var wg sync.WaitGroup
	done := make(chan struct{})
	var readOps, writeOps, readHits, writtenKeys int64

	// Pre-populate with some data
	for i := 0; i < 100; i++ {
		key := fmt.Sprintf("initial_%d", i)
		tree.Insert([]byte(key), i)
	}

	// Start writers
	for i := 0; i < numWriters; i++ {
		wg.Add(1)
		go func(writerID int) {
			defer wg.Done()
			counter := 0

			for {
				select {
				case <-done:
					return
				default:
					key := fmt.Sprintf("writer_%d_%d", writerID, counter)
					tree.Insert([]byte(key), counter)
					atomic.AddInt64(&writeOps, 1)
					atomic.AddInt64(&writtenKeys, 1)
					counter++

					// Small delay to allow readers to work
					if counter%10 == 0 {
						runtime.Gosched()
					}
				}
			}
		}(i)
	}

	// Start readers
	for i := 0; i < numReaders; i++ {
		wg.Add(1)
		go func(readerID int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(readerID)))

			for {
				select {
				case <-done:
					return
				default:
					// Mix of reading initial keys and writer keys
					var key string
					if r.Float64() < 0.5 {
						key = fmt.Sprintf("initial_%d", r.Intn(100))
					} else {
						writerID := r.Intn(numWriters)
						counter := r.Intn(100) // Might not exist yet
						key = fmt.Sprintf("writer_%d_%d", writerID, counter)
					}

					if _, found := tree.Search([]byte(key)); found {
						atomic.AddInt64(&readHits, 1)
					}
					atomic.AddInt64(&readOps, 1)

					// Small delay
					if readOps%100 == 0 {
						runtime.Gosched()
					}
				}
			}
		}(i)
	}

	// Let operations run for specified duration
	time.Sleep(duration)
	close(done)
	wg.Wait()

	t.Logf("Completed in %v:", duration)
	t.Logf("  Write operations: %d", writeOps)
	t.Logf("  Read operations: %d", readOps)
	t.Logf("  Read hits: %d (%.2f%%)", readHits, float64(readHits)/float64(readOps)*100)
	t.Logf("  Keys written: %d", writtenKeys)
}

func TestConcurrentPrefixOperations(t *testing.T) {
	tree := NewART()
	numGoroutines := 8

	var wg sync.WaitGroup

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()

			prefix := fmt.Sprintf("goroutine%d", goroutineID)

			for j := 0; j < 50; j++ {
				key := fmt.Sprintf("%s_key_%d", prefix, j)
				value := fmt.Sprintf("value_%d_%d", goroutineID, j)
				tree.Insert([]byte(key), value)
			}

			for j := 0; j < 50; j++ {
				key := fmt.Sprintf("%s_key_%d", prefix, j)
				if val, found := tree.Search([]byte(key)); found {
					expected := fmt.Sprintf("value_%d_%d", goroutineID, j)
					if val.(string) != expected {
						t.Errorf("Prefix test failed: expected %s, got %s", expected, val.(string))
					}
				} else {
					t.Errorf("Prefix test failed: key %s not found", key)
				}
			}
		}(i)
	}

	wg.Wait()
}

func TestConcurrentStressWithValidation(t *testing.T) {
	tree := NewART()
	numGoroutines := 20
	numOpsPerGoroutine := 500

	var wg sync.WaitGroup
	insertedKeys := sync.Map{} // Thread-safe map to track inserted keys
	var totalInserts int64

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			r := rand.New(rand.NewSource(time.Now().UnixNano() + int64(goroutineID)))

			for j := 0; j < numOpsPerGoroutine; j++ {
				// Generate a semi-random key with some collision potential
				keyNum := r.Intn(numGoroutines * numOpsPerGoroutine / 2) // Increase collision chance
				key := fmt.Sprintf("stress_key_%d", keyNum)
				value := fmt.Sprintf("value_%d_%d_%d", goroutineID, j, keyNum)

				tree.Insert([]byte(key), value)
				insertedKeys.Store(key, value)
				atomic.AddInt64(&totalInserts, 1)

				// Occasionally verify a random key
				if j%10 == 0 {
					verifyKey := fmt.Sprintf("stress_key_%d", r.Intn(keyNum+1))
					if expectedVal, exists := insertedKeys.Load(verifyKey); exists {
						if val, found := tree.Search([]byte(verifyKey)); !found {
							t.Errorf("Key %s should exist but not found", verifyKey)
						} else {
							// Note: due to concurrent updates, we can't guarantee exact value match
							// but we can verify the key exists
							_ = val
							_ = expectedVal
						}
					}
				}
			}
		}(i)
	}

	wg.Wait()

	t.Logf("Stress test completed: %d total inserts", totalInserts)

	// Final validation phase - check a sample of keys
	validationCount := 0
	insertedKeys.Range(func(key, expectedValue interface{}) bool {
		if val, found := tree.Search([]byte(key.(string))); found {
			validationCount++
			// Value might have been overwritten by concurrent operations
			_ = val
		} else {
			t.Errorf("Key %s not found during final validation", key.(string))
		}
		return validationCount < 100 // Limit validation to first 100 keys
	})

	t.Logf("Validated %d keys successfully", validationCount)
}

func BenchmarkInsertSequential(b *testing.B) {
	tree := NewART()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("key_%010d", i)
		tree.Insert([]byte(key), i)
	}
}

func BenchmarkInsertRandom(b *testing.B) {
	tree := NewART()
	rand.Seed(42)
	keys := make([]string, b.N)

	for i := 0; i < b.N; i++ {
		keys[i] = fmt.Sprintf("key_%010d", rand.Intn(1000000))
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Insert([]byte(keys[i]), i)
	}
}

func BenchmarkSearchExisting(b *testing.B) {
	tree := NewART()
	const numKeys = 100000

	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = fmt.Sprintf("key_%010d", i)
		tree.Insert([]byte(keys[i]), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[i%numKeys]
		tree.Search([]byte(key))
	}
}

func BenchmarkSearchNonExisting(b *testing.B) {
	tree := NewART()
	const numKeys = 100000

	for i := 0; i < numKeys; i++ {
		key := fmt.Sprintf("key_%010d", i)
		tree.Insert([]byte(key), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("nonexistent_%010d", i)
		tree.Search([]byte(key))
	}
}

func BenchmarkSearchRandomExisting(b *testing.B) {
	tree := NewART()
	const numKeys = 100000
	rand.Seed(42)

	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = fmt.Sprintf("key_%010d", rand.Intn(1000000))
		tree.Insert([]byte(keys[i]), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := keys[rand.Intn(numKeys)]
		tree.Search([]byte(key))
	}
}

func BenchmarkInsertShortKeys(b *testing.B) {
	tree := NewART()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("k%d", i)
		tree.Insert([]byte(key), i)
	}
}

func BenchmarkInsertLongKeys(b *testing.B) {
	tree := NewART()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := fmt.Sprintf("very_long_key_with_many_characters_%010d", i)
		tree.Insert([]byte(key), i)
	}
}

func BenchmarkInsertCommonPrefix(b *testing.B) {
	tree := NewART()
	prefix := "common_prefix_for_all_keys_"
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		key := prefix + strconv.Itoa(i)
		tree.Insert([]byte(key), i)
	}
}

func BenchmarkSearchCommonPrefix(b *testing.B) {
	tree := NewART()
	prefix := "common_prefix_for_all_keys_"
	const numKeys = 100000

	// Pre-populate
	for i := 0; i < numKeys; i++ {
		key := prefix + strconv.Itoa(i)
		tree.Insert([]byte(key), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		key := prefix + strconv.Itoa(i%numKeys)
		tree.Search([]byte(key))
	}
}

func BenchmarkMixedOperations(b *testing.B) {
	tree := NewART()
	rand.Seed(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if rand.Float32() < 0.7 { // 70% inserts
			key := fmt.Sprintf("key_%010d", rand.Intn(1000000))
			tree.Insert([]byte(key), i)
		} else {
			key := fmt.Sprintf("key_%010d", rand.Intn(1000000))
			tree.Search([]byte(key))
		}
	}
}

func BenchmarkMemoryUsage(b *testing.B) {
	for n := 1000; n <= 1000000; n *= 10 {
		b.Run(fmt.Sprintf("Keys_%d", n), func(b *testing.B) {
			tree := NewART()

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				b.StopTimer()
				// Create fresh tree for each iteration
				tree = NewART()

				b.StartTimer()
				for j := 0; j < n; j++ {
					key := fmt.Sprintf("key_%010d", j)
					tree.Insert([]byte(key), j)
				}
			}
		})
	}
}

func BenchmarkCompareWithMap_Insert(b *testing.B) {

	b.Run("Map", func(b *testing.B) {
		m := make(map[string]int)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key_%010d", i)
			m[key] = i
		}
	})
	b.Run("ART", func(b *testing.B) {
		tree := NewART()
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			key := fmt.Sprintf("key_%010d", i)
			tree.Insert([]byte(key), i)
		}
	})

}

func BenchmarkCompareWithMap_Search(b *testing.B) {
	const numKeys = 100000

	// Setup ART
	tree := NewART()
	keys := make([]string, numKeys)
	for i := 0; i < numKeys; i++ {
		keys[i] = fmt.Sprintf("key_%010d", i)
		tree.Insert([]byte(keys[i]), i)
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
			tree.Search([]byte(key))
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

func BenchmarkStressTest(b *testing.B) {
	tree := NewART()
	rand.Seed(time.Now().UnixNano())

	for i := 0; i < 10000; i++ {
		key := fmt.Sprintf("initial_%010d", i)
		tree.Insert([]byte(key), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		switch rand.Intn(3) {
		case 0:
			key := fmt.Sprintf("stress_%010d_%010d", i, rand.Intn(1000000))
			tree.Insert([]byte(key), i)
		case 1:
			key := fmt.Sprintf("initial_%010d", rand.Intn(10000))
			tree.Search([]byte(key))
		case 2:
			key := fmt.Sprintf("nonexist_%010d", rand.Intn(1000000))
			tree.Search([]byte(key))
		}
	}
}

func BenchmarkSingleThreadInsert(b *testing.B) {
	tree := NewART()
	keys := generateRandomKeys(b.N)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Insert(keys[i], i)
	}
}

func BenchmarkSingleThreadSearch(b *testing.B) {
	tree := NewART()
	keys := generateRandomKeys(10000)

	// Pre-populate the tree
	for i, key := range keys {
		tree.Insert(key, i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tree.Search(keys[i%len(keys)])
	}
}

func BenchmarkMultiThreadInsert(b *testing.B) {
	threadCounts := []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 1024, 2048, 5000, 10000}

	for _, numThreads := range threadCounts {
		b.Run(fmt.Sprintf("Threads-%d", numThreads), func(b *testing.B) {
			tree := NewART()
			keys := generateRandomKeys(b.N)

			var wg sync.WaitGroup
			keysPerThread := b.N / numThreads

			b.ResetTimer()
			for t := 0; t < numThreads; t++ {
				wg.Add(1)
				go func(threadID int) {
					defer wg.Done()
					start := threadID * keysPerThread
					end := start + keysPerThread
					if threadID == numThreads-1 {
						end = b.N // Handle remainder
					}

					for i := start; i < end; i++ {
						tree.Insert(keys[i], i)
					}
				}(t)
			}
			wg.Wait()
		})
	}
}

func BenchmarkMultiThreadSearch(b *testing.B) {
	threadCounts := []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 1024, 2048, 5000, 10000}

	for _, numThreads := range threadCounts {
		b.Run(fmt.Sprintf("Threads-%d", numThreads), func(b *testing.B) {
			tree := NewART()
			keys := generateRandomKeys(100000)

			// Pre-populate the tree
			for i, key := range keys {
				tree.Insert(key, i)
			}

			var wg sync.WaitGroup
			opsPerThread := b.N / numThreads

			b.ResetTimer()
			for t := 0; t < numThreads; t++ {
				wg.Add(1)
				go func(threadID int) {
					defer wg.Done()
					for i := 0; i < opsPerThread; i++ {
						keyIndex := (threadID*opsPerThread + i) % len(keys)
						tree.Search(keys[keyIndex])
					}
				}(t)
			}
			wg.Wait()
		})
	}
}

func BenchmarkMultiThreadMixed(b *testing.B) {
	ratios := []struct {
		name      string
		insertPct int
		searchPct int
	}{
		{"90Read10Write", 10, 90},
		{"50Read50Write", 50, 50},
		{"10Read90Write", 90, 10},
	}

	threadCounts := []int{1, 2, 4, 8, 16, 32, 64, 128, 256, 1024, 2048, 5000, 10000}

	for _, ratio := range ratios {
		for _, numThreads := range threadCounts {
			b.Run(fmt.Sprintf("%s-Threads-%d", ratio.name, numThreads), func(b *testing.B) {
				tree := NewART()
				keys := generateRandomKeys(100000)

				// Pre-populate with some initial data
				for i := 0; i < len(keys)/2; i++ {
					tree.Insert(keys[i], i)
				}

				var wg sync.WaitGroup
				opsPerThread := b.N / numThreads

				b.ResetTimer()
				for t := 0; t < numThreads; t++ {
					wg.Add(1)
					go func(threadID int) {
						defer wg.Done()
						for i := 0; i < opsPerThread; i++ {
							keyIndex := (threadID*opsPerThread + i) % len(keys)

							if i%100 < ratio.insertPct {
								// Insert operation
								tree.Insert(keys[keyIndex], keyIndex)
							} else {
								// Search operation
								tree.Search(keys[keyIndex])
							}
						}
					}(t)
				}
				wg.Wait()
			})
		}
	}
}

func BenchmarkContention(b *testing.B) {
	scenarioTypes := []struct {
		name string
		fn   func(int) [][]byte
	}{
		{"RandomKeys", func(n int) [][]byte { return generateRandomKeys(n) }},
		{"SequentialKeys", func(n int) [][]byte { return generateSequentialKeys(n, 16) }},
		{"CommonPrefix", func(n int) [][]byte { return generateCommonPrefixKeys(n, 8, 8) }},
	}

	for _, scenario := range scenarioTypes {
		b.Run(scenario.name, func(b *testing.B) {
			tree := NewART()
			keys := scenario.fn(b.N)
			numThreads := runtime.GOMAXPROCS(0)

			var wg sync.WaitGroup
			keysPerThread := b.N / numThreads

			b.ResetTimer()
			for t := 0; t < numThreads; t++ {
				wg.Add(1)
				go func(threadID int) {
					defer wg.Done()
					start := threadID * keysPerThread
					end := start + keysPerThread
					if threadID == numThreads-1 {
						end = b.N
					}

					for i := start; i < end; i++ {
						tree.Insert(keys[i], i)
					}
				}(t)
			}
			wg.Wait()
		})
	}
}

func BenchmarkContentionAnalysis(b *testing.B) {
	tree := NewART()
	keys := generateRandomKeys(b.N)
	numThreads := runtime.GOMAXPROCS(0)

	atomic.StoreInt64(&globalStats.TotalOps, 0)
	atomic.StoreInt64(&globalStats.Restarts, 0)
	atomic.StoreInt64(&globalStats.LockWaits, 0)

	var wg sync.WaitGroup
	keysPerThread := b.N / numThreads

	b.ResetTimer()
	start := time.Now()

	for t := 0; t < numThreads; t++ {
		wg.Add(1)
		go func(threadID int) {
			defer wg.Done()
			startIdx := threadID * keysPerThread
			endIdx := startIdx + keysPerThread
			if threadID == numThreads-1 {
				endIdx = b.N
			}

			for i := startIdx; i < endIdx; i++ {
				tree.Insert(keys[i], i)
				atomic.AddInt64(&globalStats.TotalOps, 1)
				// Note: You'd need to instrument your ART code to increment
				// globalStats.Restarts and globalStats.LockWaits
			}
		}(t)
	}
	wg.Wait()

	duration := time.Since(start)
	totalOps := atomic.LoadInt64(&globalStats.TotalOps)
	restarts := atomic.LoadInt64(&globalStats.Restarts)
	lockWaits := atomic.LoadInt64(&globalStats.LockWaits)

	b.ReportMetric(float64(totalOps)/duration.Seconds(), "ops/sec")
	b.ReportMetric(float64(restarts)/float64(totalOps)*100, "restart_pct")
	b.ReportMetric(float64(lockWaits)/float64(totalOps)*100, "lock_wait_pct")
}

func BenchmarkScalability(b *testing.B) {
	maxThreads := runtime.GOMAXPROCS(0) * 2
	keys := generateRandomKeys(100000)

	for numThreads := 1; numThreads <= maxThreads; numThreads *= 2 {
		b.Run(fmt.Sprintf("Threads-%d", numThreads), func(b *testing.B) {
			tree := NewART()

			var wg sync.WaitGroup
			opsPerThread := b.N / numThreads

			b.ResetTimer()
			start := time.Now()

			for t := 0; t < numThreads; t++ {
				wg.Add(1)
				go func(threadID int) {
					defer wg.Done()
					for i := 0; i < opsPerThread; i++ {
						keyIndex := (threadID*opsPerThread + i) % len(keys)
						tree.Insert(keys[keyIndex], keyIndex)
					}
				}(t)
			}
			wg.Wait()

			duration := time.Since(start)
			totalOps := int64(b.N)
			b.ReportMetric(float64(totalOps)/duration.Seconds(), "ops/sec")
			b.ReportMetric(float64(totalOps)/duration.Seconds()/float64(numThreads), "ops/sec/thread")
		})
	}
}

func BenchmarkMemoryPressure(b *testing.B) {
	sizes := []int{1000, 10000, 100000, 1000000}

	for _, size := range sizes {
		b.Run(fmt.Sprintf("Size-%d", size), func(b *testing.B) {
			tree := NewART()
			keys := generateRandomKeys(size)
			numThreads := runtime.GOMAXPROCS(0)

			// Pre-populate to create memory pressure
			for i, key := range keys {
				tree.Insert(key, i)
			}

			var wg sync.WaitGroup
			opsPerThread := b.N / numThreads

			b.ResetTimer()
			for t := 0; t < numThreads; t++ {
				wg.Add(1)
				go func(threadID int) {
					defer wg.Done()
					for i := 0; i < opsPerThread; i++ {
						keyIndex := (threadID*opsPerThread + i) % len(keys)
						// Mix of operations to stress the system
						if i%3 == 0 {
							tree.Insert(keys[keyIndex], keyIndex+1000000) // Update
						} else {
							tree.Search(keys[keyIndex])
						}
					}
				}(t)
			}
			wg.Wait()
		})
	}
}

func BenchmarkConcurrentOperations(b *testing.B) {
	tree := NewART()

	b.ResetTimer()

	b.RunParallel(func(pb *testing.PB) {
		counter := 0
		for pb.Next() {
			key := fmt.Sprintf("bench_key_%d", counter)
			value := fmt.Sprintf("bench_value_%d", counter)

			// Mix of insert and search operations
			if counter%2 == 0 {
				tree.Insert([]byte(key), value)
			} else {
				searchKey := fmt.Sprintf("bench_key_%d", counter-1)
				tree.Search([]byte(searchKey))
			}
			counter++
		}
	})
}

func generateRandomKeys(n int) [][]byte {
	length := rand.Intn(maxKeyLen-1) + 1
	keys := make([][]byte, n)
	for i := 0; i < n; i++ {
		keys[i] = generateRandomKey(length)
	}
	return keys
}
func generateRandomKey(length int) []byte {
	rand.Seed(time.Now().UnixNano())
	key := make([]byte, length)
	for i := 0; i < length; i++ {
		x := byte(rand.Intn(256))
		if x == TerminationChar {
			x = 'a'
		}
		key[i] = x
	}
	return key
}
func generatePrefixKeys(prefix []byte, n int) [][]byte {
	keys := make([][]byte, n)
	for i := 0; i < n; i++ {
		suffix := generateRandomKey(n)
		key := make([]byte, len(prefix)+len(suffix))
		copy(key, prefix)
		copy(key[len(prefix):], suffix)
		keys[i] = key
	}
	return keys
}
func generateSequentialKeys(count int, keyLength int) [][]byte {
	keys := make([][]byte, count)
	for i := 0; i < count; i++ {
		key := make([]byte, keyLength)
		for j := 0; j < keyLength; j++ {
			key[j] = byte((i + j) % 256)
		}
		keys[i] = key
	}
	return keys
}
func generateCommonPrefixKeys(count int, prefixLength int, suffixLength int) [][]byte {
	keys := make([][]byte, count)
	prefix := make([]byte, prefixLength)
	rand.Read(prefix)

	for i := 0; i < count; i++ {
		key := make([]byte, prefixLength+suffixLength)
		copy(key, prefix)

		suffix := make([]byte, suffixLength)
		rand.Read(suffix)
		copy(key[prefixLength:], suffix)

		keys[i] = key
	}
	return keys
}
