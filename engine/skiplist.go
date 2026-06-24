package engine

import (
	"fmt"
	"math"
	"math/rand"
	"sync"
)

// OrderNode 订单节点
type OrderNode struct {
	Order *Order
}

// SkipListNode 跳表节点
type SkipListNode struct {
	Key     float64         // 价格
	Score   int64           // 优先级（时间戳）
	Value   interface{}     // 值
	Level   int             // 层数
	Forward []*SkipListNode // 前进指针
}

// SkipList 跳表实现
type SkipList struct {
	Header   *SkipListNode
	Level    int  // 当前层数
	Size     int  // 元素数量
	MaxLevel int  // 最大层数
	Reverse  bool // 是否反向排序

	mu sync.RWMutex
}

const (
	MaxLevel = 16
	p        = 0.5 // 概率
)

// 全局随机数生成器（减少锁竞争）
var globalRand = rand.New(rand.NewSource(0))

// init 初始化随机种子
func init() {
	// 使用时间戳初始化随机种子
	globalRand = rand.New(rand.NewSource(0))
}

// fastRandomLevel 高性能随机层数生成（使用概率）
func fastRandomLevel() int {
	// 使用位运算快速判断
	level := 1
	// 16层最大，检查前8位足够
	if rand.Int()&0x80 != 0 { // 50% 概率进入下一层
		level++
		if rand.Int()&0x80 != 0 {
			level++
			if rand.Int()&0x80 != 0 {
				level++
				if rand.Int()&0x80 != 0 {
					level++
				}
			}
		}
	}
	return level
}

// NewSkipList 创建跳表
func NewSkipList(reverse bool) *SkipList {
	header := &SkipListNode{
		Level:   MaxLevel,
		Forward: make([]*SkipListNode, MaxLevel),
	}

	return &SkipList{
		Header:   header,
		Level:    1,
		MaxLevel: MaxLevel,
		Reverse:  reverse,
	}
}

// randomLevel 生成随机层数
func (s *SkipList) randomLevel() int {
	return fastRandomLevel()
}

// compare 比较两个键
func (s *SkipList) compare(aKey, aScore, bKey, bScore float64) int {
	var diff float64
	if s.Reverse {
		diff = aKey - bKey
	} else {
		diff = bKey - aKey
	}

	if math.Abs(diff) > 0.00000001 {
		if diff > 0 {
			return 1
		}
		return -1
	}

	// 价格相同，按时间排序（先到先成交，时间戳小的排在前面）
	if aScore < bScore {
		return 1
	}
	if aScore > bScore {
		return -1
	}
	return 0
}

// Insert 插入元素
func (s *SkipList) Insert(key float64, score int64, value interface{}) *SkipListNode {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 记录每一层的前驱节点
	update := make([]*SkipListNode, s.MaxLevel)

	// 找到每个层级的前驱节点
	x := s.Header
	for i := s.Level - 1; i >= 0; i-- {
		for x.Forward[i] != nil {
			cmp := s.compare(key, float64(score), x.Forward[i].Key, float64(x.Forward[i].Score))
			if cmp > 0 {
				x = x.Forward[i]
			} else {
				break
			}
		}
		update[i] = x
	}

	// 生成随机层数
	level := s.randomLevel()

	// 创建新节点
	node := &SkipListNode{
		Key:     key,
		Score:   score,
		Value:   value,
		Level:   level,
		Forward: make([]*SkipListNode, level),
	}

	// 插入节点
	for i := 0; i < level; i++ {
		node.Forward[i] = update[i].Forward[i]
		update[i].Forward[i] = node
	}

	// 更新层数
	if level > s.Level {
		s.Level = level
	}

	s.Size++
	return node
}

// Search 搜索元素
func (s *SkipList) Search(key float64, score int64) *SkipListNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	x := s.Header
	for i := s.Level - 1; i >= 0; i-- {
		for x.Forward[i] != nil {
			cmp := s.compare(key, float64(score), x.Forward[i].Key, float64(x.Forward[i].Score))
			if cmp > 0 {
				x = x.Forward[i]
			} else if cmp == 0 {
				return x.Forward[i]
			} else {
				break
			}
		}
	}

	return nil
}

// Delete 删除元素
func (s *SkipList) Delete(key float64, score int64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	update := make([]*SkipListNode, s.MaxLevel)

	x := s.Header
	for i := s.Level - 1; i >= 0; i-- {
		for x.Forward[i] != nil {
			cmp := s.compare(key, float64(score), x.Forward[i].Key, float64(x.Forward[i].Score))
			if cmp > 0 {
				x = x.Forward[i]
			} else {
				break
			}
		}
		update[i] = x
	}

	x = x.Forward[0]
	if x != nil && math.Abs(x.Key-key) < 0.00000001 && x.Score == score {
		for i := 0; i < s.Level; i++ {
			if update[i].Forward[i] != x {
				break
			}
			update[i].Forward[i] = x.Forward[i]
		}

		// 更新层数
		for s.Level > 1 && s.Header.Forward[s.Level-1] == nil {
			s.Level--
		}

		s.Size--
		return true
	}

	return false
}

// Min 获取最小元素
func (s *SkipList) Min() *SkipListNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Size == 0 {
		return nil
	}

	return s.Header.Forward[0]
}

// Max 获取最大元素
func (s *SkipList) Max() *SkipListNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if s.Size == 0 {
		return nil
	}

	x := s.Header
	for i := s.Level - 1; i >= 0; i-- {
		for x.Forward[i] != nil {
			x = x.Forward[i]
		}
	}

	return x
}

// Range 获取范围内的元素
func (s *SkipList) Range(minKey, maxKey *float64, limit int) []*SkipListNode {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*SkipListNode

	x := s.Header
	for i := s.Level - 1; i >= 0; i-- {
		for x.Forward[i] != nil {
			if minKey != nil {
				cmp := s.compare(x.Forward[i].Key, float64(x.Forward[i].Score), *minKey, 0)
				if cmp < 0 {
					x = x.Forward[i]
					continue
				}
			}
			break
		}
	}

	x = x.Forward[0]
	for x != nil && len(result) < limit {
		if maxKey != nil {
			cmp := s.compare(x.Key, float64(x.Score), *maxKey, 0)
			if cmp > 0 {
				break
			}
		}
		result = append(result, x)
		x = x.Forward[0]
	}

	return result
}

// String 打印跳表
func (s *SkipList) String() string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result string
	result += fmt.Sprintf("SkipList(size=%d, level=%d, reverse=%v):\n", s.Size, s.Level, s.Reverse)

	for i := s.Level - 1; i >= 0; i-- {
		result += fmt.Sprintf("Level %d: ", i)
		x := s.Header.Forward[i]
		for x != nil {
			result += fmt.Sprintf("[%.4f, %d] -> ", x.Key, x.Score)
			x = x.Forward[i]
		}
		result += "nil\n"
	}

	return result
}

// RedBlackTree 红黑树实现 - 用于快速查找和删除
type RedBlackTree struct {
	root *RBNode
	size int
	mu   sync.RWMutex
}

type RBColor bool

const (
	RBRed   RBColor = true
	RBBlack RBColor = false
)

type RBNode struct {
	Key    float64
	Value  interface{}
	Color  RBColor
	Left   *RBNode
	Right  *RBNode
	Parent *RBNode
}

func NewRedBlackTree() *RedBlackTree {
	return &RedBlackTree{}
}

func (t *RedBlackTree) leftRotate(x *RBNode) {
	y := x.Right
	x.Right = y.Left
	if y.Left != nil {
		y.Left.Parent = x
	}
	y.Parent = x.Parent
	if x.Parent == nil {
		t.root = y
	} else if x == x.Parent.Left {
		x.Parent.Left = y
	} else {
		x.Parent.Right = y
	}
	y.Left = x
	x.Parent = y
}

func (t *RedBlackTree) rightRotate(y *RBNode) {
	x := y.Left
	y.Left = x.Right
	if x.Right != nil {
		x.Right.Parent = y
	}
	x.Parent = y.Parent
	if y.Parent == nil {
		t.root = x
	} else if y == y.Parent.Right {
		y.Parent.Right = x
	} else {
		y.Parent.Left = x
	}
	x.Right = y
	y.Parent = x
}

func (t *RedBlackTree) Insert(key float64, value interface{}) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var y *RBNode
	x := t.root

	node := &RBNode{
		Key:   key,
		Value: value,
		Color: RBRed,
	}

	for x != nil {
		y = x
		if key < x.Key {
			x = x.Left
		} else {
			x = x.Right
		}
	}

	node.Parent = y
	if y == nil {
		t.root = node
	} else if key < y.Key {
		y.Left = node
	} else {
		y.Right = node
	}

	t.insertFixup(node)
	t.size++
}

func (t *RedBlackTree) insertFixup(z *RBNode) {
	for z.Parent != nil && z.Parent.Color == RBRed {
		if z.Parent == z.Parent.Parent.Left {
			y := z.Parent.Parent.Right
			if y != nil && y.Color == RBRed {
				z.Parent.Color = RBBlack
				y.Color = RBBlack
				z.Parent.Parent.Color = RBRed
				z = z.Parent.Parent
			} else {
				if z == z.Parent.Right {
					z = z.Parent
					t.leftRotate(z)
				}
				z.Parent.Color = RBBlack
				z.Parent.Parent.Color = RBRed
				t.rightRotate(z.Parent.Parent)
			}
		} else {
			y := z.Parent.Parent.Left
			if y != nil && y.Color == RBRed {
				z.Parent.Color = RBBlack
				y.Color = RBBlack
				z.Parent.Parent.Color = RBRed
				z = z.Parent.Parent
			} else {
				if z == z.Parent.Left {
					z = z.Parent
					t.rightRotate(z)
				}
				z.Parent.Color = RBBlack
				z.Parent.Parent.Color = RBRed
				t.leftRotate(z.Parent.Parent)
			}
		}
	}
	t.root.Color = RBBlack
}

func (t *RedBlackTree) Delete(key float64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	z := t.searchNode(t.root, key)
	if z == nil {
		return false
	}

	t.deleteNode(z)
	t.size--
	return true
}

func (t *RedBlackTree) searchNode(node *RBNode, key float64) *RBNode {
	if node == nil || math.Abs(node.Key-key) < 0.00000001 {
		return node
	}
	if key < node.Key {
		return t.searchNode(node.Left, key)
	}
	return t.searchNode(node.Right, key)
}

func (t *RedBlackTree) deleteNode(z *RBNode) {
	y := z
	yOriginalColor := y.Color
	var x *RBNode

	if z.Left == nil {
		x = z.Right
		t.transplant(z, z.Right)
	} else if z.Right == nil {
		x = z.Left
		t.transplant(z, z.Left)
	} else {
		y = t.minimum(z.Right)
		yOriginalColor = y.Color
		x = y.Right
		if y.Parent == z {
			if x != nil {
				x.Parent = y
			}
		} else {
			t.transplant(y, y.Right)
			y.Right = z.Right
			y.Right.Parent = y
		}
		t.transplant(z, y)
		y.Left = z.Left
		y.Left.Parent = y
		y.Color = z.Color
	}

	if yOriginalColor == RBBlack {
		t.deleteFixup(x)
	}
}

func (t *RedBlackTree) transplant(u, v *RBNode) {
	if u.Parent == nil {
		t.root = v
	} else if u == u.Parent.Left {
		u.Parent.Left = v
	} else {
		u.Parent.Right = v
	}
	if v != nil {
		v.Parent = u.Parent
	}
}

func (t *RedBlackTree) minimum(node *RBNode) *RBNode {
	for node.Left != nil {
		node = node.Left
	}
	return node
}

func (t *RedBlackTree) deleteFixup(x *RBNode) {
	for x != t.root && (x == nil || x.Color == RBBlack) {
		if x == x.Parent.Left {
			w := x.Parent.Right
			if w != nil && w.Color == RBRed {
				w.Color = RBBlack
				x.Parent.Color = RBRed
				t.leftRotate(x.Parent)
				w = x.Parent.Right
			}
			if w != nil && (w.Left == nil || w.Left.Color == RBBlack) && (w.Right == nil || w.Right.Color == RBBlack) {
				w.Color = RBRed
				x = x.Parent
			} else if w != nil {
				if w.Right == nil || w.Right.Color == RBBlack {
					if w.Left != nil {
						w.Left.Color = RBBlack
					}
					w.Color = RBRed
					t.rightRotate(w)
					w = x.Parent.Right
				}
				if w != nil {
					w.Color = x.Parent.Color
					x.Parent.Color = RBBlack
					if w.Right != nil {
						w.Right.Color = RBBlack
					}
					t.leftRotate(x.Parent)
				}
				x = t.root
			}
		} else {
			w := x.Parent.Left
			if w != nil && w.Color == RBRed {
				w.Color = RBBlack
				x.Parent.Color = RBRed
				t.rightRotate(x.Parent)
				w = x.Parent.Left
			}
			if w != nil && (w.Left == nil || w.Left.Color == RBBlack) && (w.Right == nil || w.Right.Color == RBBlack) {
				w.Color = RBRed
				x = x.Parent
			} else if w != nil {
				if w.Left == nil || w.Left.Color == RBBlack {
					if w.Right != nil {
						w.Right.Color = RBBlack
					}
					w.Color = RBRed
					t.leftRotate(w)
					w = x.Parent.Left
				}
				if w != nil {
					w.Color = x.Parent.Color
					x.Parent.Color = RBBlack
					if w.Left != nil {
						w.Left.Color = RBBlack
					}
					t.rightRotate(x.Parent)
				}
				x = t.root
			}
		}
	}
	if x != nil {
		x.Color = RBBlack
	}
}

func (t *RedBlackTree) Search(key float64) interface{} {
	t.mu.RLock()
	defer t.mu.RUnlock()

	node := t.searchNode(t.root, key)
	if node != nil {
		return node.Value
	}
	return nil
}

func (t *RedBlackTree) Size() int {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.size
}

// LinkedListOrderQueue 链表实现的订单队列（按时间排序）
type LinkedListOrderQueue struct {
	head *ListNode
	tail *ListNode
	size int
	mu   sync.Mutex
}

type ListNode struct {
	Order *Order
	Prev  *ListNode
	Next  *ListNode
}

func NewLinkedListOrderQueue() *LinkedListOrderQueue {
	head := &ListNode{}
	tail := &ListNode{}
	head.Next = tail
	tail.Prev = head
	return &LinkedListOrderQueue{head: head, tail: tail}
}

func (q *LinkedListOrderQueue) Enqueue(order *Order) {
	node := &ListNode{Order: order}

	q.mu.Lock()
	defer q.mu.Unlock()

	// 插入到队列尾部
	node.Prev = q.tail.Prev
	node.Next = q.tail
	q.tail.Prev.Next = node
	q.tail.Prev = node
	q.size++
}

func (q *LinkedListOrderQueue) Dequeue() *Order {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.size == 0 {
		return nil
	}

	node := q.head.Next
	q.head.Next = node.Next
	node.Next.Prev = q.head
	q.size--

	return node.Order
}

func (q *LinkedListOrderQueue) Remove(order *Order) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	for node := q.head.Next; node != q.tail; node = node.Next {
		if node.Order == order {
			node.Prev.Next = node.Next
			node.Next.Prev = node.Prev
			q.size--
			return true
		}
	}
	return false
}

func (q *LinkedListOrderQueue) Size() int {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.size
}

func (q *LinkedListOrderQueue) IsEmpty() bool {
	return q.Size() == 0
}
