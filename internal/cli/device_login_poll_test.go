package cli

import (
	"context"
	"errors"
	"reflect"
	"testing"
	"time"
)

func TestDeviceLoginPollHonorsIntervalAndSlowDown(t *testing.T) {
	var waits []time.Duration
	outputs := []deviceAuthOutput{{Status: "pending"}, {Status: "pending", slowDown: true}, {Status: "pending"}, {Status: "succeeded"}}
	index := 0
	err := pollDeviceLogin(context.Background(), 7*time.Second, func(_ context.Context, interval time.Duration) error {
		waits = append(waits, interval)
		return nil
	}, func(context.Context) (deviceAuthOutput, error) {
		output := outputs[index]
		index++
		return output, nil
	})
	if err != nil || !reflect.DeepEqual(waits, []time.Duration{7 * time.Second, 7 * time.Second, 12 * time.Second, 12 * time.Second}) {
		t.Fatalf("waits=%v error=%v", waits, err)
	}
}

func TestDeviceLoginPollStopsOnEveryTerminalState(t *testing.T) {
	for _, status := range []string{"denied", "expired", "uncertain", "restart_required", "busy", "unexpected", ""} {
		t.Run(status, func(t *testing.T) {
			calls := 0
			err := pollDeviceLogin(context.Background(), time.Second, func(context.Context, time.Duration) error { return nil }, func(context.Context) (deviceAuthOutput, error) {
				calls++
				return deviceAuthOutput{Status: status}, nil
			})
			if err == nil || calls != 1 {
				t.Fatalf("calls=%d error=%v", calls, err)
			}
		})
	}
}

func TestDeviceLoginPollStopsOnTransportOrCancellation(t *testing.T) {
	failure := errors.New("network failure")
	err := pollDeviceLogin(context.Background(), time.Second, func(context.Context, time.Duration) error { return nil }, func(context.Context) (deviceAuthOutput, error) {
		return deviceAuthOutput{}, failure
	})
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = pollDeviceLogin(ctx, time.Second, waitDeviceLoginInterval, func(context.Context) (deviceAuthOutput, error) {
		t.Fatal("canceled wait made a token request")
		return deviceAuthOutput{}, nil
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
}

func TestDeviceLoginIntervalBounds(t *testing.T) {
	for _, sample := range []struct {
		seconds int64
		want    time.Duration
	}{
		{0, 5 * time.Second}, {-1, 5 * time.Second}, {1, time.Second}, {12, 12 * time.Second}, {1 << 62, 5 * time.Minute},
	} {
		if got := deviceLoginInterval(sample.seconds); got != sample.want {
			t.Fatalf("interval(%d)=%v", sample.seconds, got)
		}
	}
}
