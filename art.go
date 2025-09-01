package main

//todo
//1) unit test
//2) implement SIMD
//3) make it threadsafe

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

func insert(n node, key string, l *leaf, depth int) node {
	if n == nil {
		return l
	}
	if n.getType() == nodeTypeLeaf {
		newNode := node4{
			keys:     [4]byte{},
			childPtr: [4]node{},
		}
		key2 := loadKey(n)
		for i := depth; i < min(len(key2), len(key)) && key[i] == key2[i]; i++ {
			newNode.prefix += string(key[i])
		}
		depth += len(newNode.prefix)
		newNode.addChild(key[depth], l)
		if depth >= len(key2) {
			newNode.addChild('\n', n)
		} else {
			newNode.addChild(key2[depth], n)
		}
		return &newNode
	}
	p := checkPrefix(n, key, depth)
	if p != len(n.getPrefix()) { // prefix mismatch
		newNode := node4{
			keys:     [4]byte{},
			childPtr: [4]node{},
		}
		newNode.addChild(key[depth+p], l)
		newNode.addChild(n.getPrefix()[p], n)
		newNode.prefix = n.getPrefix()[:p+1]
		return &newNode
	}
	depth += len(n.getPrefix())
	next, idx := n.findChild(key[depth])
	if next != nil {
		updated := insert(next, key, l, depth)
		n.replaceChild(uint8(idx), updated)
	} else {
		if n.isFull() {
			n = n.grow()
		}
		n.addChild(key[depth], l)
	}
	return n
}
func search(n node, key string, depth int) (interface{}, bool) {
	if n == nil {
		return nil, false
	}
	if n.getType() == nodeTypeLeaf {
		l := n.(*leaf)
		if l.key == key {
			return l.val, true
		}
		return nil, false
	}
	p := checkPrefix(n, key, depth)
	pre := n.getPrefix()
	l := len(pre)
	if p != l {
		return nil, false
	}
	depth += len(n.getPrefix())
	var next node
	if depth >= len(key) {
		next, _ = n.findChild('\n')
	} else {
		next, _ = n.findChild(key[depth])
	}
	return search(next, key, depth)
}
func (t *Tree) Insert(key string, val interface{}) {
	l := leaf{
		key: key,
		val: val,
	}
	t.node = insert(t.node, key, &l, 0)
}
func (t *Tree) Search(key string) (interface{}, bool) {
	return search(t.node, key, 0)
}

type node interface {
	getType() nodeType
	findChild(b byte) (node, int16)
	replaceChild(uint8, node)
	isFull() bool
	getPrefix() string
	addChild(k byte, child node)
	grow() node
}

type leaf struct {
	key string
	val interface{}
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
func (l *leaf) getPrefix() string {
	return ""
}
func (l *leaf) addChild(k byte, child node) {
	return
}

type node4 struct {
	keys          [4]uint8
	childPtr      [4]node
	numOfChildren uint8
	prefix        string
}

func (n *node4) grow() node {
	newNode := node16{
		keys:          [16]uint8{},
		childPtr:      [16]node{},
		prefix:        n.prefix,
		numOfChildren: n.numOfChildren,
	}
	copy(newNode.keys[:], n.keys[:])
	copy(newNode.childPtr[:], n.childPtr[:])
	return &newNode
}
func (n *node4) getPrefix() string {
	return n.prefix
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

type node16 struct {
	keys          [16]uint8
	childPtr      [16]node
	numOfChildren uint8
	prefix        string
}

func (n *node16) getType() nodeType {
	return nodeType16
}
func (n *node16) findChild(b byte) (node, int16) {
	//todo use SIMD
	for i, k := range n.keys {
		if k == byte(b) {
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
func (n *node16) getPrefix() string {
	return n.prefix
}
func (n *node16) addChild(k byte, child node) {
	n.keys[n.numOfChildren] = byte(k)
	n.childPtr[n.numOfChildren] = child
	n.numOfChildren++
}
func (n *node16) grow() node {
	newNode := node48{
		childIndex:    [256]byte{},
		childPtr:      [48]node{},
		numOfChildren: n.numOfChildren,
		prefix:        "",
	}
	for i := 0; i < 16; i++ {
		newNode.childPtr[i] = n.childPtr[i]
		newNode.childIndex[n.keys[i]] = byte(i)
	}
	return &newNode
}

type node48 struct {
	childIndex    [256]uint8
	childPtr      [48]node
	numOfChildren byte
	prefix        string
}

func (n *node48) getType() nodeType {
	return nodeType48
}
func (n *node48) findChild(b byte) (node, int16) {
	if n.childIndex[b] != 0 {
		return n.childPtr[n.childIndex[b]], int16(n.childIndex[b])
	}
	return nil, -1
}
func (n *node48) addChild(b byte, child node) {
	n.childIndex[b] = n.numOfChildren
	n.childPtr[n.numOfChildren] = child
	n.numOfChildren++
}
func (n *node48) replaceChild(idx uint8, child node) {
	n.childPtr[idx] = child
}
func (n *node48) isFull() bool {
	return n.numOfChildren == 48
}
func (n *node48) getPrefix() string {
	return n.prefix
}
func (n *node48) grow() node {
	newNode := node256{
		ChildPtr: [256]node{},
		prefix:   n.prefix,
	}
	for i, _ := range n.childPtr {
		newNode.ChildPtr[n.childIndex[i]] = n.childPtr[i]
	}
	return &newNode
}

type node256 struct {
	ChildPtr [256]node
	prefix   string
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
func (n *node256) getPrefix() string {
	return n.prefix
}
func (n *node256) addChild(b byte, child node) {
	n.ChildPtr[b] = child
}
func (n *node256) grow() node {
	return nil
}

// helper
func loadKey(n node) string {
	l := n.(*leaf)
	return l.key
}
func checkPrefix(n node, key string, depth int) int {
	prefix := n.getPrefix()
	length := 0
	for length = 0; length < len(prefix); length++ {
		if key[length+depth] != prefix[length] {
			break
		}
	}
	return length
}
