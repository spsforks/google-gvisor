// automatically generated by stateify.

package lock

import (
	"gvisor.dev/gvisor/pkg/state"
)

func (o *OwnerInfo) StateTypeName() string {
	return "pkg/sentry/fs/lock.OwnerInfo"
}

func (o *OwnerInfo) StateFields() []string {
	return []string{
		"PID",
	}
}

func (o *OwnerInfo) beforeSave() {}

// +checklocksignore
func (o *OwnerInfo) StateSave(stateSinkObject state.Sink) {
	o.beforeSave()
	stateSinkObject.Save(0, &o.PID)
}

func (o *OwnerInfo) afterLoad() {}

// +checklocksignore
func (o *OwnerInfo) StateLoad(stateSourceObject state.Source) {
	stateSourceObject.Load(0, &o.PID)
}

func (l *Lock) StateTypeName() string {
	return "pkg/sentry/fs/lock.Lock"
}

func (l *Lock) StateFields() []string {
	return []string{
		"Readers",
		"Writer",
		"WriterInfo",
	}
}

func (l *Lock) beforeSave() {}

// +checklocksignore
func (l *Lock) StateSave(stateSinkObject state.Sink) {
	l.beforeSave()
	stateSinkObject.Save(0, &l.Readers)
	stateSinkObject.Save(1, &l.Writer)
	stateSinkObject.Save(2, &l.WriterInfo)
}

func (l *Lock) afterLoad() {}

// +checklocksignore
func (l *Lock) StateLoad(stateSourceObject state.Source) {
	stateSourceObject.Load(0, &l.Readers)
	stateSourceObject.Load(1, &l.Writer)
	stateSourceObject.Load(2, &l.WriterInfo)
}

func (l *Locks) StateTypeName() string {
	return "pkg/sentry/fs/lock.Locks"
}

func (l *Locks) StateFields() []string {
	return []string{
		"locks",
		"blockedQueue",
	}
}

func (l *Locks) beforeSave() {}

// +checklocksignore
func (l *Locks) StateSave(stateSinkObject state.Sink) {
	l.beforeSave()
	stateSinkObject.Save(0, &l.locks)
	stateSinkObject.Save(1, &l.blockedQueue)
}

func (l *Locks) afterLoad() {}

// +checklocksignore
func (l *Locks) StateLoad(stateSourceObject state.Source) {
	stateSourceObject.Load(0, &l.locks)
	stateSourceObject.Load(1, &l.blockedQueue)
}

func (r *LockRange) StateTypeName() string {
	return "pkg/sentry/fs/lock.LockRange"
}

func (r *LockRange) StateFields() []string {
	return []string{
		"Start",
		"End",
	}
}

func (r *LockRange) beforeSave() {}

// +checklocksignore
func (r *LockRange) StateSave(stateSinkObject state.Sink) {
	r.beforeSave()
	stateSinkObject.Save(0, &r.Start)
	stateSinkObject.Save(1, &r.End)
}

func (r *LockRange) afterLoad() {}

// +checklocksignore
func (r *LockRange) StateLoad(stateSourceObject state.Source) {
	stateSourceObject.Load(0, &r.Start)
	stateSourceObject.Load(1, &r.End)
}

func (s *LockSet) StateTypeName() string {
	return "pkg/sentry/fs/lock.LockSet"
}

func (s *LockSet) StateFields() []string {
	return []string{
		"root",
	}
}

func (s *LockSet) beforeSave() {}

// +checklocksignore
func (s *LockSet) StateSave(stateSinkObject state.Sink) {
	s.beforeSave()
	var rootValue *LockSegmentDataSlices
	rootValue = s.saveRoot()
	stateSinkObject.SaveValue(0, rootValue)
}

func (s *LockSet) afterLoad() {}

// +checklocksignore
func (s *LockSet) StateLoad(stateSourceObject state.Source) {
	stateSourceObject.LoadValue(0, new(*LockSegmentDataSlices), func(y any) { s.loadRoot(y.(*LockSegmentDataSlices)) })
}

func (n *Locknode) StateTypeName() string {
	return "pkg/sentry/fs/lock.Locknode"
}

func (n *Locknode) StateFields() []string {
	return []string{
		"nrSegments",
		"parent",
		"parentIndex",
		"hasChildren",
		"maxGap",
		"keys",
		"values",
		"children",
	}
}

func (n *Locknode) beforeSave() {}

// +checklocksignore
func (n *Locknode) StateSave(stateSinkObject state.Sink) {
	n.beforeSave()
	stateSinkObject.Save(0, &n.nrSegments)
	stateSinkObject.Save(1, &n.parent)
	stateSinkObject.Save(2, &n.parentIndex)
	stateSinkObject.Save(3, &n.hasChildren)
	stateSinkObject.Save(4, &n.maxGap)
	stateSinkObject.Save(5, &n.keys)
	stateSinkObject.Save(6, &n.values)
	stateSinkObject.Save(7, &n.children)
}

func (n *Locknode) afterLoad() {}

// +checklocksignore
func (n *Locknode) StateLoad(stateSourceObject state.Source) {
	stateSourceObject.Load(0, &n.nrSegments)
	stateSourceObject.Load(1, &n.parent)
	stateSourceObject.Load(2, &n.parentIndex)
	stateSourceObject.Load(3, &n.hasChildren)
	stateSourceObject.Load(4, &n.maxGap)
	stateSourceObject.Load(5, &n.keys)
	stateSourceObject.Load(6, &n.values)
	stateSourceObject.Load(7, &n.children)
}

func (l *LockSegmentDataSlices) StateTypeName() string {
	return "pkg/sentry/fs/lock.LockSegmentDataSlices"
}

func (l *LockSegmentDataSlices) StateFields() []string {
	return []string{
		"Start",
		"End",
		"Values",
	}
}

func (l *LockSegmentDataSlices) beforeSave() {}

// +checklocksignore
func (l *LockSegmentDataSlices) StateSave(stateSinkObject state.Sink) {
	l.beforeSave()
	stateSinkObject.Save(0, &l.Start)
	stateSinkObject.Save(1, &l.End)
	stateSinkObject.Save(2, &l.Values)
}

func (l *LockSegmentDataSlices) afterLoad() {}

// +checklocksignore
func (l *LockSegmentDataSlices) StateLoad(stateSourceObject state.Source) {
	stateSourceObject.Load(0, &l.Start)
	stateSourceObject.Load(1, &l.End)
	stateSourceObject.Load(2, &l.Values)
}

func init() {
	state.Register((*OwnerInfo)(nil))
	state.Register((*Lock)(nil))
	state.Register((*Locks)(nil))
	state.Register((*LockRange)(nil))
	state.Register((*LockSet)(nil))
	state.Register((*Locknode)(nil))
	state.Register((*LockSegmentDataSlices)(nil))
}
