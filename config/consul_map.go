package config

import (
	"container/list"
	"sync"
)

type Map[T any] struct {
	mp              map[string]T
	changeListeners *list.List
	mutex           *sync.RWMutex
}

func NewMap[T any]() *Map[T] {
	return &Map[T]{
		mp:              make(map[string]T),
		changeListeners: list.New(),
		mutex:           &sync.RWMutex{},
	}
}

func (cache *Map[T]) Add(id string, item T) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	cache.mp[id] = item
}

func (cache *Map[T]) Get(id string) (T, bool) {
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()
	item, ok := cache.mp[id]
	return item, ok
}

func (cache *Map[T]) AddAll(itemMap map[string]T) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	for k, v := range itemMap {
		cache.mp[k] = v
	}
}

func (cache *Map[T]) ResetAll(itemMap map[string]T) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	for k := range cache.mp {
		delete(cache.mp, k)
	}
	for k, v := range itemMap {
		cache.mp[k] = v
	}
}

func (cache *Map[T]) ClearAll() {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	for k := range cache.mp {
		delete(cache.mp, k)
	}
}

func (cache *Map[T]) Clear(id string) {
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	delete(cache.mp, id)
}

func (cache *Map[T]) ToMap() map[string]T {
	return cache.mp
}

func HasKey[T any](dict map[string]T, key string) bool {
	if dict == nil {
		return false
	}
	_, ok := dict[key]
	return ok
}

// ChangeListener 监听器
type ChangeListener interface {
	//OnChange 增加变更监控
	OnChange(changes map[string]*ConfigChange)
}

// AddChangeListener 增加变更监控
func (cache *Map[T]) AddChangeListener(listener ChangeListener) {
	if listener == nil {
		return
	}
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	cache.changeListeners.PushBack(listener)
}

// RemoveChangeListener 删除变更监控
func (cache *Map[T]) RemoveChangeListener(listener ChangeListener) {
	if listener == nil {
		return
	}
	cache.mutex.Lock()
	defer cache.mutex.Unlock()
	for i := cache.changeListeners.Front(); i != nil; i = i.Next() {
		apolloListener := i.Value.(ChangeListener)
		if listener == apolloListener {
			cache.changeListeners.Remove(i)
		}
	}
}

// GetChangeListeners 获取配置修改监听器列表
func (cache *Map[T]) GetChangeListeners() *list.List {
	if cache.changeListeners == nil {
		return nil
	}
	cache.mutex.RLock()
	defer cache.mutex.RUnlock()
	l := list.New()
	l.PushBackList(cache.changeListeners)
	return l
}

// push config change event
func (cache *Map[T]) pushChangeEvent(changes map[string]*ConfigChange) {
	cache.pushChange(func(listener ChangeListener) {
		go listener.OnChange(changes)
	})
}

func (cache *Map[T]) pushChange(f func(ChangeListener)) {
	// if channel is null ,mean no listener,don't need to push msg
	listeners := cache.GetChangeListeners()
	if listeners == nil || listeners.Len() == 0 {
		return
	}

	for i := listeners.Front(); i != nil; i = i.Next() {
		listener := i.Value.(ChangeListener)
		f(listener)
	}
}

type ConfigChange struct {
	OldValue   []byte
	NewValue   []byte
	ChangeType ConfigChangeType
}

// config change type
type ConfigChangeType int

const (
	ADDED ConfigChangeType = iota
	MODIFIED
	DELETED
)

// create modify config change
func createModifyConfigChange(oldValue []byte, newValue []byte) *ConfigChange {
	return &ConfigChange{
		OldValue:   oldValue,
		NewValue:   newValue,
		ChangeType: MODIFIED,
	}
}

// create add config change
func createAddConfigChange(newValue []byte) *ConfigChange {
	return &ConfigChange{
		NewValue:   newValue,
		ChangeType: ADDED,
	}
}

// create delete config change
func createDeletedConfigChange(oldValue []byte) *ConfigChange {
	return &ConfigChange{
		OldValue:   oldValue,
		ChangeType: DELETED,
	}
}
