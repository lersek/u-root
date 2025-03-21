// Copyright 2018-2025 the u-root Authors. All rights reserved
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

//go:build !race

package integration

import (
	"testing"
	"time"

	"github.com/hugelgupf/vmtest/qemu"
	"github.com/hugelgupf/vmtest/qemu/qnetwork"
	"github.com/hugelgupf/vmtest/scriptvm"
	"github.com/u-root/mkuimage/uimage"
)

func vm(t *testing.T, name, script string, net *qnetwork.InterVM) *qemu.VM {
	return scriptvm.Start(t, name, script,
		scriptvm.WithUimage(
			uimage.WithBusyboxCommands(
				"github.com/u-root/u-root/cmds/core/basename",
				"github.com/u-root/u-root/cmds/core/cat",
				"github.com/u-root/u-root/cmds/core/dirname",
				"github.com/u-root/u-root/cmds/core/echo",
				"github.com/u-root/u-root/cmds/core/grep",
				"github.com/u-root/u-root/cmds/core/ip",
				"github.com/u-root/u-root/cmds/core/kill",
				// loopback tests disabled due to https://github.com/mvdan/sh/issues/1142
				// "github.com/u-root/u-root/cmds/core/mkfifo",
				"github.com/u-root/u-root/cmds/core/seq",
				"github.com/u-root/u-root/cmds/core/shasum",
				"github.com/u-root/u-root/cmds/core/sleep",
			),
			uimage.WithCoveredCommands(
				"github.com/u-root/u-root/cmds/core/netcat",
			),
		),
		scriptvm.WithQEMUFn(
			qemu.WithVMTimeout(4*time.Minute),
			net.NewVM(),
		),
	)
}

func TestNetcat(t *testing.T) {
	net := qnetwork.NewInterVM()

	serverScript := `
		ip addr add 192.168.0.2/24 dev eth0
		ip link set eth0 up
		ip route add 0.0.0.0/0 dev eth0
		echo "192.168.0.1 netcat_client" >>/etc/hosts
		echo "192.168.0.2 netcat_server" >>/etc/hosts

		# loopback tests disabled due to https://github.com/mvdan/sh/issues/1142
		#
		# mkfifo fifo
		#
		# # TCPv4 server: loopback
		# : >fifo &
		# netcat -l 192.168.0.2 5005 <fifo >fifo &
		#
		# # TCPv4 server: checksum
		# seq -w 0 99999 | netcat -l 192.168.0.2 5006 | shasum >5006.out &

		# accept file from TCPv4 client
		netcat -l 192.168.0.2 5007 </dev/null | shasum >5007.out &

		# send file to TCPv4 client
		seq -w 0 99999 | netcat -l 192.168.0.2 5008 &

		# exchange files with TCPv4 client
		seq -w 0 99999 | netcat -l 192.168.0.2 5009 | shasum >5009.out &

		wait

		# loopback tests disabled due to https://github.com/mvdan/sh/issues/1142
		# grep -q a7ffaef825af40e08daef5a1e0804d851904b5aa 5006.out

		# verify files from TCPv4 client
		grep -q a7ffaef825af40e08daef5a1e0804d851904b5aa 5007.out
		grep -q a7ffaef825af40e08daef5a1e0804d851904b5aa 5009.out


		# run TCPv4 server in keep-open mode for about 60 seconds
		# run TCPv4 server in broker (chat) mode for about 60 seconds
		netcat -l -k 192.168.0.2 5010 </dev/null | shasum >5010.out &
		netcat -l --chat 192.168.0.2 5011 >5011.out &
		sleep 60
		grep -l netcat /proc/*/comm |
			while read P; do
				kill $(basename $(dirname $P))
			done
		wait


		# verify output from keep-open mode server
		grep -q a7ffaef825af40e08daef5a1e0804d851904b5aa 5010.out

		# verify output from chat server
		expected=$(
			echo 'user<1>: hello-1'
			echo 'user<2>: hello-2'
			echo 'user<3>: hello-3'
		)
		got=$(cat 5011.out)
		test "$expected" = "$got"
	`
	clientScript := `
		ip addr add 192.168.0.1/24 dev eth0
		ip link set eth0 up
		ip route add 0.0.0.0/0 dev eth0
		echo "192.168.0.1 netcat_client" >>/etc/hosts
		echo "192.168.0.2 netcat_server" >>/etc/hosts

		# wait a bit for the server to come up
		sleep 2

		# loopback tests disabled due to https://github.com/mvdan/sh/issues/1142
		#
		# mkfifo fifo
		#
		# # TCPv4 client: checksum
		# seq -w 0 99999 | netcat 192.168.0.2 5005 | shasum >5005.out
		# grep -q a7ffaef825af40e08daef5a1e0804d851904b5aa 5005.out
		#
		# # TCPv4 client: loopback
		# : >fifo &
		# netcat 192.168.0.2 5006 <fifo >fifo
		#
		# # unix server: loopback
		# : >fifo &
		# netcat -l -U stream.sock <fifo >fifo &
		# sleep 1
		#
		# # unix client: checksum
		# seq -w 0 99999 | netcat -U stream.sock | shasum >stream.client.out
		# grep -q a7ffaef825af40e08daef5a1e0804d851904b5aa stream.client.out
		# wait
		# rm stream.sock
		#
		# # unix server: checksum
		# seq -w 0 99999 | netcat -l -U stream.sock | shasum >stream.server.out &
		# sleep 1
		#
		# # unix client: loopback
		# : >fifo &
		# netcat -U stream.sock <fifo >fifo
		#
		# wait
		# rm stream.sock
		# grep -q a7ffaef825af40e08daef5a1e0804d851904b5aa stream.server.out

		# upload file to TCPv4 server
		seq -w 0 99999 | netcat 192.168.0.2 5007

		# download file from TCPv4 server
		netcat 192.168.0.2 5008 </dev/null | shasum >5008.out

		# exchange files with TCPv4 server
		seq -w 0 99999 | netcat 192.168.0.2 5009 | shasum >5009.out

		# verify files from TCPv4 server
		grep -q a7ffaef825af40e08daef5a1e0804d851904b5aa 5008.out
		grep -q a7ffaef825af40e08daef5a1e0804d851904b5aa 5009.out


		# wait a bit until the keep-open and chat servers start up
		sleep 2

		# upload file in two parts to keep-open server
		seq -w     0 49999 | netcat 192.168.0.2 5010
		seq -w 50000 99999 | netcat 192.168.0.2 5010

		# Connect with three clients to the chat server in a predefined order (at 0,
		# 3, and 6 seconds), and once they're all connected (which happens slightly
		# after the 6 second mark), make them send strings in a predefined order (at
		# 12, 15, and 18 seconds from the start). Each client lingers until the 24
		# second mark (so that everyone can hear everyone).
		(sleep 12; echo hello-1; sleep 12) | netcat 192.168.0.2 5011 >5011-1.out &
		sleep 3
		(sleep 12; echo hello-2; sleep  9) | netcat 192.168.0.2 5011 >5011-2.out &
		sleep 3
		(sleep 12; echo hello-3; sleep  6) | netcat 192.168.0.2 5011 >5011-3.out &
		wait

		# verify output from each chat client
		expected1=$(
			echo 'user<2>: hello-2'
			echo 'user<3>: hello-3'
		)
		expected2=$(
			echo 'user<1>: hello-1'
			echo 'user<3>: hello-3'
		)
		expected3=$(
			echo 'user<1>: hello-1'
			echo 'user<2>: hello-2'
		)
		got1=$(cat 5011-1.out)
		got2=$(cat 5011-2.out)
		got3=$(cat 5011-3.out)
		test "$expected1" = "$got1"
		test "$expected2" = "$got2"
		test "$expected3" = "$got3"


		# accept file from unix client
		netcat -l -U stream-1.sock </dev/null | shasum >stream-1.server.out &

		# send file to unix client
		seq -w 0 99999 | netcat -l -U stream-2.sock &

		# exchange files with unix client
		seq -w 0 99999 | netcat -l -U stream-3.sock | shasum >stream-3.server.out &

		sleep 1

		# upload file to unix server
		seq -w 0 99999 | netcat -U stream-1.sock

		# download file from unix server
		netcat -U stream-2.sock </dev/null | shasum >stream-2.client.out

		# exchange files with unix server
		seq -w 0 99999 | netcat -U stream-3.sock | shasum >stream-3.client.out

		# verify files from unix client
		wait
		grep -q a7ffaef825af40e08daef5a1e0804d851904b5aa stream-1.server.out
		grep -q a7ffaef825af40e08daef5a1e0804d851904b5aa stream-3.server.out

		# verify files from unix server
		grep -q a7ffaef825af40e08daef5a1e0804d851904b5aa stream-2.client.out
		grep -q a7ffaef825af40e08daef5a1e0804d851904b5aa stream-3.client.out
	`

	serverVM := vm(t, "netcat_server", serverScript, net)
	clientVM := vm(t, "netcat_client", clientScript, net)

	if _, err := serverVM.Console.ExpectString("TESTS PASSED MARKER"); err != nil {
		t.Errorf("serverVM: %v", err)
	}
	if _, err := clientVM.Console.ExpectString("TESTS PASSED MARKER"); err != nil {
		t.Errorf("clientVM: %v", err)
	}

	clientVM.Wait()
	serverVM.Wait()
}
