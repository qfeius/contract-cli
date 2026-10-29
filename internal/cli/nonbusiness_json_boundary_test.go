package cli_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"testing"

	"cn.qfei/contract-cli/internal/cli"
)

func TestNonBusinessJSONRejectsPositionalsBeforeSideEffects(t *testing.T) {
	t.Parallel()
	for _, command := range [][]string{{"config", "add"}, {"auth", "login"}, {"auth", "status"}, {"auth", "logout"}, {"auth", "use"}, {"skills", "install"}} {
		t.Run(strings.Join(command, "/"), func(t *testing.T) {
			t.Parallel()
			fixture := newNonBusinessJSONFixture(t)
			before, err := os.ReadFile(fixture.store.Path())
			if err != nil {
				t.Fatal(err)
			}
			calls := 0
			fixture.options.HTTPClient = &http.Client{Transport: roundTripFunc(func(*http.Request) (*http.Response, error) {
				calls++
				return nil, errors.New("invalid arguments reached HTTP")
			})}
			args := append(append([]string{}, command...), "unexpected-positional", "--output", "json")
			code := cli.New(fixture.options).Execute(context.Background(), args)
			if code != 1 || calls != 0 || fixture.stdout.Len() != 0 || !json.Valid(fixture.stderr.Bytes()) {
				t.Fatalf("invalid JSON command dispatched or printed text: exit=%d calls=%d stdout=%s stderr=%s", code, calls, fixture.stdout, fixture.stderr)
			}
			after, err := os.ReadFile(fixture.store.Path())
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("invalid command changed profile: %v", err)
			}
		})
	}
}
