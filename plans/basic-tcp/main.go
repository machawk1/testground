package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/ipfs/testground/sdk/runtime"
	sdksync "github.com/ipfs/testground/sdk/sync"
)

var (
	MetricBytesSent     = &runtime.MetricDefinition{Name: "sent_bytes", Unit: "bytes", ImprovementDir: -1}
	MetricBytesReceived = &runtime.MetricDefinition{Name: "received_bytes", Unit: "bytes", ImprovementDir: -1}
	MetricTimeToSend    = &runtime.MetricDefinition{Name: "time_to_send", Unit: "ms", ImprovementDir: -1}
	MetricTimeToReceive = &runtime.MetricDefinition{Name: "time_to_receive", Unit: "ms", ImprovementDir: -1}
)

var peerIPSubtree = &sdksync.Subtree{
	GroupKey:    "peerIPs",
	PayloadType: reflect.TypeOf(&net.IP{}),
	KeyFunc: func(val interface{}) string {
		return val.(*net.IP).String()
	},
}

func main() {
	runenv := runtime.CurrentRunEnv()
	withShaping := runenv.TestCaseSeq == 1

	ctx := context.Background()

	if withShaping && !runenv.TestSidecar {
		runenv.Abort("Need sidecar to shape traffic")
		return
	}

	ifaddrs, err := net.InterfaceAddrs()
	if err != nil {
		runenv.Abort(err)
		return
	}

	_, localnet, _ := net.ParseCIDR("8.0.0.0/8")

	var peerIP net.IP
	for _, ifaddr := range ifaddrs {
		var ip net.IP
		switch v := ifaddr.(type) {
		case *net.IPNet:
			ip = v.IP
		case *net.IPAddr:
			ip = v.IP
		}
		if localnet.Contains(ip) {
			peerIP = ip
			break
		}
	}
	if peerIP == nil {
		runenv.Abort("No IP match")
		return
	}

	watcher, writer := sdksync.MustWatcherWriter(runenv)
	defer watcher.Close()

	if withShaping {
		runenv.Message("Waiting for network to be initialized")
		if err := sdksync.WaitNetworkInitialized(ctx, runenv, watcher); err != nil {
			runenv.Abort(err)
			return
		}
		runenv.Message("Network initialized")

		config := sdksync.NetworkConfig{
			// Control the "default" network. At the moment, this is the only network.
			Network: "default",

			// Enable this network. Setting this to false will disconnect this test
			// instance from this network. You probably don't want to do that.
			Enable: true,
		}
		config.Default = sdksync.LinkShape{
			// Latency:   100 * time.Millisecond,
			Bandwidth: 1 << 20, // 1Mib
		}
		config.State = "network-configured"

		hostname, err := os.Hostname()
		if err != nil {
			runenv.Abort(err)
			return
		}

		_, err = writer.Write(sdksync.NetworkSubtree(hostname), &config)
		if err != nil {
			runenv.Abort(err)
			return
		}

		err = <-watcher.Barrier(ctx, config.State, int64(runenv.TestInstanceCount))
		if err != nil {
			runenv.Abort(err)
			return
		}
		runenv.Message("Network configured")
	}

	seq, err := writer.Write(peerIPSubtree, &peerIP)
	if err != nil {
		runenv.Abort(err)
		return
	}
	defer writer.Close()

	// States
	ready := sdksync.State("ready")
	received := sdksync.State("received")

	switch {
	case seq == 1: // receiver
		runenv.Message(fmt.Sprintf("Receiver: %v", peerIP))

		quit := make(chan int)

		l, err := net.Listen("tcp", peerIP.String()+":2000")
		if err != nil {
			runenv.Abort(err)
			return
		}
		defer func() {
			close(quit)
			l.Close()
		}()

		// Signal we're ready
		_, err = writer.SignalEntry(ready)
		if err != nil {
			runenv.Abort(err)
			return
		}
		// runenv.Message("State: ready")

		var wg sync.WaitGroup
		wg.Add(runenv.TestInstanceCount - 1)
		// runenv.Message(fmt.Sprintf("Waiting for connections: %v", runenv.TestInstanceCount-1))

		go func() {
			for {
				conn, err := l.Accept()
				if err != nil {
					select {
					case <-quit:
						// runenv.Message("Accepted, but quitting")
						return
					default:
						runenv.Abort(err)
						return
					}
				}
				// runenv.Message("Accepted")
				go func(c net.Conn) {
					defer c.Close()
					bytesRead := 0
					buf := make([]byte, 128*1024)
					tstarted := time.Now()
					for {
						n, err := c.Read(buf)
						bytesRead += n
						// runenv.Message(fmt.Sprintf("Received %v", n))
						if err == io.EOF {
							// runenv.Message("EOF")
							break
						} else if err != nil {
							fmt.Fprintln(os.Stderr, "Error", err)
							wg.Done()
							return
						}
					}
					runenv.EmitMetric(MetricBytesReceived, float64(bytesRead))
					runenv.EmitMetric(MetricTimeToReceive, float64(time.Now().Sub(tstarted)/time.Millisecond))
					wg.Done()
				}(conn)
			}
		}()

		wg.Wait()

		// Signal we've received all the data
		_, err = writer.SignalEntry(received)
		if err != nil {
			runenv.Abort(err)
			return
		}
		// runenv.Message("State: received")

	case seq == 2: // sender
		runenv.Message(fmt.Sprintf("Sender: %v", peerIP))

		// Connect to other peers
		peerIPCh := make(chan *net.IP, 16)
		cancel, err := watcher.Subscribe(peerIPSubtree, peerIPCh)
		if err != nil {
			runenv.Abort(err)
		}
		defer cancel()

		var peerIPsToDial = make([]net.IP, 0)
		for i := 0; i < runenv.TestInstanceCount; i++ {
			receivedPeerIP := <-peerIPCh
			if receivedPeerIP.String() == peerIP.String() {
				continue
			}
			peerIPsToDial = append(peerIPsToDial, *receivedPeerIP)
		}

		// Wait until ready state is signalled.
		// runenv.Message("Waiting for ready")
		readyCh := watcher.Barrier(ctx, ready, 1)
		if err := <-readyCh; err != nil {
			panic(err)
		}
		// runenv.Message("State: ready")

		for _, peerIPToDial := range peerIPsToDial {
			// runenv.Message(fmt.Sprintf("Dialing %v", peerIPToDial))
			conn, err := net.Dial("tcp", peerIPToDial.String()+":2000")
			if err != nil {
				fmt.Fprintln(os.Stderr, "Error", err)
				return
			}
			buf := make([]byte, 100*1024)
			for i := 0; i < len(buf); i++ {
				buf[i] = byte(i)
			}
			bytesWritten := 0
			tstarted := time.Now()
			for i := 0; i < 10; i++ {
				n, err := conn.Write(buf)
				// runenv.Message(fmt.Sprintf("Sent %v", n))
				bytesWritten += n
				if err != nil {
					fmt.Fprintln(os.Stderr, "Error", err)
					break
				}
			}
			runenv.EmitMetric(MetricBytesSent, float64(bytesWritten))
			runenv.EmitMetric(MetricTimeToSend, float64(time.Now().Sub(tstarted)/time.Millisecond))
			conn.Close()
		}

		// Wait until all data is received before shutting down
		// runenv.Message("Waiting for received state")
		receivedCh := watcher.Barrier(ctx, received, 1)
		if err := <-receivedCh; err != nil {
			panic(err)
		}
		// runenv.Message("State: received")

	default:
		runenv.Abort(fmt.Errorf("Unexpected seq: %v", seq))
	}

	runenv.OK()
}
