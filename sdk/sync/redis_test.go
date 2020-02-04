package sync

import (
	"context"
	"fmt"
	"math/rand"
	"os/exec"
	"reflect"
	"testing"
	"time"

	"github.com/ipfs/testground/sdk/runtime"

	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
)

func init() {
	// Avoid collisions in Redis keys over test runs.
	rand.Seed(time.Now().UnixNano())
}

// Check if there's a running instance of redis, or start it otherwise. If we
// start an ad-hoc instance, the close function will terminate it.
func ensureRedis(t *testing.T) (close func()) {
	t.Helper()

	runenv := runtime.RandomRunEnv()

	// Try to obtain a client; if this fails, we'll attempt to start a redis
	// instance.
	client, err := redisClient(context.Background(), runenv)
	if err == nil {
		return func() {}
	}

	cmd := exec.Command("redis-server", "-")
	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start redis: %s", err)
	}

	time.Sleep(1 * time.Second)

	// Try to obtain a client again.
	if client, err = redisClient(context.Background(), runenv); err != nil {
		t.Fatalf("failed to obtain redis client despite starting instance: %v", err)
	}
	defer client.Close()

	return func() {
		if err := cmd.Process.Kill(); err != nil {
			t.Fatalf("failed while stopping test-scoped redis: %s", err)
		}
	}
}

func TestWatcherWriter(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	close := ensureRedis(t)
	defer close()

	runenv := runtime.RandomRunEnv()

	watcher, err := NewWatcher(ctx, runenv)
	if err != nil {
		t.Fatal(err)
	}
	defer watcher.Close()

	peersCh := make(chan *peer.AddrInfo, 16)
	err = watcher.Subscribe(ctx, PeerSubtree, peersCh)
	if err != nil {
		t.Fatal(err)
	}
	defer cancel()

	if err != nil {
		t.Fatal(err)
	}

	writer, err := NewWriter(ctx, runenv)
	if err != nil {
		t.Fatal(err)
	}

	ma, err := multiaddr.NewMultiaddr("/ip4/1.2.3.4/tcp/8001/p2p/QmeiLa9HDf5B47utrZHQ1TLcotvCyk2AeVqJrMGRpH5zLu")
	if err != nil {
		t.Fatal(err)
	}

	ai, err := peer.AddrInfoFromP2pAddr(ma)
	if err != nil {
		t.Fatal(err)
	}

	writer.Write(ctx, PeerSubtree, ai)
	if err != nil {
		t.Fatal(err)
	}

	select {
	case ai = <-peersCh:
		fmt.Println(ai)
	case <-time.After(5 * time.Second):
		t.Fatal("no event received within 5 seconds")
	}

}

func TestBarrier(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	close := ensureRedis(t)
	defer close()

	runenv := runtime.RandomRunEnv()

	watcher, writer := MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	state := State("yoda")
	ch := watcher.Barrier(ctx, state, 10)

	for i := 1; i <= 10; i++ {
		if curr, err := writer.SignalEntry(ctx, state); err != nil {
			t.Fatal(err)
		} else if curr != int64(i) {
			t.Fatalf("expected current count to be: %d; was: %d", i, curr)
		}
	}

	if err := <-ch; err != nil {
		t.Fatal(err)
	}
}

// TestWatchInexistentKeyThenWrite starts watching a subtree that doesn't exist
// yet.
func TestWatchInexistentKeyThenWrite(t *testing.T) {
	var (
		length  = 1000
		values  = generateValues(length)
		runenv  = runtime.RandomRunEnv()
		subtree = randomTestSubtree()
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	closeRedis := ensureRedis(t)
	defer closeRedis()

	watcher, writer := MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	ch := make(chan *string, 128)
	err := watcher.Subscribe(ctx, subtree, ch)
	if err != nil {
		t.Fatal(err)
	}

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		consumeOrdered(t, ctx, ch, values)
	}()

	produce(t, writer, subtree, values)

	<-doneCh
}

func TestWriteAllBeforeWatch(t *testing.T) {
	var (
		length  = 1000
		values  = generateValues(length)
		runenv  = runtime.RandomRunEnv()
		subtree = randomTestSubtree()
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	closeRedis := ensureRedis(t)
	defer closeRedis()

	watcher, writer := MustWatcherWriter(ctx, runenv)
	defer watcher.Close()
	defer writer.Close()

	produce(t, writer, subtree, values)

	ch := make(chan *string, 128)
	err := watcher.Subscribe(ctx, subtree, ch)
	if err != nil {
		t.Fatal(err)
	}

	doneCh := make(chan struct{})
	go func() {
		defer close(doneCh)

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		consumeUnordered(t, ctx, ch, values)
	}()

	<-doneCh
}

func TestSequenceOnWrite(t *testing.T) {
	var (
		iterations = 1000
		runenv     = runtime.RandomRunEnv()
		subtree    = randomTestSubtree()
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	closeRedis := ensureRedis(t)
	defer closeRedis()

	s := "a"
	for i := 1; i <= iterations; i++ {
		w, err := NewWriter(ctx, runenv)
		if err != nil {
			t.Fatal(err)
		}

		seq, err := w.Write(ctx, subtree, &s)
		if err != nil {
			t.Fatal(err)
		}

		if seq != int64(i) {
			t.Fatalf("expected seq %d, got %d", i, seq)
		}

		w.Close()
	}
}

func TestCloseSubscription(t *testing.T) {
	close := ensureRedis(t)
	defer close()

	var (
		runenv  = runtime.RandomRunEnv()
		subtree = randomTestSubtree()
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	watcher, writer := MustWatcherWriter(ctx, runenv)

	sctx, scancel := context.WithCancel(ctx)

	ch := make(chan *string, 128)
	err := watcher.Subscribe(sctx, subtree, ch)
	if err != nil {
		t.Fatal(err)
	}

	s := "foo"
	if _, err := writer.Write(ctx, subtree, &s); err != nil {
		t.Fatal(err)
	}

	v, ok := <-ch
	if !ok && *v != s {
		t.Fatalf("expected channel to be open, and v to be %s; was: %s", s, *v)
	}

	// cancel the subscription.
	scancel()

	v, ok = <-ch
	if ok && *v != "" {
		t.Fatalf("expected channel to be closed, and v to be empty; was: %s", *v)
	}
}

func consumeOrdered(t *testing.T, ctx context.Context, ch chan *string, values []string) {
	t.Helper()

	for i, expected := range values {
		select {
		case val := <-ch:
			if *val != expected {
				t.Fatalf("expected value %s, got %s in position %d", expected, *val, i)
			}
		case <-ctx.Done():
			t.Fatal("failed to receive all expected items within 10 seconds")
		}
	}
}

func consumeUnordered(t *testing.T, ctx context.Context, ch chan *string, values []string) {
	t.Helper()

	uniq := make(map[string]struct{}, len(values))

	for range values {
		select {
		case val := <-ch:
			uniq[*val] = struct{}{}
		case <-ctx.Done():
			t.Fatal("failed to receive all expected items within 10 seconds")
		}
	}

	// we've received len(values) values; check the size of the unique index
	// matches.
	if len(uniq) != len(values) {
		t.Fatalf("failed to receive %d unique elements; got: %d", len(values), len(uniq))
	}
}

func produce(t *testing.T, writer *Writer, subtree *Subtree, values []string) {
	for i, s := range values {
		if seq, err := writer.Write(context.Background(), subtree, &s); err != nil {
			t.Fatalf("failed while writing key to subtree: %s", err)
		} else if seq != int64(i)+1 {
			t.Fatalf("expected seq == i+1; seq: %d; i: %d", seq, i)
		}
	}
}

func generateValues(length int) []string {
	values := make([]string, 0, length)
	for i := 0; i < length; i++ {
		values = append(values, fmt.Sprintf("item-%d", i))
	}
	return values
}

func randomTestSubtree() *Subtree {
	return &Subtree{
		GroupKey:    fmt.Sprintf("test-%d", rand.Int()),
		PayloadType: reflect.TypeOf((*string)(nil)),
		KeyFunc:     func(payload interface{}) string { return *payload.(*string) },
	}
}
