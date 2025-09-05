package art

import (
	"bytes"
	"runtime"
	"sync/atomic"
)

// todo
// 1) implement SIMD
// 2) make it threadsafe
// 3) Combine ART with a Bloom filter for ultra-fast negative lookups.
const TerminationChar = '\xff'
const MaxInlinePrefixLength = 8

type nodeType int

const (
	nodeTypeLeaf nodeType = iota
	nodeType4
	nodeType16
	nodeType48
	nodeType256
)

type Tree struct {
	node node
}

func NewART() Tree {
	return Tree{
		node: nil,
	}
}

func insert(n node, key []byte, l *leaf, depth int) node {
	if n == nil {
		return l
	}
	if n.getType() == nodeTypeLeaf {
		newNode := node4{
			childPtr:      [4]node{},
			prefixPtr:     nil,
			prefix:        [8]byte{},
			keys:          [4]byte{},
			prefixLen:     0,
			numOfChildren: 0,
		}
		key2 := loadKey(n)
		newNode.setPrefix(getCommonPrefix(key, key2, depth))
		depth += int(newNode.prefixLen)
		if bytes.Equal(n.(*leaf).key, key) {
			n.(*leaf).val = l.val
			return n
		}
		addChild(&newNode, n, key2, depth)
		addChild(&newNode, l, key, depth)
		return &newNode
	}
	curPrefix := n.getPrefix()
	p := checkPrefix(curPrefix, key, depth)
	if p != len(curPrefix) { // prefix mismatch
		newNode := node4{
			keys:          [4]byte{},
			childPtr:      [4]node{},
			numOfChildren: 0,
		}
		addChild(&newNode, l, key, depth+p)
		addChild(&newNode, n, curPrefix, p)
		newNode.setPrefix(curPrefix[:p])
		n.setPrefix(curPrefix[p:])
		return &newNode
	}
	depth += len(curPrefix)
	next, idx := findChild(n, key, depth)
	if next != nil {
		updated := insert(next, key, l, depth)
		n.replaceChild(uint8(idx), updated)
	} else {
		if n.isFull() {
			n = n.grow()
		}
		addChild(n, l, key, depth)
	}
	return n
}
func search(n node, key []byte, depth int) (interface{}, bool) {
	if n == nil {
		return nil, false
	}
	if n.getType() == nodeTypeLeaf {
		l := n.(*leaf)
		return l.val, bytes.Equal(l.key, key)
	}
	pre := n.getPrefix()
	p := checkPrefix(pre, key, depth)
	l := len(pre)
	if p != l {
		return nil, false
	}
	depth += len(n.getPrefix())
	var next node
	next, _ = findChild(n, key, depth)
	return search(next, key, depth)
}
func (t *Tree) Insert(key []byte, val interface{}) {
	l := leaf{
		key: key,
		val: val,
	}
	t.node = insert(t.node, key, &l, 0)
}
func (t *Tree) Search(key []byte) (interface{}, bool) {
	return search(t.node, key, 0)
}

type node interface {
	getType() nodeType
	findChild(b byte) (node, int16)
	replaceChild(uint8, node)
	isFull() bool
	getPrefix() []byte
	addChild(k byte, child node)
	grow() node
	setPrefix(prefix []byte)
	version() *atomic.Uint64
}

type leaf struct {
	key                 []byte
	versionLockObsolete *atomic.Uint64 //62b version 1b lock 1b obsolete
	val                 interface{}
}

func (l *leaf) setPrefix(prefix []byte) {
}
func (l *leaf) findChild(b byte) (node, int16) {
	return nil, -1
}
func (l *leaf) grow() node {
	return nil
}
func (l *leaf) getType() nodeType {
	return nodeTypeLeaf
}
func (l *leaf) replaceChild(i uint8, n node) {
	return
}
func (l *leaf) isFull() bool {
	return false
}
func (l *leaf) getPrefix() []byte {
	return nil
}
func (l *leaf) addChild(k byte, child node) {
	return
}
func (l *leaf) version() *atomic.Uint64 { return l.versionLockObsolete }

type node4 struct {
	childPtr            [4]node
	prefixPtr           []byte
	prefix              [MaxInlinePrefixLength]byte
	versionLockObsolete *atomic.Uint64 //62b version 1b lock 1b obsolete
	keys                [4]uint8
	prefixLen           uint16
	numOfChildren       uint8
}

func (n *node4) setPrefix(prefix []byte) {
	length := len(prefix)
	n.prefixLen = uint16(length)
	if length <= MaxInlinePrefixLength {
		copy(n.prefix[:length], prefix)
		return
	}
	n.prefixPtr = prefix
}
func (n *node4) grow() node {
	newNode := node16{
		childPtr:      [16]node{},
		prefixPtr:     n.prefixPtr,
		keys:          [16]uint8{},
		prefix:        n.prefix,
		prefixLen:     n.prefixLen,
		numOfChildren: n.numOfChildren,
	}
	copy(newNode.keys[:], n.keys[:])
	copy(newNode.childPtr[:], n.childPtr[:])
	return &newNode
}
func (n *node4) getPrefix() []byte {
	if n.prefixLen > MaxInlinePrefixLength {
		return n.prefixPtr
	}
	return n.prefix[:n.prefixLen]

}
func (n *node4) getType() nodeType {
	return nodeType4
}
func (n *node4) isFull() bool {
	return n.numOfChildren == 4
}
func (n *node4) findChild(b byte) (node, int16) {
	//simple search over keys
	for i, k := range n.keys {
		if k == b {
			return n.childPtr[i], int16(i)
		}
	}
	return nil, -1
}
func (n *node4) addChild(k byte, child node) {
	n.keys[n.numOfChildren] = k
	n.childPtr[n.numOfChildren] = child
	n.numOfChildren++
}
func (n *node4) replaceChild(idx uint8, child node) {
	n.childPtr[idx] = child
}
func (n *node4) version() *atomic.Uint64 {
	return n.versionLockObsolete
}

type node16 struct {
	childPtr            [16]node
	prefixPtr           []byte
	keys                [16]uint8
	prefix              [MaxInlinePrefixLength]byte
	versionLockObsolete *atomic.Uint64 //62b version 1b lock 1b obsolete
	prefixLen           uint16
	numOfChildren       uint8
}

func (n *node16) setPrefix(prefix []byte) {
	length := len(prefix)
	n.prefixLen = uint16(length)
	if length <= MaxInlinePrefixLength {
		copy(n.prefix[:length], prefix)
		return
	}
	n.prefixPtr = prefix
}
func (n *node16) getType() nodeType {
	return nodeType16
}
func (n *node16) findChild(b byte) (node, int16) {
	//todo use SIMD
	for i, k := range n.keys {
		if k == b {
			return n.childPtr[i], int16(i)
		}
	}
	return nil, -1

}
func (n *node16) replaceChild(idx uint8, child node) {
	n.childPtr[idx] = child

}
func (n *node16) isFull() bool {
	return n.numOfChildren == 16
}
func (n *node16) getPrefix() []byte {
	if n.prefixLen > MaxInlinePrefixLength {
		return n.prefixPtr
	}
	return n.prefix[:n.prefixLen]
}
func (n *node16) addChild(k byte, child node) {
	n.keys[n.numOfChildren] = byte(k)
	n.childPtr[n.numOfChildren] = child
	n.numOfChildren++
}
func (n *node16) grow() node {
	var idxArr [256]int16
	for i := 0; i < 256; i++ {
		idxArr[i] = -1
	}
	newNode := node48{
		childPtr:      [48]node{},
		prefixPtr:     n.prefixPtr,
		childIndex:    idxArr,
		prefix:        n.prefix,
		prefixLen:     n.prefixLen,
		numOfChildren: n.numOfChildren,
	}
	for i := 0; i < int(n.numOfChildren); i++ {
		newNode.childPtr[i] = n.childPtr[i]
		newNode.childIndex[n.keys[i]] = int16(i)
	}
	return &newNode
}
func (n *node16) version() *atomic.Uint64 {
	return n.versionLockObsolete
}

type node48 struct {
	childPtr            [48]node
	prefixPtr           []byte
	childIndex          [256]int16
	versionLockObsolete *atomic.Uint64 //62b version 1b lock 1b obsolete
	prefix              [MaxInlinePrefixLength]byte
	prefixLen           uint16
	numOfChildren       uint8
}

func (n *node48) setPrefix(prefix []byte) {
	length := len(prefix)
	n.prefixLen = uint16(length)
	if length <= MaxInlinePrefixLength {
		copy(n.prefix[:length], prefix)
		return
	}
	n.prefixPtr = prefix
}
func (n *node48) getType() nodeType {
	return nodeType48
}
func (n *node48) findChild(b byte) (node, int16) {
	if n.childIndex[b] != -1 {
		return n.childPtr[n.childIndex[b]], n.childIndex[b]
	}
	return nil, -1
}
func (n *node48) addChild(b byte, child node) {
	n.childIndex[b] = int16(n.numOfChildren)
	n.childPtr[n.numOfChildren] = child
	n.numOfChildren++
}
func (n *node48) replaceChild(idx uint8, child node) {
	n.childPtr[idx] = child
}
func (n *node48) isFull() bool {
	return n.numOfChildren == 48
}
func (n *node48) getPrefix() []byte {
	if n.prefixLen > MaxInlinePrefixLength {
		return n.prefixPtr
	}
	return n.prefix[:n.prefixLen]
}
func (n *node48) grow() node {
	newNode := node256{
		ChildPtr:  [256]node{},
		prefixPtr: n.prefixPtr,
		prefixLen: n.prefixLen,
		prefix:    n.prefix,
	}
	for char := 0; char < 256; char++ {
		if n.childIndex[char] != -1 {
			newNode.ChildPtr[char] = n.childPtr[n.childIndex[char]]
		}
	}
	return &newNode
}
func (n *node48) version() *atomic.Uint64 {
	return n.versionLockObsolete
}

type node256 struct {
	ChildPtr            [256]node
	prefixPtr           []byte
	versionLockObsolete *atomic.Uint64 //62b version 1b lock 1b obsolete
	prefixLen           uint16
	prefix              [MaxInlinePrefixLength]byte
}

func (n *node256) setPrefix(prefix []byte) {
	length := len(prefix)
	n.prefixLen = uint16(length)
	if length <= MaxInlinePrefixLength {
		copy(n.prefix[:length], prefix)
		return
	}
	n.prefixPtr = prefix
}
func (n *node256) findChild(b byte) (node, int16) {
	return n.ChildPtr[b], int16(b)

}
func (n *node256) getType() nodeType {
	return nodeType256
}
func (n *node256) replaceChild(idx uint8, child node) {
	n.ChildPtr[idx] = child
}
func (n *node256) isFull() bool {
	return false
}
func (n *node256) getPrefix() []byte {
	if n.prefixLen > MaxInlinePrefixLength {
		return n.prefixPtr
	}
	return n.prefix[:n.prefixLen]
}
func (n *node256) addChild(b byte, child node) {
	n.ChildPtr[b] = child
}
func (n *node256) grow() node {
	return nil
}
func (n *node256) version() *atomic.Uint64 {
	return n.versionLockObsolete
}

// helper function
func loadKey(n node) []byte {
	l := n.(*leaf)
	return l.key
}
func checkPrefix(prefix []byte, key []byte, depth int) int {
	length := 0
	for length = 0; length < len(prefix); length++ {
		if length+depth >= len(key) || key[length+depth] != prefix[length] {
			break
		}
	}
	return length
}
func getCommonPrefix(s1 []byte, s2 []byte, depth int) []byte {
	minLen := min(len(s1), len(s2))
	for i := depth; i < minLen; i++ {
		if s1[i] != s2[i] {
			return s1[depth:i]
		}
	}
	return s1[depth:minLen]
}
func addChild(parent node, child node, key []byte, pos int) {
	if pos >= len(key) {
		parent.addChild(TerminationChar, child)
	} else {
		parent.addChild(key[pos], child)
	}
}
func findChild(n node, key []byte, depth int) (node, int16) {
	if depth >= len(key) {
		return n.findChild(TerminationChar)
	}
	return n.findChild(key[depth])
}
func readLockOrRestart(n node) (uint64, bool) {
	ver := awaitNodeUnlocked(n)
	//if obsolete try to restart
	if ver&1 == 1 {
		return 0, true
	}
	return ver, false

}
func validate(n node, version uint64) bool {
	//atomic operation
	ver := n.version().Load()
	return ver == version
}
func writeUnlock(n node) {
	n.version().Add(2)
}
func writeUnlockObsolete(n node) {
	// set obsolete, reset locked, overflow into version
	n.version().Add(3)
}
func upgradeToWriteLockOrRestart(n node, version uint64) bool {
	return !n.version().CompareAndSwap(version, setLockedBit(version))
}
func writeLockOrRestart(n node) bool {
	for {
		version, needToRestart := readLockOrRestart(n)
		if needToRestart {
			return true
		}
		if upgradeToWriteLockOrRestart(n, version) {
			return true
		} else {
			break
		}

	}
	return false
}
func setLockedBit(version uint64) uint64 {
	return version + 2
}

// there is improvement in here todo
func awaitNodeUnlocked(n node) uint64 {
	version := n.version().Load()
	spinCount := 0
	for (version & 2) == 2 {
		if spinCount < 10 {
			spinCount++
		} else {
			runtime.Gosched() // yield
		}
		version = n.version().Load()
	}
	return version
}
