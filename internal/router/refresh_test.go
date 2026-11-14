package router

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"testing/synctest"
)

func TestRefreshSharesOnlyInFlightState(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		finish := make(chan struct{})
		calls := 0
		client := NewClient(runnerFunc(func(context.Context, string) ([]byte, error) {
			calls++
			if calls == 1 {
				<-finish
			}
			return fmt.Appendf(nil, `{"network":[{"name":"lan","type":"interface","options":{"metric":"%d"}}]}`, calls), nil
		}), nil)
		selector := Section{Package: "network", Name: "lan", Type: "interface"}
		var reads sync.WaitGroup
		for range 8 {
			reads.Go(func() {
				actual, found, err := client.ReadSection(t.Context(), selector)
				if err != nil || !found || actual.Options["metric"] != "1" {
					t.Errorf("Concurrent refresh = %+v, %v, %v", actual, found, err)
				}
				actual.Options["metric"] = "local-only"
			})
		}
		synctest.Wait()
		close(finish)
		reads.Wait()
		actual, found, err := client.ReadSection(t.Context(), selector)
		if err != nil || !found || actual.Options["metric"] != "2" || calls != 2 {
			t.Fatalf("Next refresh = %+v, %v, %v after %d requests", actual, found, err, calls)
		}
	})
}

func TestRefreshWaiterReadsAgainWhenFirstCallerCancels(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		calls := 0
		client := NewClient(runnerFunc(func(ctx context.Context, _ string) ([]byte, error) {
			calls++
			if calls == 1 {
				<-ctx.Done()
				return nil, errors.New("SSH operation did not complete")
			}
			return []byte(`{"network":[{"name":"lan","type":"interface"}]}`), nil
		}), nil)
		selector := Section{Package: "network", Name: "lan", Type: "interface"}
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		var reads sync.WaitGroup
		reads.Go(func() {
			if _, _, err := client.ReadSection(ctx, selector); !errors.Is(err, context.Canceled) {
				t.Errorf("First read error = %v, want context canceled", err)
			}
		})
		synctest.Wait()
		reads.Go(func() {
			if _, found, err := client.ReadSection(t.Context(), selector); !found || err != nil {
				t.Errorf("Waiting read = %v, %v", found, err)
			}
		})
		synctest.Wait()
		cancel()
		reads.Wait()
		if calls != 2 {
			t.Fatalf("Read %d snapshots, want 2", calls)
		}
	})
}
