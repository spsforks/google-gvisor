// automatically generated by stateify.

package overlay

import (
	"gvisor.dev/gvisor/pkg/state"
)

func (fd *directoryFD) StateTypeName() string {
	return "pkg/sentry/fsimpl/overlay.directoryFD"
}

func (fd *directoryFD) StateFields() []string {
	return []string{
		"fileDescription",
		"DirectoryFileDescriptionDefaultImpl",
		"DentryMetadataFileDescriptionImpl",
		"off",
		"dirents",
	}
}

func (fd *directoryFD) beforeSave() {}

func (fd *directoryFD) StateSave(stateSinkObject state.Sink) {
	fd.beforeSave()
	stateSinkObject.Save(0, &fd.fileDescription)
	stateSinkObject.Save(1, &fd.DirectoryFileDescriptionDefaultImpl)
	stateSinkObject.Save(2, &fd.DentryMetadataFileDescriptionImpl)
	stateSinkObject.Save(3, &fd.off)
	stateSinkObject.Save(4, &fd.dirents)
}

func (fd *directoryFD) afterLoad() {}

func (fd *directoryFD) StateLoad(stateSourceObject state.Source) {
	stateSourceObject.Load(0, &fd.fileDescription)
	stateSourceObject.Load(1, &fd.DirectoryFileDescriptionDefaultImpl)
	stateSourceObject.Load(2, &fd.DentryMetadataFileDescriptionImpl)
	stateSourceObject.Load(3, &fd.off)
	stateSourceObject.Load(4, &fd.dirents)
}

func (fstype *FilesystemType) StateTypeName() string {
	return "pkg/sentry/fsimpl/overlay.FilesystemType"
}

func (fstype *FilesystemType) StateFields() []string {
	return []string{}
}

func (fstype *FilesystemType) beforeSave() {}

func (fstype *FilesystemType) StateSave(stateSinkObject state.Sink) {
	fstype.beforeSave()
}

func (fstype *FilesystemType) afterLoad() {}

func (fstype *FilesystemType) StateLoad(stateSourceObject state.Source) {
}

func (f *FilesystemOptions) StateTypeName() string {
	return "pkg/sentry/fsimpl/overlay.FilesystemOptions"
}

func (f *FilesystemOptions) StateFields() []string {
	return []string{
		"UpperRoot",
		"LowerRoots",
	}
}

func (f *FilesystemOptions) beforeSave() {}

func (f *FilesystemOptions) StateSave(stateSinkObject state.Sink) {
	f.beforeSave()
	stateSinkObject.Save(0, &f.UpperRoot)
	stateSinkObject.Save(1, &f.LowerRoots)
}

func (f *FilesystemOptions) afterLoad() {}

func (f *FilesystemOptions) StateLoad(stateSourceObject state.Source) {
	stateSourceObject.Load(0, &f.UpperRoot)
	stateSourceObject.Load(1, &f.LowerRoots)
}

func (fs *filesystem) StateTypeName() string {
	return "pkg/sentry/fsimpl/overlay.filesystem"
}

func (fs *filesystem) StateFields() []string {
	return []string{
		"vfsfs",
		"opts",
		"creds",
		"dirDevMinor",
		"lowerDevMinors",
		"lastDirIno",
	}
}

func (fs *filesystem) beforeSave() {}

func (fs *filesystem) StateSave(stateSinkObject state.Sink) {
	fs.beforeSave()
	stateSinkObject.Save(0, &fs.vfsfs)
	stateSinkObject.Save(1, &fs.opts)
	stateSinkObject.Save(2, &fs.creds)
	stateSinkObject.Save(3, &fs.dirDevMinor)
	stateSinkObject.Save(4, &fs.lowerDevMinors)
	stateSinkObject.Save(5, &fs.lastDirIno)
}

func (fs *filesystem) afterLoad() {}

func (fs *filesystem) StateLoad(stateSourceObject state.Source) {
	stateSourceObject.Load(0, &fs.vfsfs)
	stateSourceObject.Load(1, &fs.opts)
	stateSourceObject.Load(2, &fs.creds)
	stateSourceObject.Load(3, &fs.dirDevMinor)
	stateSourceObject.Load(4, &fs.lowerDevMinors)
	stateSourceObject.Load(5, &fs.lastDirIno)
}

func (d *dentry) StateTypeName() string {
	return "pkg/sentry/fsimpl/overlay.dentry"
}

func (d *dentry) StateFields() []string {
	return []string{
		"vfsd",
		"refs",
		"fs",
		"mode",
		"uid",
		"gid",
		"copiedUp",
		"parent",
		"name",
		"children",
		"dirents",
		"upperVD",
		"lowerVDs",
		"inlineLowerVDs",
		"devMajor",
		"devMinor",
		"ino",
		"mapsMu",
		"lowerMappings",
		"dataMu",
		"wrappedMappable",
		"isMappable",
		"locks",
		"watches",
	}
}

func (d *dentry) beforeSave() {}

func (d *dentry) StateSave(stateSinkObject state.Sink) {
	d.beforeSave()
	stateSinkObject.Save(0, &d.vfsd)
	stateSinkObject.Save(1, &d.refs)
	stateSinkObject.Save(2, &d.fs)
	stateSinkObject.Save(3, &d.mode)
	stateSinkObject.Save(4, &d.uid)
	stateSinkObject.Save(5, &d.gid)
	stateSinkObject.Save(6, &d.copiedUp)
	stateSinkObject.Save(7, &d.parent)
	stateSinkObject.Save(8, &d.name)
	stateSinkObject.Save(9, &d.children)
	stateSinkObject.Save(10, &d.dirents)
	stateSinkObject.Save(11, &d.upperVD)
	stateSinkObject.Save(12, &d.lowerVDs)
	stateSinkObject.Save(13, &d.inlineLowerVDs)
	stateSinkObject.Save(14, &d.devMajor)
	stateSinkObject.Save(15, &d.devMinor)
	stateSinkObject.Save(16, &d.ino)
	stateSinkObject.Save(17, &d.mapsMu)
	stateSinkObject.Save(18, &d.lowerMappings)
	stateSinkObject.Save(19, &d.dataMu)
	stateSinkObject.Save(20, &d.wrappedMappable)
	stateSinkObject.Save(21, &d.isMappable)
	stateSinkObject.Save(22, &d.locks)
	stateSinkObject.Save(23, &d.watches)
}

func (d *dentry) afterLoad() {}

func (d *dentry) StateLoad(stateSourceObject state.Source) {
	stateSourceObject.Load(0, &d.vfsd)
	stateSourceObject.Load(1, &d.refs)
	stateSourceObject.Load(2, &d.fs)
	stateSourceObject.Load(3, &d.mode)
	stateSourceObject.Load(4, &d.uid)
	stateSourceObject.Load(5, &d.gid)
	stateSourceObject.Load(6, &d.copiedUp)
	stateSourceObject.Load(7, &d.parent)
	stateSourceObject.Load(8, &d.name)
	stateSourceObject.Load(9, &d.children)
	stateSourceObject.Load(10, &d.dirents)
	stateSourceObject.Load(11, &d.upperVD)
	stateSourceObject.Load(12, &d.lowerVDs)
	stateSourceObject.Load(13, &d.inlineLowerVDs)
	stateSourceObject.Load(14, &d.devMajor)
	stateSourceObject.Load(15, &d.devMinor)
	stateSourceObject.Load(16, &d.ino)
	stateSourceObject.Load(17, &d.mapsMu)
	stateSourceObject.Load(18, &d.lowerMappings)
	stateSourceObject.Load(19, &d.dataMu)
	stateSourceObject.Load(20, &d.wrappedMappable)
	stateSourceObject.Load(21, &d.isMappable)
	stateSourceObject.Load(22, &d.locks)
	stateSourceObject.Load(23, &d.watches)
}

func (fd *fileDescription) StateTypeName() string {
	return "pkg/sentry/fsimpl/overlay.fileDescription"
}

func (fd *fileDescription) StateFields() []string {
	return []string{
		"vfsfd",
		"FileDescriptionDefaultImpl",
		"LockFD",
	}
}

func (fd *fileDescription) beforeSave() {}

func (fd *fileDescription) StateSave(stateSinkObject state.Sink) {
	fd.beforeSave()
	stateSinkObject.Save(0, &fd.vfsfd)
	stateSinkObject.Save(1, &fd.FileDescriptionDefaultImpl)
	stateSinkObject.Save(2, &fd.LockFD)
}

func (fd *fileDescription) afterLoad() {}

func (fd *fileDescription) StateLoad(stateSourceObject state.Source) {
	stateSourceObject.Load(0, &fd.vfsfd)
	stateSourceObject.Load(1, &fd.FileDescriptionDefaultImpl)
	stateSourceObject.Load(2, &fd.LockFD)
}

func (fd *regularFileFD) StateTypeName() string {
	return "pkg/sentry/fsimpl/overlay.regularFileFD"
}

func (fd *regularFileFD) StateFields() []string {
	return []string{
		"fileDescription",
		"copiedUp",
		"cachedFD",
		"cachedFlags",
		"lowerWaiters",
	}
}

func (fd *regularFileFD) beforeSave() {}

func (fd *regularFileFD) StateSave(stateSinkObject state.Sink) {
	fd.beforeSave()
	stateSinkObject.Save(0, &fd.fileDescription)
	stateSinkObject.Save(1, &fd.copiedUp)
	stateSinkObject.Save(2, &fd.cachedFD)
	stateSinkObject.Save(3, &fd.cachedFlags)
	stateSinkObject.Save(4, &fd.lowerWaiters)
}

func (fd *regularFileFD) afterLoad() {}

func (fd *regularFileFD) StateLoad(stateSourceObject state.Source) {
	stateSourceObject.Load(0, &fd.fileDescription)
	stateSourceObject.Load(1, &fd.copiedUp)
	stateSourceObject.Load(2, &fd.cachedFD)
	stateSourceObject.Load(3, &fd.cachedFlags)
	stateSourceObject.Load(4, &fd.lowerWaiters)
}

func init() {
	state.Register((*directoryFD)(nil))
	state.Register((*FilesystemType)(nil))
	state.Register((*FilesystemOptions)(nil))
	state.Register((*filesystem)(nil))
	state.Register((*dentry)(nil))
	state.Register((*fileDescription)(nil))
	state.Register((*regularFileFD)(nil))
}
