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
	"fmt"
	"unsafe"

	"golang.org/x/sys/unix"
	"gvisor.dev/gvisor/pkg/abi/linux"
	"gvisor.dev/gvisor/pkg/sentry/arch"
)

func callWithSignalFrame(stack uintptr, handler uintptr, sigframeRAX uintptr, sigframe *arch.UContext64, fpstate uintptr)

// CallWithSignalFrame create a signal frame on the stack and execute a
// user-defined callback function within that context.
//
//go:nosplit
func CallWithSignalFrame(stack uintptr, handlerAddr uintptr, sigframeRAX uintptr, sigframe *arch.UContext64, fpstate uintptr, sigmask *linux.SignalSet) {
	_, _, errno := unix.RawSyscall6(
		unix.SYS_RT_SIGPROCMASK, linux.SIG_BLOCK,
		uintptr(unsafe.Pointer(sigmask)),
		uintptr(unsafe.Pointer(&sigframe.Sigset)),
		linux.SignalSetSize,
		0, 0)
	if errno != 0 {
		panic(fmt.Sprintf("sigprocmask failed: %d", errno))
	}
	callWithSignalFrame(stack, handlerAddr, sigframeRAX, sigframe, fpstate)
}

// Sigreturn restores the thread state from the signal frame.
func Sigreturn(sigframeAddr *arch.UContext64)
