// automatically generated by stateify.

package auth

import (
	"gvisor.dev/gvisor/pkg/state"
)

func (p *AtomicPtrCredentials) StateTypeName() string {
	return "pkg/sentry/kernel/auth.AtomicPtrCredentials"
}

func (p *AtomicPtrCredentials) StateFields() []string {
	return []string{
		"ptr",
	}
}

func (p *AtomicPtrCredentials) beforeSave() {}

// +checklocksignore
func (p *AtomicPtrCredentials) StateSave(stateSinkObject state.Sink) {
	p.beforeSave()
	var ptrValue *Credentials
	ptrValue = p.savePtr()
	stateSinkObject.SaveValue(0, ptrValue)
}

func (p *AtomicPtrCredentials) afterLoad() {}

// +checklocksignore
func (p *AtomicPtrCredentials) StateLoad(stateSourceObject state.Source) {
	stateSourceObject.LoadValue(0, new(*Credentials), func(y any) { p.loadPtr(y.(*Credentials)) })
}

func init() {
	state.Register((*AtomicPtrCredentials)(nil))
}