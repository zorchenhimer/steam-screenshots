package steamscreenshots

import (
	"sync"
)

type DataTree struct {
	data map[string][]string
	lock *sync.RWMutex
}

func NewDataTree() *DataTree {
	return &DataTree{
		//data: map[string][string][]string{},
		lock: &sync.RWMutex{},
	}
}

func (dt *DataTree) GetKeys() []string {
	dt.lock.RLock()
	defer dt.lock.RUnlock()

	// Shouldn't ever happen, but handle it anyway.
	if dt.data == nil {
		return nil
	}

	keys := []string{}
	for k, _ := range dt.data {
		keys = append(keys, k)
	}

	return keys
}

func (dt *DataTree) Update(newData map[string][]string) {
	dt.lock.Lock()
	defer dt.lock.Unlock()

	dt.data = newData
}
