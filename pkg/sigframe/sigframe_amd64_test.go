// Copyright 2024 The gVisor Authors.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

//go:build amd64
// +build amd64

package sigframe

import (
	"testing"
	"unsafe"

	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/abi/linux"
	"gvisor.dev/gvisor/pkg/hostarch"
	"gvisor.dev/gvisor/pkg/sentry/arch"
)

func TestUserSignalHandler(t *testing.T) {
	addr, _, _ := unix.Syscall6(unix.SYS_MMAP, 0, hostarch.PageSize*4, uintptr(unix.PROT_READ|unix.PROT_WRITE),
		uintptr(unix.MAP_PRIVATE|unix.MAP_ANONYMOUS),
		0, 0)
	set := linux.MakeSignalSet(linux.SIGURG)
	sigframe := (*arch.UContext64)(unsafe.Pointer(addr + hostarch.PageSize))
	CallWithSignalFrame(addr+hostarch.PageSize, addrOfUserHandler(), addr+hostarch.PageSize*2, sigframe, 0, &set)
}

func BenchmarkUserSigHandler(b *testing.B) {
	addr, _, _ := unix.Syscall6(unix.SYS_MMAP, 0, hostarch.PageSize*4, uintptr(unix.PROT_READ|unix.PROT_WRITE),
		uintptr(unix.MAP_PRIVATE|unix.MAP_ANONYMOUS),
		0, 0)
	set := linux.MakeSignalSet(linux.SIGURG)
	sigframe := (*arch.UContext64)(unsafe.Pointer(addr + hostarch.PageSize))
	for i := 0; i < b.N; i++ {
		CallWithSignalFrame(addr+hostarch.PageSize, addrOfUserHandler(), addr+hostarch.PageSize*2, sigframe, 0, &set)
	}
}

func addrOfUserHandler() uintptr

func userHandler(sigframe *arch.UContext64) {
	Sigreturn(sigframe)
}
