package testdata

import (
	"sync"
)

type ProtectResource struct {
	*sync.RWMutex
	resource string
}

func (r *ProtectResource) GetResource() string {
	defer r.RUnlock()
	r.RLock()
	return r.resource
}

type NotProtected struct {
	resource string
}

func (r *NotProtected) GetResource() string {
	return r.resource
}

type NestedResource struct {
	*NotProtected
	p ProtectResource
}
