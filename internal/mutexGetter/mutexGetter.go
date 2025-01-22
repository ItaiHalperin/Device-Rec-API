package mutexGetter

import "sync"

var mutex *sync.Mutex

func GetMutex() *sync.Mutex {
	if mutex == nil {
		mutex = new(sync.Mutex)
	}
	return mutex
}
