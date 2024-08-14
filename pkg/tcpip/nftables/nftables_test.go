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

package nftables

import (
	"fmt"
	"reflect"
	"testing"

	"gvisor.dev/gvisor/pkg/abi/linux"
	"gvisor.dev/gvisor/pkg/buffer"
	"gvisor.dev/gvisor/pkg/tcpip/stack"
)

const (
	arbitraryTargetChain string        = "target_chain"
	arbitraryHook        Hook          = Prerouting
	arbitraryFamily      AddressFamily = Inet
)

var (
	arbitraryPriority Priority = func() Priority {
		priority, err := NewStandardPriority("filter", arbitraryFamily, arbitraryHook)
		if err != nil {
			panic(fmt.Sprintf("unexpected error for NewStandardPriority: %v", err))
		}
		return priority
	}()
	arbitraryInfoPolicyAccept *BaseChainInfo = &BaseChainInfo{
		BcType:   BaseChainTypeFilter,
		Hook:     arbitraryHook,
		Priority: arbitraryPriority,
	}
)

// makeTestingPacket creates an arbitrary packet for testing.
func makeTestingPacket() *stack.PacketBuffer {
	return stack.NewPacketBuffer(stack.PacketBufferOptions{
		ReserveHeaderBytes: 50,
		Payload:            buffer.MakeWithData([]byte{0, 2, 4, 8, 16, 32, 64, 128}),
	})
}

// TestUnsupportedAddressFamily tests that an empty NFTables object returns an
// error when evaluating a packet for an unsupported address family.
func TestUnsupportedAddressFamily(t *testing.T) {
	nf := NewNFTables()
	for _, unsupportedFamily := range []AddressFamily{AddressFamily(NumAFs), AddressFamily(-1)} {
		// Note: the Prerouting hook is arbitrary (any hook would work).
		pkt := makeTestingPacket()
		v, err := nf.EvaluateHook(unsupportedFamily, arbitraryHook, pkt)
		if err == nil {
			t.Fatalf("expecting error for EvaluateHook with unsupported address family %d; got %v verdict, %s packet, and error %v",
				int(unsupportedFamily),
				v, packetResultString(makeTestingPacket(), pkt), err)
		}
	}
}

// TestAcceptAll tests that an empty NFTables object accepts all packets for
// supported hooks and errors for unsupported hooks for all address families
// when evaluating packets at the hook-level.
func TestAcceptAllForSupportedHooks(t *testing.T) {
	for _, family := range []AddressFamily{IP, IP6, Inet, Arp, Bridge, Netdev} {
		t.Run(family.String()+" address family", func(t *testing.T) {
			nf := NewNFTables()
			for _, hook := range []Hook{Prerouting, Input, Forward, Output, Postrouting, Ingress, Egress} {
				pkt := makeTestingPacket()
				v, err := nf.EvaluateHook(family, hook, pkt)

				supported := false
				for _, h := range supportedHooks[family] {
					if h == hook {
						supported = true
						break
					}
				}

				if supported {
					if err != nil || v.Code != VC(linux.NF_ACCEPT) {
						t.Fatalf("expecting accept verdict for EvaluateHook with supported hook %v for family %v; got %v verdict, %s packet, and error %v",
							hook, family,
							v, packetResultString(makeTestingPacket(), pkt), err)
					}
				} else {
					if err == nil {
						t.Fatalf("expecting error for EvaluateHook with unsupported hook %v for family %v; got %v verdict, %s packet, and error %v",
							hook, family,
							v, packetResultString(makeTestingPacket(), pkt), err)
					}
				}
			}
		})
	}
}

// TestEvaluateImmediate tests that the Immediate operation correctly sets the
// register value and behaves as expected during evaluation.
func TestEvaluateImmediate(t *testing.T) {
	for _, test := range []struct {
		tname    string
		baseOp1  operation // will be nil if unused
		baseOp2  operation // will be nil if unused
		targetOp operation // will be nil if unused
		verdict  Verdict
	}{
		{
			tname:   "no operations",
			verdict: Verdict{Code: VC(linux.NF_ACCEPT)}, // from base chain policy
		},
		{
			tname:   "immediately accept",
			baseOp1: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_ACCEPT)})),
			verdict: Verdict{Code: VC(linux.NF_ACCEPT)},
		},
		{
			tname:   "immediately drop",
			baseOp1: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)})),
			verdict: Verdict{Code: VC(linux.NF_DROP)},
		},
		{
			tname:   "immediately continue with base chain policy accept",
			baseOp1: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_CONTINUE)})),
			verdict: Verdict{Code: VC(linux.NF_ACCEPT)}, // from base chain policy
		},
		{
			tname:   "immediately return with base chain policy accept",
			baseOp1: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_RETURN)})),
			verdict: Verdict{Code: VC(linux.NF_ACCEPT)}, // from base chain policy
		},
		{
			tname:    "immediately jump to target chain that accepts",
			baseOp1:  mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: arbitraryTargetChain})),
			targetOp: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_ACCEPT)})),
			verdict:  Verdict{Code: VC(linux.NF_ACCEPT)},
		},
		{
			tname:    "immediately jump to target chain that drops",
			baseOp1:  mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: arbitraryTargetChain})),
			targetOp: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)})),
			verdict:  Verdict{Code: VC(linux.NF_DROP)},
		},
		{
			tname:    "immediately jump to target chain that continues with second rule that accepts",
			baseOp1:  mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: arbitraryTargetChain})),
			targetOp: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_CONTINUE)})),
			baseOp2:  mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_ACCEPT)})),
			verdict:  Verdict{Code: VC(linux.NF_ACCEPT)},
		},
		{
			tname:    "immediately jump to target chain that continues with second rule that drops",
			baseOp1:  mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: arbitraryTargetChain})),
			targetOp: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_CONTINUE)})),
			baseOp2:  mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)})),
			verdict:  Verdict{Code: VC(linux.NF_DROP)},
		},
		{
			tname:    "immediately goto to target chain that accepts",
			baseOp1:  mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: arbitraryTargetChain})),
			targetOp: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_ACCEPT)})),
			verdict:  Verdict{Code: VC(linux.NF_ACCEPT)},
		},
		{
			tname:    "immediately goto to target chain that drops",
			baseOp1:  mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: arbitraryTargetChain})),
			targetOp: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)})),
			verdict:  Verdict{Code: VC(linux.NF_DROP)},
		},
		{
			tname:    "immediately goto to target chain that continues with second rule that accepts",
			baseOp1:  mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: arbitraryTargetChain})),
			targetOp: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_CONTINUE)})),
			baseOp2:  mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_ACCEPT)})),
			verdict:  Verdict{Code: VC(linux.NF_ACCEPT)}, // from base chain policy
		},
		{
			tname:    "immediately goto to target chain that continues with second rule that drops",
			baseOp1:  mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: arbitraryTargetChain})),
			targetOp: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_CONTINUE)})),
			baseOp2:  mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)})),
			verdict:  Verdict{Code: VC(linux.NF_ACCEPT)}, // from base chain policy
		},
		{
			tname:   "add data to register then accept",
			baseOp1: mustCreateImmediate(t, linux.NFT_REG32_13, newBytesData([]byte{0, 1, 2, 3})),
			baseOp2: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_ACCEPT)})),
			verdict: Verdict{Code: VC(linux.NF_ACCEPT)},
		},
		{
			tname:   "add data to register then drop",
			baseOp1: mustCreateImmediate(t, linux.NFT_REG32_15, newBytesData([]byte{0, 1, 2, 3})),
			baseOp2: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)})),
			verdict: Verdict{Code: VC(linux.NF_DROP)},
		},
		{
			tname:   "add data to register then continue",
			baseOp1: mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{0, 1, 2, 3})),
			baseOp2: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_CONTINUE)})),
			verdict: Verdict{Code: VC(linux.NF_ACCEPT)}, // from base chain policy
		},
		{
			tname:   "multiple accepts",
			baseOp1: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_ACCEPT)})),
			baseOp2: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_ACCEPT)})),
			verdict: Verdict{Code: VC(linux.NF_ACCEPT)},
		},
		{
			tname:   "multiple drops",
			baseOp1: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)})),
			baseOp2: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)})),
			verdict: Verdict{Code: VC(linux.NF_DROP)},
		},
		{
			tname:   "immediately accept then drop",
			baseOp1: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_ACCEPT)})),
			baseOp2: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)})),
			verdict: Verdict{Code: VC(linux.NF_ACCEPT)},
		},
		{
			tname:   "immediately drop then accept",
			baseOp1: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)})),
			baseOp2: mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_ACCEPT)})),
			verdict: Verdict{Code: VC(linux.NF_DROP)},
		},
	} {
		t.Run(test.tname, func(t *testing.T) {
			// Sets up an NFTables object with a base chain (for 2 rules) and another
			// target chain (for 1 rule).
			nf := NewNFTables()
			tab, err := nf.AddTable(arbitraryFamily, "test", "test table", false)
			if err != nil {
				t.Fatalf("unexpected error for AddTable: %v", err)
			}
			bc, err := tab.AddChain("base_chain", nil, "test chain", false)
			if err != nil {
				t.Fatalf("unexpected error for AddChain: %v", err)
			}
			bc.SetBaseChainInfo(arbitraryInfoPolicyAccept)
			tc, err := tab.AddChain(arbitraryTargetChain, nil, "test chain", false)
			if err != nil {
				t.Fatalf("unexpected error for AddChain: %v", err)
			}

			// Adds testing rules and operations.
			if test.baseOp1 != nil {
				rule1 := &Rule{}
				rule1.addOperation(test.baseOp1)
				if err := bc.RegisterRule(rule1, -1); err != nil {
					t.Fatalf("unexpected error for RegisterRule for the first operation: %v", err)
				}
			}
			if test.baseOp2 != nil {
				rule2 := &Rule{}
				rule2.addOperation(test.baseOp2)
				if err := bc.RegisterRule(rule2, -1); err != nil {
					t.Fatalf("unexpected error for RegisterRule for the second operation: %v", err)
				}
			}
			if test.targetOp != nil {
				ruleTarget := &Rule{}
				ruleTarget.addOperation(test.targetOp)
				if err := tc.RegisterRule(ruleTarget, -1); err != nil {
					t.Fatalf("unexpected error for RegisterRule for the target operation: %v", err)
				}
			}

			// Runs evaluation and checks verdict.
			pkt := makeTestingPacket()
			v, err := nf.EvaluateHook(arbitraryFamily, arbitraryHook, pkt)

			if err != nil {
				t.Fatalf("unexpected error for EvaluateHook: %v", err)
			}
			if v.Code != test.verdict.Code {
				t.Fatalf("expected verdict %v, got %v", test.verdict, v)
			}
		})
	}
}

// TestEvaluateComparison tests that the Comparison operation correctly compares
// the data in the source register to the given data.
// Note: Relies on expected behavior of the Immediate operation.
func TestEvaluateComparison(t *testing.T) {
	for _, test := range []struct {
		tname string
		op1   operation // will be nil if unused
		op2   operation // will be nil if unused
		res   bool      // should be true if we reach end of the rule (no breaks)
	}{
		// 4-byte data comparisons, alternates between 4-byte and 16-byte registers.
		{
			tname: "compare register == 4-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_EQ, newBytesData([]byte{0, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare register == 4-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG32_11, newBytesData([]byte{1, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG32_11, linux.NFT_CMP_EQ, newBytesData([]byte{0, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare register != 4-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG32_03, newBytesData([]byte{1, 7, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG32_03, linux.NFT_CMP_NEQ, newBytesData([]byte{1, 98, 0, 56})),
			res:   true,
		},
		{
			tname: "compare register != 4-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{1, 98, 0, 56})),
			op2:   mustCreateComparison(t, linux.NFT_REG_3, linux.NFT_CMP_NEQ, newBytesData([]byte{1, 98, 0, 56})),
			res:   false,
		},
		{
			tname: "compare register < 4-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{29, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_LT, newBytesData([]byte{100, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare register < 4-byte data, false eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG32_04, newBytesData([]byte{100, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG32_04, linux.NFT_CMP_LT, newBytesData([]byte{100, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare register < 4-byte data, false gt",
			op1:   mustCreateImmediate(t, linux.NFT_REG32_14, newBytesData([]byte{200, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG32_14, linux.NFT_CMP_LT, newBytesData([]byte{100, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare register > 4-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG32_15, newBytesData([]byte{0, 0, 0, 1})),
			op2:   mustCreateComparison(t, linux.NFT_REG32_15, linux.NFT_CMP_GT, newBytesData([]byte{29, 76, 230, 0})),
			res:   true,
		},
		{
			tname: "compare register > 4-byte data, false eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG32_07, newBytesData([]byte{29, 76, 230, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG32_07, linux.NFT_CMP_GT, newBytesData([]byte{29, 76, 230, 0})),
			res:   false,
		},
		{
			tname: "compare register > 4-byte data, false lt",
			op1:   mustCreateImmediate(t, linux.NFT_REG32_05, newBytesData([]byte{28, 76, 230, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG32_05, linux.NFT_CMP_GT, newBytesData([]byte{29, 76, 230, 0})),
			res:   false,
		},
		{
			tname: "compare register <= 4-byte data, true lt",
			op1:   mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{29, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_2, linux.NFT_CMP_LTE, newBytesData([]byte{100, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare register <= 4-byte data, true eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG32_09, newBytesData([]byte{100, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG32_09, linux.NFT_CMP_LTE, newBytesData([]byte{100, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare register <= 4-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG32_06, newBytesData([]byte{200, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG32_06, linux.NFT_CMP_LTE, newBytesData([]byte{100, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare register >= 4-byte data, true gt",
			op1:   mustCreateImmediate(t, linux.NFT_REG32_12, newBytesData([]byte{0, 0, 0, 1})),
			op2:   mustCreateComparison(t, linux.NFT_REG32_12, linux.NFT_CMP_GTE, newBytesData([]byte{29, 76, 230, 0})),
			res:   true,
		},
		{
			tname: "compare register >= 4-byte data, true eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{29, 76, 230, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_GTE, newBytesData([]byte{29, 76, 230, 0})),
			res:   true,
		},
		{
			tname: "compare register >= 4-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{28, 76, 230, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_3, linux.NFT_CMP_GTE, newBytesData([]byte{29, 76, 230, 0})),
			res:   false,
		},
		// 8-byte data comparisons.
		{
			tname: "compare register == 8-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{0, 0, 0, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_EQ, newBytesData([]byte{0, 0, 0, 0, 0, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare register == 8-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{1, 0, 0, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_2, linux.NFT_CMP_EQ, newBytesData([]byte{0, 0, 0, 0, 0, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare register != 8-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{1, 7, 0, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_3, linux.NFT_CMP_NEQ, newBytesData([]byte{1, 98, 0, 56, 0, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare register != 8-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{1, 98, 0, 56, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_NEQ, newBytesData([]byte{1, 98, 0, 56, 0, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare register < 8-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{29, 0, 0, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_LT, newBytesData([]byte{100, 0, 0, 0, 0, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare register < 8-byte data, false eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{100, 0, 0, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_2, linux.NFT_CMP_LT, newBytesData([]byte{100, 0, 0, 0, 0, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare register < 8-byte data, false gt",
			op1:   mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{200, 0, 0, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_3, linux.NFT_CMP_LT, newBytesData([]byte{100, 0, 0, 0, 0, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare register > 8-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{0, 0, 0, 1, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_GT, newBytesData([]byte{29, 76, 230, 0, 0, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare register > 8-byte data, false eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{29, 76, 230, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_GT, newBytesData([]byte{29, 76, 230, 0, 0, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare register > 8-byte data, false lt",
			op1:   mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{28, 76, 230, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_2, linux.NFT_CMP_GT, newBytesData([]byte{29, 76, 230, 0, 0, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare register <= 8-byte data, true lt",
			op1:   mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{29, 0, 0, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_3, linux.NFT_CMP_LTE, newBytesData([]byte{100, 0, 0, 0, 0, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare register <= 8-byte data, true eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{100, 0, 0, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_LTE, newBytesData([]byte{100, 0, 0, 0, 0, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare register <= 8-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{200, 0, 0, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_LTE, newBytesData([]byte{100, 0, 0, 0, 0, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare register >= 8-byte data, true gt",
			op1:   mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{0, 0, 0, 1, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_2, linux.NFT_CMP_GTE, newBytesData([]byte{29, 76, 230, 0, 0, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare register >= 8-byte data, true eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{29, 76, 230, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_3, linux.NFT_CMP_GTE, newBytesData([]byte{29, 76, 230, 0, 0, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare register >= 8-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{28, 76, 230, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_GTE, newBytesData([]byte{29, 76, 230, 0, 0, 0, 0, 0})),
			res:   false,
		},
		// 12-byte data comparisons.
		{
			tname: "compare register == 12-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_EQ, newBytesData([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare register == 12-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})),
			op2:   mustCreateComparison(t, linux.NFT_REG_2, linux.NFT_CMP_EQ, newBytesData([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare register != 12-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})),
			op2:   mustCreateComparison(t, linux.NFT_REG_3, linux.NFT_CMP_NEQ, newBytesData([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11})),
			res:   true,
		},
		{
			tname: "compare register != 12-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})),
			op2:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_NEQ, newBytesData([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})),
			res:   false,
		},
		{
			tname: "compare register < 12-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{0x0a, 0x00, 0x01, 0x1f, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			op2:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_LT, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			res:   true,
		},
		{
			tname: "compare register < 12-byte data, false eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			op2:   mustCreateComparison(t, linux.NFT_REG_2, linux.NFT_CMP_LT, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			res:   false,
		},
		{
			tname: "compare register < 12-byte data, false gt",
			op1:   mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{0x0a, 0x00, 0x01, 0x21, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			op2:   mustCreateComparison(t, linux.NFT_REG_3, linux.NFT_CMP_LT, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			res:   false,
		},
		{
			tname: "compare register > 12-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{0x0a, 0x00, 0x01, 0x21, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			op2:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_GT, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			res:   true,
		},
		{
			tname: "compare register > 12-byte data, false eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			op2:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_GT, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			res:   false,
		},
		{
			tname: "compare register > 12-byte data, false lt",
			op1:   mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{0x0a, 0x00, 0x01, 0x1f, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			op2:   mustCreateComparison(t, linux.NFT_REG_2, linux.NFT_CMP_GT, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			res:   false,
		},
		{
			tname: "compare register <= 12-byte data, true lt",
			op1:   mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			op2:   mustCreateComparison(t, linux.NFT_REG_3, linux.NFT_CMP_LTE, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			res:   true,
		},
		{
			tname: "compare register <= 12-byte data, true eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			op2:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_LTE, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			res:   true,
		},
		{
			tname: "compare register <= 12-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{0xaa, 0xaa, 0xaa, 0x20, 0xaa, 0xaa, 0xaa, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			op2:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_LTE, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			res:   false,
		},
		{
			tname: "compare register >= 12-byte data, true gt",
			op1:   mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{0xaa, 0xaa, 0xaa, 0x20, 0xaa, 0xaa, 0xaa, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			op2:   mustCreateComparison(t, linux.NFT_REG_2, linux.NFT_CMP_GTE, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			res:   true,
		},
		{
			tname: "compare register >= 12-byte data, true eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{0xab, 0xbc, 0xcd, 0xde, 0xef, 0x00, 0x01, 0x12, 0x23, 0x34, 0x45, 0x56})),
			op2:   mustCreateComparison(t, linux.NFT_REG_3, linux.NFT_CMP_GTE, newBytesData([]byte{0xab, 0xbc, 0xcd, 0xde, 0xef, 0x00, 0x01, 0x12, 0x23, 0x34, 0x45, 0x56})),
			res:   true,
		},
		{
			tname: "compare register >= 12-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{0x0a, 0x00, 0x01, 0x19, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			op2:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_GTE, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00})),
			res:   false,
		},
		// 16-byte data comparisons.
		{
			tname: "compare register == 16-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_EQ, newBytesData([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare register == 16-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})),
			op2:   mustCreateComparison(t, linux.NFT_REG_2, linux.NFT_CMP_EQ, newBytesData([]byte{1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1})),
			res:   false,
		},
		{
			tname: "compare register != 16-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})),
			op2:   mustCreateComparison(t, linux.NFT_REG_3, linux.NFT_CMP_NEQ, newBytesData([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})),
			res:   true,
		},
		{
			tname: "compare register != 16-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})),
			op2:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_NEQ, newBytesData([]byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16})),
			res:   false,
		},
		{
			tname: "compare register < 16-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{0x0a, 0x00, 0x01, 0x1f, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			op2:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_LT, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			res:   true,
		},
		{
			tname: "compare register < 16-byte data, false eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			op2:   mustCreateComparison(t, linux.NFT_REG_2, linux.NFT_CMP_LT, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			res:   false,
		},
		{
			tname: "compare register < 16-byte data, false gt",
			op1:   mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{0x0a, 0x00, 0x01, 0x21, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			op2:   mustCreateComparison(t, linux.NFT_REG_3, linux.NFT_CMP_LT, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			res:   false,
		},
		{
			tname: "compare register > 16-byte data, true",
			op1:   mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{0x0a, 0x00, 0x01, 0x21, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			op2:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_GT, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			res:   true,
		},
		{
			tname: "compare register > 16-byte data, false eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			op2:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_GT, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			res:   false,
		},
		{
			tname: "compare register > 16-byte data, false lt",
			op1:   mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{0x0a, 0x00, 0x01, 0x1f, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			op2:   mustCreateComparison(t, linux.NFT_REG_2, linux.NFT_CMP_GT, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			res:   false,
		},
		{
			tname: "compare register <= 16-byte data, true lt",
			op1:   mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x86})),
			op2:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_LTE, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			res:   true,
		},
		{
			tname: "compare register <= 16-byte data, true eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			op2:   mustCreateComparison(t, linux.NFT_REG_2, linux.NFT_CMP_LTE, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			res:   true,
		},
		{
			tname: "compare register <= 16-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0xaa, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			op2:   mustCreateComparison(t, linux.NFT_REG_3, linux.NFT_CMP_LTE, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			res:   false,
		},
		{
			tname: "compare register >= 16-byte data, true gt",
			op1:   mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{0xaa, 0xaa, 0xaa, 0x20, 0xaa, 0xaa, 0xaa, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			op2:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_GTE, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			res:   true,
		},
		{
			tname: "compare register >= 16-byte data, true eq",
			op1:   mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{0xab, 0xbc, 0xcd, 0xde, 0xef, 0x00, 0x01, 0x12, 0x23, 0x34, 0x45, 0x56, 0x67, 0x78, 0x89, 0x90})),
			op2:   mustCreateComparison(t, linux.NFT_REG_3, linux.NFT_CMP_GTE, newBytesData([]byte{0xab, 0xbc, 0xcd, 0xde, 0xef, 0x00, 0x01, 0x12, 0x23, 0x34, 0x45, 0x56, 0x67, 0x78, 0x89, 0x90})),
			res:   true,
		},
		{
			tname: "compare register >= 16-byte data, false",
			op1:   mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0a, 0x13, 0x6a, 0x87})),
			op2:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_GTE, newBytesData([]byte{0x0a, 0x00, 0x01, 0x20, 0x00, 0x00, 0x0f, 0x13, 0xc0, 0x09, 0x00, 0x00, 0x0b, 0x13, 0x6a, 0x87})),
			res:   false,
		},
		// Empty register comparisons.
		{
			tname: "compare empty 4-byte register, true",
			op1:   mustCreateComparison(t, linux.NFT_REG32_10, linux.NFT_CMP_EQ, newBytesData([]byte{0, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare empty 4-byte register, false",
			op1:   mustCreateComparison(t, linux.NFT_REG32_11, linux.NFT_CMP_EQ, newBytesData([]byte{1, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare empty 8-byte register, true",
			op1:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_NEQ, newBytesData([]byte{1, 1, 1, 1, 0, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare empty 8-byte register, false",
			op1:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_GT, newBytesData([]byte{1, 1, 1, 1, 0, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare empty 12-byte register, true",
			op1:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_LTE, newBytesData([]byte{1, 1, 1, 1, 0, 0, 0, 0, 8, 9, 10, 11})),
			res:   true,
		},
		{
			tname: "compare empty 12-byte register, false",
			op1:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_NEQ, newBytesData([]byte{0, 0, 0, 0, 0, 0, 0, 0})),
			res:   false,
		},
		{
			tname: "compare empty 16-byte register, true",
			op1:   mustCreateComparison(t, linux.NFT_REG_1, linux.NFT_CMP_LT, newBytesData([]byte{1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})),
			res:   true,
		},
		{
			tname: "compare empty 16-byte register, false",
			op1:   mustCreateComparison(t, linux.NFT_REG_4, linux.NFT_CMP_GTE, newBytesData([]byte{1, 1, 1, 1, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0})),
			res:   false,
		},
	} {
		t.Run(test.tname, func(t *testing.T) {
			// Sets up an NFTables object with a single table, chain, and rule.
			nf := NewNFTables()
			tab, err := nf.AddTable(arbitraryFamily, "test", "test table", false)
			if err != nil {
				t.Fatalf("unexpected error for AddTable: %v", err)
			}
			bc, err := tab.AddChain("base_chain", nil, "test chain", false)
			if err != nil {
				t.Fatalf("unexpected error for AddChain: %v", err)
			}
			bc.SetBaseChainInfo(arbitraryInfoPolicyAccept)
			rule := &Rule{}

			// Adds testing operations.
			if test.op1 != nil {
				rule.addOperation(test.op1)
			}
			if test.op2 != nil {
				rule.addOperation(test.op2)
			}

			// Add an operation that drops. This is what the final verdict should be
			// if all the comparisons are true (res = true).
			rule.addOperation(mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)})))

			// Registers the rule to the base chain.
			if err := bc.RegisterRule(rule, -1); err != nil {
				t.Fatalf("unexpected error for RegisterRule: %v", err)
			}

			// Runs evaluation and checks verdict.
			pkt := makeTestingPacket()
			v, err := nf.EvaluateHook(arbitraryFamily, arbitraryHook, pkt)
			if err != nil {
				t.Fatalf("unexpected error for EvaluateHook: %v", err)
			}
			// If all comparisons are true, the packet will get to the end of the rule
			// and the last operation above will set the final verdict to oppose the
			// base chain policy. If any comparison is false, the comparison operation
			// will break from the rule and the final verdict will default to the base
			// chain policy.
			if test.res {
				if v.Code != VC(linux.NF_DROP) {
					t.Fatalf("expected verdict Drop for %t result, got %v", test.res, v)
				}
			} else {
				if v.Code != VC(linux.NF_ACCEPT) {
					t.Fatalf("expected base chain policy verdict Accept for %t result, got %v", test.res, v)
				}
			}
		})
	}
}

// TestLoopCheckOnRegisterAndUnregister tests the loop checking and accompanying
// logic on registering and unregistering rules.
func TestLoopCheckOnRegisterAndUnregister(t *testing.T) {
	for _, test := range []struct {
		tname     string
		chains    map[string]*Chain
		verdict   Verdict
		shouldErr bool
	}{
		{
			tname: "jump to non-existent chain",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "non_existent_chain"}))},
					}},
				},
			},
			shouldErr: true,
		},
		{
			tname: "goto to non-existent chain",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "non_existent_chain"}))},
					}},
				},
			},
			shouldErr: true,
		},
		{
			tname: "jump to itself",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "base_chain"}))},
					}},
				},
			},
			shouldErr: true,
		},
		{
			tname: "goto to itself",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "base_chain"}))},
					}},
				},
			},
			shouldErr: true,
		},
		{
			tname: "simple 2-chain loop",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain"}))},
					}},
				},
				"aux_chain": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "base_chain"}))},
					}},
				},
			},
			shouldErr: true,
		},
		{
			tname: "2-chain loop with entry point outside loop",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain"}))},
					}},
				},
				"aux_chain": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "aux_chain2"}))},
					}},
				},
				"aux_chain2": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "aux_chain"}))},
					}},
				},
			},
			shouldErr: true,
		},
		{
			tname: "simple 3-chain loop",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain"}))},
					}},
				},
				"aux_chain": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain2"}))},
					}},
				},
				"aux_chain2": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "base_chain"}))},
					}},
				},
			},
			shouldErr: true,
		},
		{
			tname: "3-chain loop with entry point 2 points outside loop",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain"}))},
					}},
				},
				"aux_chain": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "aux_chain2"}))},
					}},
				},
				"aux_chain2": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain3"}))},
					}},
				},
				"aux_chain3": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "aux_chain4"}))},
					}},
				},
				"aux_chain4": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain2"}))},
					}},
				},
			},
			shouldErr: true,
		},
		{
			tname: "simple 4-chain loop",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain"}))},
					}},
				},
				"aux_chain": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "aux_chain2"}))},
					}},
				},
				"aux_chain2": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain3"}))},
					}},
				},
				"aux_chain3": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "base_chain"}))},
					}},
				},
			},
			shouldErr: true,
		},
		{
			tname: "simple 5-chain loop",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain"}))},
					}},
				},
				"aux_chain": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "aux_chain2"}))},
					}},
				},
				"aux_chain2": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain3"}))},
					}},
				},
				"aux_chain3": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "base_chain"}))},
					}},
				},
			},
			shouldErr: true,
		},
		{
			//     0
			//  	/ \
			//   v   v
			//   1 <- 2 <-> 3
			tname: "complex 2-3 loop",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{&Rule{
						ops: []operation{
							mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain"})),
							mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain2"})),
						},
					}},
				},
				"aux_chain": &Chain{
					comment: "strictly target",
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)}))},
					}},
				},
				"aux_chain2": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{
							mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain"})),
							mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain3"})),
						},
					}},
				},
				"aux_chain3": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "aux_chain2"}))},
					}},
				},
			},
			shouldErr: true,
		},
		{
			tname: "simple loop amongst other rules and operations",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{
						&Rule{ops: []operation{mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{0, 1, 2, 3}))}},
						&Rule{ops: []operation{mustCreateImmediate(t, linux.NFT_REG32_14, newBytesData([]byte{0, 1, 2, 3}))}},
						&Rule{ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain"}))}},
					},
				},
				"aux_chain": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{
							mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)})),
							mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "aux_chain2"})),
						},
					}},
				},
				"aux_chain2": &Chain{
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain3"}))},
					}},
				},
				"aux_chain3": &Chain{
					rules: []*Rule{
						&Rule{ops: []operation{mustCreateImmediate(t, linux.NFT_REG_1, newBytesData([]byte{0, 1, 2, 3}))}},
						&Rule{ops: []operation{mustCreateImmediate(t, linux.NFT_REG32_14, newBytesData([]byte{0, 1, 2, 3}))}},
						&Rule{ops: []operation{
							mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15})),
							mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_GOTO), ChainName: "aux_chain"})),
							mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)})),
						}},
					},
				},
			},
			shouldErr: true,
		},
		{
			tname: "base chain jump to 3 other chains",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{
						&Rule{
							ops: []operation{
								mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain"})),
								mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain2"})),
							},
						},
						&Rule{ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain3"}))}},
					},
				},
				"aux_chain": &Chain{
					comment: "strictly target",
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}))},
					}},
				},
				"aux_chain2": &Chain{
					comment: "strictly target",
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}))},
					}},
				},
				"aux_chain3": &Chain{
					comment: "strictly target",
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}))},
					}},
				},
			},
			verdict: Verdict{Code: VC(linux.NF_ACCEPT)}, // from base chain policy
		},
		{
			tname: "base chain jump to 3 other chains with last chain dropping",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{
						&Rule{
							ops: []operation{
								mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain"})),
								mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain2"})),
							},
						},
						&Rule{ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain3"}))}},
					},
				},
				"aux_chain": &Chain{
					comment: "strictly target",
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}))},
					}},
				},
				"aux_chain2": &Chain{
					comment: "strictly target",
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}))},
					}},
				},
				"aux_chain3": &Chain{
					comment: "strictly target",
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)}))},
					}},
				},
			},
			verdict: Verdict{Code: VC(linux.NF_DROP)}, // from last chain
		},
		{
			tname: "base chain jump to 3 other chains with last rule in base chain dropping",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{
						&Rule{
							ops: []operation{
								mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain"})),
								mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain2"})),
							},
						},
						&Rule{ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain3"}))}},
						&Rule{ops: []operation{mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)}))}},
					},
				},
				"aux_chain": &Chain{
					comment: "strictly target",
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_2, newBytesData([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}))},
					}},
				},
				"aux_chain2": &Chain{
					comment: "strictly target",
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_3, newBytesData([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}))},
					}},
				},
				"aux_chain3": &Chain{
					comment: "strictly target",
					rules: []*Rule{&Rule{
						ops: []operation{mustCreateImmediate(t, linux.NFT_REG_4, newBytesData([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15}))},
					}},
				},
			},
			verdict: Verdict{Code: VC(linux.NF_DROP)}, // from last rule in base chain
		},
		{
			tname: "jump to the same chain",
			chains: map[string]*Chain{
				"base_chain": &Chain{
					baseChainInfo: arbitraryInfoPolicyAccept,
					rules: []*Rule{
						&Rule{
							ops: []operation{
								mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain"})),
								mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NFT_JUMP), ChainName: "aux_chain"})),
							},
						},
					},
				},
				"aux_chain": &Chain{
					comment: "strictly target",
					rules:   []*Rule{&Rule{}},
				},
			},
			verdict: Verdict{Code: VC(linux.NF_ACCEPT)}, // from base chain policy
		},
	} {
		t.Run(test.tname, func(t *testing.T) {
			// Sets up an NFTables object based on test struct.
			nf := NewNFTables()
			tab, err := nf.AddTable(arbitraryFamily, "test", "test table", false)
			if err != nil {
				t.Fatalf("unexpected error for AddTable: %v", err)
			}
			// Creates all chains in the test struct first. This is necessary so the
			// loop checking sees the target chains exist (otherwise it would error).
			for chainName, chainInit := range test.chains {
				tab.AddChain(chainName, chainInit.GetBaseChainInfo(), chainInit.GetComment(), false)
			}
			if len(test.chains) != tab.ChainCount() {
				t.Fatalf("not all chains added to table")
			}
			// Registers all rules to all chains in the test struct.
			for chainName, chainInit := range test.chains {
				chain, err := nf.GetChain(tab.GetAddressFamily(), tab.GetName(), chainName)
				if err != nil {
					t.Fatalf("unexpected error for GetChain: %v", err)
				}
				for _, rule := range chainInit.rules {
					// Note: this is where the loop checking is triggered.
					if err := chain.RegisterRule(rule, -1); err != nil {
						if !test.shouldErr {
							t.Fatalf("unexpected error for RegisterRule: %v", err)
						}
						return
					}
					// Checks that the chain was assigned to the rule.
					if rule.chain == nil {
						t.Fatalf("chain is not assigned to rule after RegisterRule")
					}
				}
				if chainInit.RuleCount() != chain.RuleCount() {
					t.Fatalf("not all rules added to chain")
				}
			}

			// Runs evaluation and checks verdict.
			pkt := makeTestingPacket()
			v, err := nf.EvaluateHook(arbitraryFamily, arbitraryHook, pkt)
			if err != nil {
				if test.verdict.ChainName != "error" {
					t.Fatalf("unexpected error for EvaluateHook: %v", err)
				}
			}
			if v.Code != test.verdict.Code {
				t.Fatalf("expected verdict %v, got %v", test.verdict, v)
			}

			// Unregisters all rules from all chains and checks that the chain is
			// unassigned from the rule.
			for chainName, chainInit := range test.chains {
				chain, err := nf.GetChain(tab.GetAddressFamily(), tab.GetName(), chainName)
				if err != nil {
					t.Fatalf("unexpected error for GetChain: %v", err)
				}
				for rIdx := chainInit.RuleCount() - 1; rIdx >= 0; rIdx-- {
					rule, err := chain.UnregisterRule(rIdx)
					if err != nil {
						t.Fatalf("unexpected error for UnregisterRule: %v", err)
					}
					if rule != chainInit.rules[rIdx] {
						t.Fatalf("rule returned by UnregisterRule does not match previously registered rule")
					}
					if rule.chain != nil {
						t.Fatalf("chain is not unassigned from rule after UnregisterRule")
					}
				}
				if chain.RuleCount() != 0 {
					t.Fatalf("not all rules removed from chain")
				}
			}
		})
	}
}

// TestMaxNestedJumps tests the limit on nested jumps (no limit for gotos).
func TestMaxNestedJumps(t *testing.T) {
	for _, test := range []struct {
		tname         string
		useJumpOp     bool
		numberOfJumps int
		verdict       Verdict // ChainName is set to "error" if an error is expected
	}{
		{
			tname:         "nested jump limit reached with jumps",
			useJumpOp:     true,
			numberOfJumps: nestedJumpLimit,
			verdict:       Verdict{Code: VC(linux.NF_DROP)},
		},
		{
			tname:         "nested jump limit reached with gotos",
			useJumpOp:     false,
			numberOfJumps: nestedJumpLimit,
			verdict:       Verdict{Code: VC(linux.NF_DROP)},
		},
		{
			tname:         "nested jump limit exceeded with jumps",
			useJumpOp:     true,
			numberOfJumps: nestedJumpLimit + 1,
			verdict:       Verdict{ChainName: "error"},
		},
		{
			tname:         "nested jump limit exceeded with gotos",
			useJumpOp:     false,
			numberOfJumps: nestedJumpLimit + 1,
			verdict:       Verdict{Code: VC(linux.NF_DROP)}, // limit only for jumps
		},
	} {
		t.Run(test.tname, func(t *testing.T) {
			// Sets up chains of nested jumps or gotos.
			nf := NewNFTables()
			tab, err := nf.AddTable(arbitraryFamily, "test", "test table", false)
			if err != nil {
				t.Fatalf("unexpected error for AddTable: %v", err)
			}
			for i := test.numberOfJumps - 1; i >= 0; i-- {
				name := fmt.Sprintf("chain %d", i)
				c, err := tab.AddChain(name, nil, "test chain", false)
				if i == 0 {
					c.SetBaseChainInfo(arbitraryInfoPolicyAccept)
				}
				if err != nil {
					t.Fatalf("unexpected error for AddChain: %v", err)
				}
				r := &Rule{}
				if i == test.numberOfJumps-1 {
					err = r.addOperation(mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: VC(linux.NF_DROP)})))
				} else {
					targetName := fmt.Sprintf("chain %d", i+1)
					code := VC(linux.NFT_JUMP)
					if !test.useJumpOp {
						code = VC(linux.NFT_GOTO)
					}
					err = r.addOperation(mustCreateImmediate(t, linux.NFT_REG_VERDICT, newVerdictData(Verdict{Code: code, ChainName: targetName})))
				}
				if err != nil {
					t.Fatalf("unexpected error for AddOperation: %v", err)
				}
				if err := c.RegisterRule(r, -1); err != nil {
					t.Fatalf("unexpected error for RegisterRule: %v", err)
				}
			}

			// Runs evaluation and checks verdict.
			pkt := makeTestingPacket()
			v, err := nf.EvaluateHook(arbitraryFamily, arbitraryHook, pkt)
			if err != nil {
				if test.verdict.ChainName != "error" {
					t.Fatalf("unexpected error for EvaluateHook: %v", err)
				}
			}
			if v.Code != test.verdict.Code {
				t.Fatalf("expected verdict %v, got %v", test.verdict, v)
			}
		})
	}
}

// packetResultString compares 2 packets by equality and returns a string
// representation.
func packetResultString(initial, final *stack.PacketBuffer) string {
	if final == nil {
		return "nil"
	}
	if reflect.DeepEqual(final, initial) {
		return "unmodified"
	}
	return "modified"
}

// mustCreateImmediate wraps the NewImmediate function for brevity.
func mustCreateImmediate(t *testing.T, dreg uint8, data registerData) *immediate {
	imm, err := newImmediate(dreg, data)
	if err != nil {
		t.Fatalf("failed to create immediate: %v", err)
	}
	return imm
}

// mustCreateComparison wraps the NewComparison function for brevity.
func mustCreateComparison(t *testing.T, sreg uint8, cop int, data registerData) *comparison {
	cmp, err := newComparison(sreg, cop, data)
	if err != nil {
		t.Fatalf("failed to create comparison: %v", err)
	}
	return cmp
}
