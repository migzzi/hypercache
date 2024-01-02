package hypercache

import "sync"

type dlListNode[T any] struct {
	next, prev *dlListNode[T]
	value      T
}

type dlList[T any] struct {
	head, tail *dlListNode[T]
	mu         sync.Mutex
}

func newDLList[T any]() *dlList[T] {
	return &dlList[T]{
		mu: sync.Mutex{},
	}
}

func (l *dlList[T]) pushFront(v T) *dlListNode[T] {
	l.mu.Lock()
	defer l.mu.Unlock()
	node := &dlListNode[T]{}
	node.value = v
	if l.head == nil {
		l.head = node
		l.tail = node
	} else {
		node.next = l.head
		l.head.prev = node
		l.head = node
	}
	return node
}

func (l *dlList[T]) pushBack(v T) *dlListNode[T] {
	l.mu.Lock()
	defer l.mu.Unlock()
	node := &dlListNode[T]{}
	node.value = v
	if l.tail == nil {
		l.head = node
		l.tail = node
	} else {
		node.prev = l.tail
		l.tail.next = node
		l.tail = node
	}
	return node
}

func (l *dlList[T]) remove(node *dlListNode[T]) {
	if node == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l._removeNode(node)
}

func (l *dlList[T]) _removeNode(node *dlListNode[T]) {
	if node.prev == nil {
		l.head = node.next
	} else {
		node.prev.next = node.next
	}
	if node.next == nil {
		l.tail = node.prev
	} else {
		node.next.prev = node.prev
	}
}

func (l *dlList[T]) popBack() T {
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.tail == nil {
		var v T
		return v
	}
	v := l.tail.value
	l.tail = l.tail.prev
	if l.tail == nil {
		l.head = nil
	}
	return v
}

func (l *dlList[T]) moveToFront(node *dlListNode[T]) {
	if node == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	l._removeNode(node)
	if l.head == nil {
		l.head = node
		l.tail = node
		return
	}
	l.head.prev = node
	node.next = l.head
	l.head = node
}
