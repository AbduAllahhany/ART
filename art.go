package art

import (
	"bytes"
	"runtime"
	"sync"
	"sync/atomic"
)

// todo
// 1) Implement SIMD
// 2) Make it threadsafe
// 3) Combine ART with a Bloom filter for ultra-fast negative lookups.
// 4) Improve performance after the OLC shit
const TerminationChar = '\xff'
const MaxInlinePrefixLength = 8
const (
	OBSOLETE_BIT   = uint64(1)
	LOCK_BIT       = uint64(1 << 1)
	LOCK_INCREMENT = uint64(2)
)

type nodeType int

const (
	nodeTypeLeaf nodeType = iota
	nodeType4
	nodeType16
	nodeType48
	nodeType256
)

type Tree struct {
	node     node
	rootLock sync.Mutex
}

func NewART() Tree {
	return Tree{
		node: nil,
	}
}

func (t *Tree) insert(key []byte, l *leaf, depth int, parent node, parentVersion uint64) {
restart:
	parent = nil
	parentVersion = 0
	depth = 0
	curNodeAddress := &t.node
	for {
		version, needToRestart := readLockOrRestart(*curNodeAddress)
		if needToRestart {
			goto restart
		}
		curNode := *curNodeAddress
		if curNode == nil {
			needToRestart = upgradeToWriteLockOrRestart(parent, parentVersion)
			if needToRestart {
				goto restart
			}
			*curNodeAddress = l
			writeUnlock(parent)
			break
		}
		if curNode.getType() == nodeTypeLeaf {
			needToRestart = upgradeToWriteLockOrRestart(parent, parentVersion)
			if needToRestart {
				goto restart
			}
			needToRestart = upgradeToWriteLockOrRestart(curNode, version)
			if needToRestart {
				writeUnlock(parent)
				goto restart
			}
			if bytes.Equal(curNode.(*leaf).key, key) {
				(*curNodeAddress).(*leaf).val = l.val
				writeUnlock(parent)
				writeUnlock(curNode)
				break
			}
			newNode := newNode4Locked()
			key2 := loadKey(curNode)
			commonPrefix := getCommonPrefix(key, key2, depth)
			newNode.setPrefix(commonPrefix)
			depth += int(newNode.prefixLen)
			addChild(newNode, curNode, key2, depth)
			addChild(newNode, l, key, depth)
			*curNodeAddress = newNode

			writeUnlock(parent)
			writeUnlock(curNode)
			writeUnlock(newNode)

			break
		}
		//copy the prefix
		curPrefix := append([]byte(nil), curNode.getPrefix()...)
		p := checkPrefix(curPrefix, key, depth)
		needToRestart = !validate(curNode, version)
		if needToRestart {
			goto restart
		}
		if p != len(curPrefix) { // prefix mismatch
			needToRestart = upgradeToWriteLockOrRestart(parent, parentVersion)
			if needToRestart {
				goto restart
			}
			needToRestart = upgradeToWriteLockOrRestart(curNode, version)
			if needToRestart {
				writeUnlock(parent)
				goto restart
			}
			newNode := newNode4Locked()
			addChild(newNode, l, key, depth+p)
			addChild(newNode, curNode, curPrefix, p)
			newNode.setPrefix(curPrefix[:p])
			curNode.setPrefix(curPrefix[p:])
			*curNodeAddress = newNode
			writeUnlock(parent)
			writeUnlock(curNode)
			writeUnlock(newNode)
			break
		}
		depth += len(curPrefix)
		next := findChild(curNode, key, depth)
		needToRestart = !validate(curNode, version)
		if needToRestart {
			goto restart
		}
		if next == nil {
			needToRestart = upgradeToWriteLockOrRestart(parent, parentVersion)
			if needToRestart {
				goto restart
			}
			needToRestart = upgradeToWriteLockOrRestart(curNode, version)
			if needToRestart {
				writeUnlock(parent)
				goto restart
			}
			if curNode.isFull() {
				grown := growLocked(curNode)
				addChild(grown, l, key, depth)
				*curNodeAddress = grown
				writeUnlock(parent)
				writeUnlockObsolete(curNode)
				writeUnlock(grown)
			} else {
				addChild(*curNodeAddress, l, key, depth)
				writeUnlock(parent)
				writeUnlock(curNode)
			}
			break
		}
		parent = curNode
		parentVersion = version
		curNodeAddress = next
	}
}

func (t *Tree) search(key []byte, depth int, parent node, parentVersion uint64) (interface{}, bool) {
restart:
	curNodeAddress := &t.node
	parent = nil
	parentVersion = 0
	depth = 0
	for {
		if curNodeAddress == nil || *curNodeAddress == nil {
			return nil, false
		}
		version, needToRestart := readLockOrRestart(*curNodeAddress)
		if needToRestart {
			goto restart
		}
		curNode := *curNodeAddress
		if curNode == nil {
			return nil, false
		}
		if curNode.getType() == nodeTypeLeaf {
			curLeaf := curNode.(*leaf)
			needToRestart = !validate(curNode, version)
			if needToRestart {
				goto restart
			}
			if bytes.Equal(curLeaf.key, key) {
				return curLeaf.val, true
			}
			return nil, false
		}
		pre := curNode.getPrefix()
		p := checkPrefix(pre, key, depth)
		l := len(pre)
		if p != l {
			needToRestart = !validate(curNode, version)
			if needToRestart {
				goto restart
			}
			return nil, false
		}
		depth += len(curNode.getPrefix())
		nextAdd := findChild(curNode, key, depth)
		needToRestart = !validate(curNode, version)
		if needToRestart {
			goto restart
		}
		if nextAdd != nil {
			parent = curNode
			parentVersion = version
			curNodeAddress = nextAdd
		} else {
			needToRestart = !validate(curNode, version)
			if needToRestart {
				goto restart
			}
			break
		}
	}
	return nil, false
}
func (t *Tree) Insert(key []byte, val interface{}) {
	l := &leaf{
		key:                 key,
		val:                 val,
		versionLockObsolete: &atomic.Uint64{},
	}

	t.insert(key, l, 0, nil, 0)
}
func (t *Tree) Search(key []byte) (interface{}, bool) {
	return t.search(key, 0, nil, 0)
}

type node interface {
	getType() nodeType
	findChild(b byte) *node
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
func (l *leaf) findChild(b byte) *node {
	return nil
}
func (l *leaf) grow() node {
	return nil
}
func (l *leaf) getType() nodeType {
	return nodeTypeLeaf
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
func (l *leaf) version() *atomic.Uint64 {
	return l.versionLockObsolete
}

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
		n.prefix = [8]byte{}
		copy(n.prefix[:length], prefix)
		return
	}
	n.prefixPtr = prefix
}
func (n *node4) grow() node {
	newNode := &node16{
		childPtr:            [16]node{},
		prefixPtr:           n.prefixPtr,
		keys:                [16]uint8{},
		prefix:              n.prefix,
		prefixLen:           n.prefixLen,
		numOfChildren:       n.numOfChildren,
		versionLockObsolete: &atomic.Uint64{},
	}

	copy(newNode.keys[:], n.keys[:])
	copy(newNode.childPtr[:], n.childPtr[:])
	return newNode
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
func (n *node4) findChild(b byte) *node {
	//simple search over keys
	for i, k := range n.keys {
		if k == b {
			return &n.childPtr[i]
		}
	}
	return nil
}
func (n *node4) addChild(k byte, child node) {
	n.keys[n.numOfChildren] = k
	n.childPtr[n.numOfChildren] = child
	n.numOfChildren++
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

func (n *node16) setPrefix(pre []byte) {
	length := len(pre)
	n.prefixLen = uint16(length)
	if length <= MaxInlinePrefixLength {
		n.prefix = [8]byte{}
		for i := 0; i < length; i++ {
			n.prefix[i] = pre[i]
		}
		return
	}
	n.prefixPtr = pre
}
func (n *node16) getType() nodeType {
	return nodeType16
}
func (n *node16) findChild(b byte) *node {
	//todo use SIMD
	for i, k := range n.keys {
		if k == b {
			return &n.childPtr[i]
		}
	}
	return nil

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
	n.keys[n.numOfChildren] = k
	n.childPtr[n.numOfChildren] = child
	n.numOfChildren++
}
func (n *node16) grow() node {
	var idxArr [256]int16
	for i := 0; i < 256; i++ {
		idxArr[i] = -1
	}
	newNode := node48{
		childPtr:            [48]node{},
		prefixPtr:           n.prefixPtr,
		childIndex:          idxArr,
		prefix:              n.prefix,
		prefixLen:           n.prefixLen,
		numOfChildren:       n.numOfChildren,
		versionLockObsolete: &atomic.Uint64{},
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
		n.prefix = [8]byte{}
		copy(n.prefix[:length], prefix)
		return
	}
	n.prefixPtr = prefix
}
func (n *node48) getType() nodeType {
	return nodeType48
}
func (n *node48) findChild(b byte) *node {
	if n.childIndex[b] != -1 {
		return &n.childPtr[n.childIndex[b]]
	}
	return nil
}
func (n *node48) addChild(b byte, child node) {
	n.childIndex[b] = int16(n.numOfChildren)
	n.childPtr[n.numOfChildren] = child
	n.numOfChildren++
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
		ChildPtr:            [256]node{},
		prefixPtr:           n.prefixPtr,
		prefixLen:           n.prefixLen,
		prefix:              n.prefix,
		versionLockObsolete: &atomic.Uint64{},
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
		n.prefix = [8]byte{}
		copy(n.prefix[:length], prefix)
		return
	}
	n.prefixPtr = prefix
}
func (n *node256) findChild(b byte) *node {
	if n.ChildPtr[b] != nil {
		return &n.ChildPtr[b]
	}
	return nil

}
func (n *node256) getType() nodeType {
	return nodeType256
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
	if pos >= len(key) || len(key) == 0 {
		parent.addChild(TerminationChar, child)
	} else {
		parent.addChild(key[pos], child)
	}
}
func findChild(n node, key []byte, depth int) *node {
	if depth >= len(key) {
		return n.findChild(TerminationChar)
	}
	return n.findChild(key[depth])
}
func readLockOrRestart(n node) (uint64, bool) {
	if n == nil {
		return 0, false
	}
	ver := awaitNodeUnlocked(n)
	//if obsolete try to restart
	if ver&OBSOLETE_BIT != 0 {
		return 0, true
	}
	return ver, false

}
func validate(n node, version uint64) bool {
	if n == nil {
		return true
	}
	//atomic operation
	ver := n.version().Load()
	return ver == version
}
func writeUnlock(n node) {
	if n == nil {
		return
	}
	n.version().Add(LOCK_INCREMENT)
}
func writeUnlockObsolete(n node) {
	if n == nil {
		return
	}
	// set obsolete bit and bump version in CAS loop
	for {
		v := n.version().Load()
		desired := (v | OBSOLETE_BIT) + LOCK_INCREMENT
		if n.version().CompareAndSwap(v, desired) {
			return
		}
	}
}
func upgradeToWriteLockOrRestart(n node, version uint64) bool {
	if n == nil {
		return false
	}
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
	return version | LOCK_BIT
}

// there is improvement in here todo
func awaitNodeUnlocked(n node) uint64 {
	if n == nil {
		return 0
	}
	version := n.version().Load()
	spinCount := 0
	for (version & LOCK_BIT) == 2 {
		if spinCount < 10 {
			spinCount++
		} else {
			runtime.Gosched() // yield
		}
		version = n.version().Load()
	}
	return version
}
func newNode4Locked() *node4 {
	n := &node4{
		childPtr:            [4]node{},
		prefixPtr:           nil,
		prefix:              [8]byte{},
		keys:                [4]byte{},
		prefixLen:           0,
		numOfChildren:       0,
		versionLockObsolete: &atomic.Uint64{},
	}
	n.version().Store(setLockedBit(0))
	return n
}
func growLocked(n node) node {
	newNode := n.grow()
	newNode.version().Store(setLockedBit(0))
	return newNode
}
