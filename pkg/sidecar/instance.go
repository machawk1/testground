//+build linux

package sidecar

import (
	"context"
	"fmt"
	"io"

	"github.com/hashicorp/go-multierror"

	"github.com/ipfs/testground/pkg/logging"
	"github.com/ipfs/testground/sdk/runtime"
	"github.com/ipfs/testground/sdk/sync"
)

// Instance is a test instance as seen by the sidecar.
type Instance struct {
	logging.Logging

	Hostname string
	Watcher  *sync.Watcher
	Writer   *sync.Writer
	RunEnv   *runtime.RunEnv
	Network  Network
	Logs     Logs
}

// Network is a test instance's network, as seen by the sidecar.
//
// Sidecar runners must implement this interface.
type Network interface {
	io.Closer
	ConfigureNetwork(ctx context.Context, cfg *sync.NetworkConfig) error
	ListActive() []string
}

// Logs are logs from a test instance.
type Logs interface {
	io.Closer
	Stderr() io.Reader
	Stdout() io.Reader
}

// NewInstance constructs a new test instance handle.
func NewInstance(ctx context.Context, runenv *runtime.RunEnv, hostname string, network Network, logs Logs) (*Instance, error) {
	// Get a redis reader/writer.
	watcher, writer, err := sync.WatcherWriter(ctx, runenv)
	if err != nil {
		return nil, fmt.Errorf("during sync.WatcherWriter: %w", err)
	}

	return &Instance{
		Logging:  logging.NewLogging(runenv.SLogger().With("sidecar", true).Desugar()),
		Hostname: hostname,
		RunEnv:   runenv,
		Network:  network,
		Logs:     logs,
		Watcher:  watcher,
		Writer:   writer,
	}, nil
}

// Close closes the instance. It should not be used after closing.
func (inst *Instance) Close() error {
	var err *multierror.Error
	err = multierror.Append(err, inst.Watcher.Close())
	err = multierror.Append(err, inst.Writer.Close())
	err = multierror.Append(err, inst.Network.Close())
	err = multierror.Append(err, inst.Logs.Close())
	return err.ErrorOrNil()
}
