package testdata

import (
	"sync"
)

var mutex *sync.RWMutex

func RLockFuncs() {
	regularRLock()
	nestedRLock1Level()
	nestedRLock2Levels()
	deferredRLock()
	recursiveRLock(true)
}

func regularRLock() {
	mutex.RLock()
	mutex.RUnlock()
}

func nestedRLock1Level() {
	mutex.RLock()
	regularRLock() // want `found recursive read lock call`
	mutex.RUnlock()
}

func nestedRLock2Levels() {
	mutex.RLock()
	callRegularRLock() // want `found recursive read lock call`
	mutex.RUnlock()
}

func callRegularRLock() {
	regularRLock()
}

func deferredRLock() {
	mutex.RLock()
	defer mutex.RUnlock()
	regularRLock() // want `found recursive read lock call`
}

func recursiveRLock(run bool) {
	mutex.RLock()
	defer mutex.RUnlock()
	if run {
		recursiveRLock(!run) // want `found recursive read lock call`
	}
}
