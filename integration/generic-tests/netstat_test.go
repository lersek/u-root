// Copyright 2021-2025 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build amd64 && !race

package integration

import (
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/Netflix/go-expect"
	"github.com/hugelgupf/vmtest/qemu"
	"github.com/hugelgupf/vmtest/qemu/qnetwork"
	"github.com/hugelgupf/vmtest/scriptvm"
	"github.com/u-root/mkuimage/uimage"
)

func TestNetstat(t *testing.T) {
	tests := []struct {
		cmd string
		exp expect.ExpectOpt
	}{
		{
			cmd: "netstat -I lo",
			exp: expect.All(
				expect.String("Kernel Interface table"),
				expect.String("Iface            MTU      Rx-OK    Rx-ERR   Rx-DRP   Rx-OVR   TX-OK    TX-ERR   TX-DRP   TX-OVR   Flg"),
				expect.String("lo               65536    2        0        0        0        2        0        0        0        LUR"),
			),
		},
		{
			cmd: "netstat -r",
			exp: expect.All(
				expect.String("Kernel IP routing table"),
				expect.String("Destination      Gateway          Genmask          Flags    MSS Window  irrt Iface"),
				expect.String("default          0.0.0.0          0.0.0.0          U        0   0          0 eth0"),
				expect.String("192.168.0.0      0.0.0.0          255.255.255.0    U        0   0          0 eth0"),
			),
		},
		{
			cmd: "netstat -s",
			exp: expect.All(
				expect.String("ip:"),
				expect.String("Forwarding is 2"),
				expect.String("Default TTL is 64"),
				expect.String("10 total packets received"),
				expect.String("0 forwarded"),
				expect.String("0 incoming packets discarded"),
				expect.String("10 incoming packets delivered"),
				expect.String("10 requests sent out"),
				expect.String("icmp:"),
				expect.String("6 ICMP messages received"),
				expect.String("0 input ICMP message failed"),
				expect.String("6 ICMP messages sent"),
				expect.String("0 ICMP messages failed"),
				expect.String("Input historam:"),
				expect.String("destination unreachable: 4"),
				expect.String("echo requests: 1"),
				expect.String("echo replies: 1"),
				expect.String("Output historam:"),
				expect.String("IcmpMsg:"),
				expect.String("InType3: 4"),
				expect.String("OutType3: 4"),
				expect.String("udp:"),
				expect.String("0 packets received"),
				expect.String("4 packets to unknown port received"),
				expect.String("0 packet receive errors"),
				expect.String("4 packets sent"),
				expect.String("0 receive buffer errors"),
				expect.String("0 send buffer errors"),
				expect.String("ipExt:"),
				expect.String("InOctets: 896"),
				expect.String("OutOctets: 896"),
				expect.String("InNoECTPkts: 10"),
			),
		},
	}

	var script strings.Builder
	fmt.Fprint(&script, `
		ip addr add 192.168.0.1/24 dev eth0
		ip link set eth0 up
		ip route add 0.0.0.0/0 dev eth0
		ping -c 1 192.168.0.1
	`)
	for _, test := range tests {
		fmt.Fprintln(&script, test.cmd)
	}

	vm := scriptvm.Start(t, "vm", script.String(),
		scriptvm.WithUimage(
			uimage.WithBusyboxCommands(
				"github.com/u-root/u-root/cmds/core/ip",
				"github.com/u-root/u-root/cmds/core/ping",
			),
			uimage.WithCoveredCommands(
				"github.com/u-root/u-root/cmds/core/netstat",
			),
		),
		scriptvm.WithQEMUFn(
			qemu.WithVMTimeout(2*time.Minute),
			qnetwork.HostNetwork("192.168.0.0/24"),
		),
	)

	for _, test := range tests {
		t.Run(test.cmd, func(t *testing.T) {
			if _, err := vm.Console.Expect(test.exp); err != nil {
				t.Errorf("VM output did not match expectations: %v", err)
			}
		})
	}

	if err := vm.Kill(); err != nil {
		t.Errorf("Kill: %v", err)
	}
	_ = vm.Wait()
}
