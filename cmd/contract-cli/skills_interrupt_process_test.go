//go:build linux || darwin

package main

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"testing/fstest"
	"time"

	"cn.qfei/contract-cli/internal/cli"
	"cn.qfei/contract-cli/internal/config"
	"cn.qfei/contract-cli/internal/skillsync"
)

func TestSkillsInstallSIGINTDuringStagingPreservesPreviousInstallation(t *testing.T) {
	if os.Getenv("QFEI_SKILLS_INTERRUPT_HELPER") == "1" {
		runSkillsInterruptProcessHelper()
		return
	}
	state, target := filepath.Join(t.TempDir(), "state"), filepath.Join(t.TempDir(), "skills")
	initial := fstest.MapFS{
		"a-slow/SKILL.md":  {Data: []byte("---\nname: a-slow\ndescription: old slow fixture\n---\nold slow\n")},
		"a-slow/old.txt":   {Data: []byte("keep old slow resource\n")},
		"z-later/SKILL.md": {Data: []byte("---\nname: z-later\ndescription: old later fixture\n---\nold later\n")},
		"z-later/old.txt":  {Data: []byte("keep old later resource\n")},
	}
	service := skillsync.Service{StateDir: state, Source: initial, Version: "1.0.0-before-interrupt"}
	result, err := service.Install(target, []string{"a-slow", "z-later"}, true, true)
	if err != nil || !result.Confirmed {
		t.Fatalf("prepare previous installation: result=%+v error=%v", result, err)
	}
	thirdParty := filepath.Join(target, "third-party", "keep.txt")
	if err := os.MkdirAll(filepath.Dir(thirdParty), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(thirdParty, []byte("unrelated user file\n"), 0600); err != nil {
		t.Fatal(err)
	}
	before := skillsInterruptTreeSnapshot(t, target)
	registryPath := filepath.Join(state, "skill-locations.json")
	registryBefore, err := os.ReadFile(registryPath)
	if err != nil {
		t.Fatal(err)
	}

	releaseReader, releaseWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = releaseReader.Close(); _ = releaseWriter.Close() }()
	eventReader, eventWriter, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = eventReader.Close(); _ = eventWriter.Close() }()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	command := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestSkillsInstallSIGINTDuringStagingPreservesPreviousInstallation$")
	command.Env = append(os.Environ(), "QFEI_SKILLS_INTERRUPT_HELPER=1", "QFEI_SKILLS_INTERRUPT_STATE="+state, "QFEI_SKILLS_INTERRUPT_TARGET="+target)
	command.ExtraFiles = []*os.File{releaseReader, eventWriter}
	stdout, stderr := &bytes.Buffer{}, &bytes.Buffer{}
	command.Stdout, command.Stderr = stdout, stderr
	if err := command.Start(); err != nil {
		t.Fatal(err)
	}
	_ = releaseReader.Close()
	_ = eventWriter.Close()
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	exited := false
	defer func() {
		if !exited {
			_ = command.Process.Kill()
			<-done
		}
	}()
	events := make(chan string, 4)
	go func() {
		defer close(events)
		scanner := bufio.NewScanner(eventReader)
		for scanner.Scan() {
			events <- scanner.Text()
		}
	}()
	awaitEvent := func(want string) {
		t.Helper()
		select {
		case got := <-events:
			if got != want {
				t.Fatalf("helper event=%q, want %q", got, want)
			}
		case <-ctx.Done():
			t.Fatalf("helper did not reach %q: %v", want, ctx.Err())
		}
	}
	awaitEvent("staging-resource")
	if err := command.Process.Signal(os.Interrupt); err != nil {
		t.Fatal(err)
	}
	// Wait for the real signal context to be cancelled before releasing the
	// resource read. The source deliberately returns valid bytes afterwards.
	awaitEvent("signal-cancelled")
	if _, err := releaseWriter.Write([]byte{1}); err != nil {
		t.Fatal(err)
	}
	_ = releaseWriter.Close()
	select {
	case err := <-done:
		exited = true
		var exitError *exec.ExitError
		if !errors.As(err, &exitError) || exitError.ExitCode() != 130 {
			t.Errorf("installation interrupt must exit 130: %v, stderr=%s", err, stderr)
		}
	case <-ctx.Done():
		t.Fatalf("interrupted installation did not finish: %v", ctx.Err())
	}
	if stdout.Len() != 0 {
		t.Errorf("cancelled JSON installation emitted success/text output: %s", stdout)
	}
	var failure struct {
		OK    *bool `json:"ok"`
		Error struct {
			Type      string `json:"type"`
			Cancelled bool   `json:"cancelled"`
		} `json:"error"`
	}
	if err := json.Unmarshal(stderr.Bytes(), &failure); err != nil || failure.OK == nil || *failure.OK || failure.Error.Type != "cancelled" || !failure.Error.Cancelled {
		t.Errorf("cancelled JSON installation lost its error: %v, stderr=%s", err, stderr)
	}
	after := skillsInterruptTreeSnapshot(t, target)
	if len(after) != len(before) {
		t.Errorf("interrupted staging changed live file count: before=%v after=%v", before, after)
	}
	for name, content := range before {
		if !bytes.Equal(after[name], content) {
			t.Errorf("interrupted staging changed existing file %q: before=%q after=%q", name, content, after[name])
		}
	}
	for _, name := range []string{"a-slow/resource.bin", "z-later/new.txt"} {
		if _, exists := after[name]; exists {
			t.Errorf("interrupted staging installed subsequent resource %q", name)
		}
	}
	registryAfter, err := os.ReadFile(registryPath)
	if err != nil || !bytes.Equal(registryBefore, registryAfter) {
		t.Errorf("interrupted staging advanced registration: error=%v before=%s after=%s", err, registryBefore, registryAfter)
	}
}

func runSkillsInterruptProcessHelper() {
	ctx, stop := newInterruptContext(context.Background())
	events := os.NewFile(4, "skills-interrupt-events")
	release := os.NewFile(3, "skills-interrupt-release")
	go func() {
		<-ctx.Done()
		_, _ = fmt.Fprintln(events, "signal-cancelled")
	}()
	source := &skillsInterruptControlledFS{
		FS: fstest.MapFS{
			"a-slow/SKILL.md":     {Data: []byte("---\nname: a-slow\ndescription: new slow fixture\n---\nnew slow\n")},
			"a-slow/resource.bin": {Data: []byte("new slow resource\n")},
			"z-later/SKILL.md":    {Data: []byte("---\nname: z-later\ndescription: new later fixture\n---\nnew later\n")},
			"z-later/new.txt":     {Data: []byte("new subsequent resource\n")},
		},
		events: events, release: release,
	}
	state := os.Getenv("QFEI_SKILLS_INTERRUPT_STATE")
	app := cli.New(cli.Options{
		Store: config.NewStore(state), Secrets: config.NewSecretsStore(state), SkillsFS: source,
		Stdout: os.Stdout, Stderr: os.Stderr,
		LookupEnv:  func(key string) (string, bool) { return "1", key == "CONTRACT_CLI_NO_UPDATE_CHECK" },
		HTTPClient: &http.Client{Transport: skillsInterruptNoNetwork{}},
	})
	code := app.Execute(ctx, []string{"skills", "install", "--target", os.Getenv("QFEI_SKILLS_INTERRUPT_TARGET"), "--force", "--output", "json"})
	stop()
	_ = events.Close()
	_ = release.Close()
	os.Exit(code)
}

type skillsInterruptControlledFS struct {
	fs.FS
	events, release *os.File
	staged          bool
}

func (source *skillsInterruptControlledFS) ReadFile(name string) ([]byte, error) {
	if name == "a-slow/resource.bin" && !source.staged {
		source.staged = true
		if _, err := fmt.Fprintln(source.events, "staging-resource"); err != nil {
			return nil, err
		}
		var value [1]byte
		if _, err := io.ReadFull(source.release, value[:]); err != nil {
			return nil, err
		}
	}
	return fs.ReadFile(source.FS, name)
}

type skillsInterruptNoNetwork struct{}

func (skillsInterruptNoNetwork) RoundTrip(*http.Request) (*http.Response, error) {
	return nil, errors.New("network is disabled for the installation interrupt test")
}

func skillsInterruptTreeSnapshot(t *testing.T, root string) map[string][]byte {
	t.Helper()
	files := make(map[string][]byte)
	if err := fs.WalkDir(os.DirFS(root), ".", func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		body, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		files[name] = body
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	return files
}
