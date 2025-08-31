# ART - Adaptive Radix Trie

A Go implementation of the Adaptive Radix Trie (ART) data structure, a memory-efficient and high-performance trie optimized for in-memory indexing.

## Overview

This implementation is based on the research paper "The Adaptive Radix Tree: ARTful Indexing for Main-Memory Databases" by Viktor Leis et al. The ART data structure provides excellent performance characteristics for string-based key-value storage with adaptive node compression and efficient memory usage.

## Features

- **Adaptive Node Types**: Automatically grows from small to large node types based on the number of children
  - Node4: Up to 4 children (linear search)
  - Node16: Up to 16 children (linear search, SIMD-ready)
  - Node48: Up to 48 children (array indexing)
  - Node256: Up to 256 children (direct indexing)
- **Path Compression**: Eliminates single-child nodes by storing common prefixes
- **Memory Efficient**: Optimized memory layout for cache performance
- **Generic Value Storage**: Store any type of value with string keys

## Current Status

 **Work in Progress** - This implementation is currently under development with the following todo items:

- [ ] Unit tests
- [ ] SIMD implementation for Node16
- [ ] Thread safety

## Quick Start

```go
package main

import (
    "fmt"
)

func main() {
    // Create a new ART
    t := NewART()
    
    // Insert key-value pairs
    t.Insert("foo", 1)
    t.Insert("far", 2)
    t.Insert("fooz", 3)
    t.Insert("faz", 4)
    
    // Search for values
    value, found := t.Search("far")
    if found {
        fmt.Printf("Found: %v\n", value) // Output: Found: 2
    }
}
```

## API Reference

### Tree

#### `NewART() Tree`
Creates a new empty ART instance.

#### `Insert(key string, val interface{})`
Inserts a key-value pair into the tree. If the key already exists, the value will be updated.

**Parameters:**
- `key`: The string key to insert
- `val`: The value to associate with the key (can be any type)

#### `Search(key string) (interface{}, bool)`
Searches for a key in the tree and returns its associated value.

**Parameters:**
- `key`: The string key to search for

**Returns:**
- `interface{}`: The value associated with the key (nil if not found)
- `bool`: True if the key was found, false otherwise

## Architecture

### Node Types

The ART uses four different node types that adapt based on the number of children:

1. **Node4**: Stores up to 4 children using linear arrays
2. **Node16**: Stores up to 16 children using linear arrays (SIMD-optimizable)
3. **Node48**: Stores up to 48 children using an index array for O(1) lookup
4. **Node256**: Stores up to 256 children with direct array indexing

### Path Compression

The implementation uses path compression to reduce memory usage and improve cache performance by storing common prefixes directly in nodes rather than creating chains of single-child nodes.

## Performance Characteristics

- **Search**: O(k) where k is the key length
- **Insert**: O(k) where k is the key length
- **Space**: Adaptive based on data distribution
- **Cache-friendly**: Optimized memory layout for modern CPUs

## Building and Running

```bash
# Clone or download the project
# Ensure you have Go 1.25.0 or later installed

# Run the example
go run main.go

# Build the project
go build
```

## References

This implementation is based on the following resources:

- [Original ART Paper](https://db.in.tum.de/~leis/papers/ART.pdf) - Viktor Leis et al.
- [ART Paper Notes](https://www.the-paper-trail.org/post/art-paper-notes/)
- [Medium Article on ART Implementation](https://medium.com/techlog/how-i-implemented-an-art-adaptive-radix-trie-data-structure-in-go-to-increase-the-performance-of-a8a2300b246a)
- [go-art by kellydunn](https://github.com/kellydunn/go-art)
- [art by arriqaaq](https://github.com/arriqaaq/art)

## Contributing

This is currently a work-in-progress implementation. Contributions are welcome, especially for:

- Unit tests
- SIMD optimizations for Node16
- Thread safety improvements
- Performance benchmarks
- Bug fixes and optimizations

## License

This project is open source. Please check the repository for license details.

## Future Improvements

- [ ] Complete unit test coverage
- [ ] SIMD implementation for faster Node16 searches
- [ ] Thread-safe operations using appropriate synchronization
- [ ] Benchmarking suite
- [ ] Memory usage optimizations
- [ ] Support for deletion operations
- [ ] Iterator support for range queries
