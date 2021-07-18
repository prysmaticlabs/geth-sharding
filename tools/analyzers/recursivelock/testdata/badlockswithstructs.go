package testdata

var resource *ProtectResource = &ProtectResource{resource: "protected"}
var nested *NestedResource = &NestedResource{p: ProtectResource{resource: "hello"}}

func DoSomething() {
	resource.RLock()
	resource.GetResource() // want `found recursive read lock call`
	resource.RUnlock()
}

func AnotherWayToDoSomething(r *ProtectResource) {
	r.RLock()
	r.GetResource() // want `found recursive read lock call`
	r.RUnlock()
}

func NestedStruct() {
	nested.p.RLock()
	nested.p.GetResource() // want `found recursive read lock call`
	nested.p.RUnlock()
}
