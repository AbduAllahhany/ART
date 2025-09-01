package art

// todo
// 1) unit test
// 2) implement SIMD
// 3) make it threadsafe
const TerminationChar = '\xff'

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
		newNode.prefix = getCommonPrefix(key, key2, depth)
		depth += len(newNode.prefix)
		addChild(&newNode, n, key2, depth)
		addChild(&newNode, l, key, depth)
		return &newNode
	}
	curPrefix := n.getPrefix()
	p := checkPrefix(n, key, depth)
	if p != len(curPrefix) { // prefix mismatch
		newNode := node4{
			keys:          [4]byte{},
			childPtr:      [4]node{},
			numOfChildren: 0,
		}
		addChild(&newNode, l, key, depth+p)
		addChild(&newNode, n, curPrefix, p)
		newNode.prefix = curPrefix[:p]
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
	next, _ = findChild(n, key, depth)
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
	setPrefix(s string)
}

type leaf struct {
	key string
	val interface{}
}

func (l *leaf) setPrefix(s string) {
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

func (n *node4) setPrefix(s string) {
	n.prefix = s
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

func (n *node16) setPrefix(s string) {
	n.prefix = s
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

func (n *node48) setPrefix(s string) {
	n.prefix = s
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

func (n *node256) setPrefix(s string) {
	n.prefix = s
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
		if length+depth >= len(key) || key[length+depth] != prefix[length] {
			break
		}
	}
	return length
}
func getCommonPrefix(s1 string, s2 string, depth int) string {
	minLen := min(len(s1), len(s2))
	for i := depth; i < minLen; i++ {
		if s1[i] != s2[i] {
			return s1[depth:i]
		}
	}
	return s1[depth:minLen]
}
func addChild(parent node, child node, key string, pos int) {
	if pos >= len(key) {
		parent.addChild(TerminationChar, child)
	} else {
		parent.addChild(key[pos], child)
	}
}
func findChild(n node, key string, depth int) (node, int16) {
	if depth >= len(key) {
		return n.findChild(TerminationChar)
	}
	return n.findChild(key[depth])
}
