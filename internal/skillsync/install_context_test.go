package skillsync

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"testing/fstest"
)

type cancelInstallSource struct {
	fs.FS
	read func(string)
}

func (source cancelInstallSource) ReadFile(name string) ([]byte, error) {
	content, err := fs.ReadFile(source.FS, name)
	source.read(name)
	return content, err
}

func TestInstallContextPreCancelledDoesNotCreateStateOrTarget(t *testing.T) {
	service := fixture(t)
	target := filepath.Join(t.TempDir(), "skills")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result, err := service.InstallContext(ctx, target, service.Names, true, true)
	if !errors.Is(err, context.Canceled) || result.Confirmed {
		t.Fatalf("cancelled install returned %#v, %v", result, err)
	}
	if _, err := os.Stat(target); !os.IsNotExist(err) {
		t.Fatalf("cancelled install created target: %v", err)
	}
	entries, err := os.ReadDir(service.StateDir)
	if err != nil || len(entries) != 0 {
		t.Fatalf("cancelled install touched state: %v %v", entries, err)
	}
}

func TestInstallContextCancellationDuringPreparationStopsNextSkill(t *testing.T) {
	service := fixture(t)
	service.Source = fstest.MapFS{
		"auth/SKILL.md": {Data: []byte("---\nname: auth\n---\nauth")},
		"next/SKILL.md": {Data: []byte("---\nname: next\n---\nnext")},
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	var reads []string
	service.Source = cancelInstallSource{FS: service.Source, read: func(name string) {
		reads = append(reads, name)
		cancel()
	}}
	target := filepath.Join(t.TempDir(), "skills")
	result, err := service.InstallContext(ctx, target, []string{"auth", "next"}, true, true)
	if !errors.Is(err, context.Canceled) || result.Confirmed || len(reads) != 1 {
		t.Fatalf("preparation continued after cancellation: reads=%v result=%#v err=%v", reads, result, err)
	}
	for _, name := range []string{filepath.Join(target, "auth"), filepath.Join(target, "next"), transactionDir(target), filepath.Join(service.StateDir, "skill-locations.json")} {
		if _, err := os.Stat(name); !os.IsNotExist(err) {
			t.Fatalf("cancelled preparation left %s: %v", name, err)
		}
	}
}

func TestInstallContextCancellationRestoresFilesAndRegistration(t *testing.T) {
	for _, complete := range []bool{true, false} {
		for _, phase := range []string{"backup", "installed", "verification", "last-verification"} {
			t.Run(phase+map[bool]string{true: "-complete", false: "-selected"}[complete], func(t *testing.T) {
				service, target, original := cancelledInstallFixture(t)
				registry := read(t, filepath.Join(service.StateDir, "skill-locations.json"))
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				var liveMoves []string
				service.Rename = func(from, to string) error {
					liveMoves = append(liveMoves, to)
					err := os.Rename(from, to)
					if phase == "backup" && from == filepath.Join(target, "auth") || phase == "installed" && to == filepath.Join(target, "auth") {
						cancel()
					}
					return err
				}
				reads := map[string]int{}
				service.Source = cancelInstallSource{FS: service.Source, read: func(name string) {
					reads[name]++
					if phase == "verification" && name == "auth/SKILL.md" && reads[name] == 3 || phase == "last-verification" && name == "next/SKILL.md" && reads[name] == 3 {
						cancel()
					}
				}}
				result, err := service.InstallContext(ctx, target, service.Names, true, complete)
				if !errors.Is(err, context.Canceled) || RecoveryStatus(err) != "restored" || result.Confirmed {
					t.Fatalf("cancellation lost recovery: result=%#v err=%v", result, err)
				}
				if phase == "backup" && len(liveMoves) != 1 || phase == "installed" && len(liveMoves) != 2 {
					t.Fatalf("installation continued live moves after cancel: %v", liveMoves)
				}
				if got := installTreeSnapshot(t, target); !reflect.DeepEqual(got, original) {
					t.Fatalf("original files not restored: got=%v want=%v", got, original)
				}
				if got := read(t, filepath.Join(service.StateDir, "skill-locations.json")); got != registry {
					t.Fatal("cancellation changed registration")
				}
				if exists(transactionDir(target)) {
					t.Fatal("successful rollback left transaction material")
				}
				service.Source = service.Source.(cancelInstallSource).FS
				service.Rename = nil
				if _, err := service.Install(target, service.Names, true, complete); err != nil {
					t.Fatalf("cancellation did not release existing locks: %v", err)
				}
			})
		}
	}
}

func cancelledInstallFixture(t *testing.T) (*Service, string, map[string]string) {
	t.Helper()
	service := fixture(t)
	service.Names = []string{"auth", "retired"}
	service.Source = fstest.MapFS{
		"auth/SKILL.md":    {Data: []byte("---\nname: auth\n---\noriginal")},
		"retired/SKILL.md": {Data: []byte("---\nname: retired\n---\nretired")},
	}
	target, err := canonical(filepath.Join(t.TempDir(), "skills"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Install(target, service.Names, true, true); err != nil {
		t.Fatal(err)
	}
	put(t, filepath.Join(target, "auth", "local.txt"), "preserve local changes")
	put(t, filepath.Join(target, "third-party", "SKILL.md"), "preserve third party")
	original := installTreeSnapshot(t, target)
	service.Names = []string{"auth", "next"}
	service.Version = "3.0.0"
	service.Source = fstest.MapFS{
		"auth/SKILL.md": {Data: []byte("---\nname: auth\n---\nreplacement")},
		"next/SKILL.md": {Data: []byte("---\nname: next\n---\nnew skill")},
	}
	return service, target, original
}

func installTreeSnapshot(t *testing.T, target string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(target, func(name string, entry fs.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return err
		}
		files[strings.TrimPrefix(name, target)] = read(t, name)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func TestInstallContextCancellationRemovesNewEntriesAndPreservesSkipped(t *testing.T) {
	for _, skip := range []bool{false, true} {
		t.Run(map[bool]string{false: "first-install", true: "skipped-existing"}[skip], func(t *testing.T) {
			service := fixture(t)
			service.Source = fstest.MapFS{
				"auth/SKILL.md": {Data: []byte("---\nname: auth\n---\nnew")},
				"next/SKILL.md": {Data: []byte("---\nname: next\n---\nnext")},
			}
			target, err := canonical(filepath.Join(t.TempDir(), "skills"))
			if err != nil {
				t.Fatal(err)
			}
			put(t, filepath.Join(target, "third-party", "SKILL.md"), "keep")
			if skip {
				put(t, filepath.Join(target, "auth", "SKILL.md"), "keep original")
			}
			original := installTreeSnapshot(t, target)
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			moves := 0
			service.Rename = func(from, to string) error {
				moves++
				err := os.Rename(from, to)
				cancel()
				return err
			}
			result, err := service.InstallContext(ctx, target, []string{"auth", "next"}, false, true)
			if !errors.Is(err, context.Canceled) || RecoveryStatus(err) != "restored" || result.Confirmed || moves != 1 {
				t.Fatalf("new entry cancellation failed: result=%#v moves=%d err=%v", result, moves, err)
			}
			if !reflect.DeepEqual(installTreeSnapshot(t, target), original) || exists(filepath.Join(service.StateDir, "skill-locations.json")) {
				t.Fatal("cancelled first/partial install changed original files or registration")
			}
		})
	}
}

func TestInstallContextCancellationRollbackFailureRetainsRecoveryMaterials(t *testing.T) {
	service, target, _ := cancelledInstallFixture(t)
	registry := read(t, filepath.Join(service.StateDir, "skill-locations.json"))
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Rename = func(from, to string) error {
		if from == filepath.Join(transactionDir(target), "stage", "auth") {
			if err := os.Rename(target, target+"-moved"); err != nil {
				return err
			}
			if err := os.WriteFile(target, []byte("block rollback in isolated test"), 0600); err != nil {
				return err
			}
			cancel()
			return ctx.Err()
		}
		return os.Rename(from, to)
	}
	_, err := service.InstallContext(ctx, target, service.Names, true, true)
	var failure *TransactionFailure
	if !errors.Is(err, context.Canceled) || !errors.As(err, &failure) || failure.RollbackErr == nil || RecoveryStatus(err) != "failed" {
		t.Fatalf("cancellation hid rollback failure: %v", err)
	}
	if !exists(filepath.Join(transactionDir(target), "backup", "auth", "SKILL.md")) || read(t, filepath.Join(service.StateDir, "skill-locations.json")) != registry {
		t.Fatal("failed rollback lost recovery materials or changed registration")
	}
	if err := os.Remove(target); err != nil {
		t.Fatal(err)
	}
	if err := os.Rename(target+"-moved", target); err != nil {
		t.Fatal(err)
	}
	service.Rename = nil
	if _, err := service.Install(target, service.Names, true, true); err != nil {
		t.Fatalf("retry could not recover after cancellation: %v", err)
	}
}

func TestReplaceContextCancellationAfterCommitKeepsCommittedFilesAndRegistry(t *testing.T) {
	service, target, _ := cancelledInstallFixture(t)
	registry, err := service.load()
	if err != nil {
		t.Fatal(err)
	}
	location := registry.Locations[0]
	location.Version = service.Version
	location.Managed = append([]string{}, service.Names...)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	err = service.replaceContext(ctx, target, service.Names, []string{"retired"}, &location, func(id string) error {
		location.Transaction = id
		if err := service.save(upsert(registry, location)); err != nil {
			return err
		}
		cancel()
		return nil
	})
	if err != nil || ctx.Err() == nil || exists(transactionDir(target)) {
		t.Fatalf("committed operation treated as rollback: %v", err)
	}
	if !strings.Contains(read(t, filepath.Join(target, "auth", "SKILL.md")), "version: 3.0.0") || exists(filepath.Join(target, "retired")) {
		t.Fatal("late cancellation undid committed files")
	}
	registered, err := service.load()
	if err != nil || registered.Locations[0].Version != service.Version || registered.Locations[0].Transaction != location.Transaction {
		t.Fatalf("late cancellation broke committed registration: %v %v", registered, err)
	}
}

func TestInstallContextCancellationCleansPartiallyPreparedResources(t *testing.T) {
	service, target, original := cancelledInstallFixture(t)
	registry := read(t, filepath.Join(service.StateDir, "skill-locations.json"))
	source := service.Source.(fstest.MapFS)
	source["auth/references/guide.md"] = &fstest.MapFile{Data: []byte("resource")}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	staged := false
	service.Source = cancelInstallSource{FS: source, read: func(name string) {
		if name == "auth/references/guide.md" {
			staged = exists(filepath.Join(transactionDir(target), "stage", "auth", "SKILL.md"))
			cancel()
		}
	}}
	_, err := service.InstallContext(ctx, target, service.Names, true, true)
	if !staged || !errors.Is(err, context.Canceled) || exists(transactionDir(target)) {
		t.Fatalf("partial preparation was not cleaned: staged=%t err=%v", staged, err)
	}
	if !reflect.DeepEqual(installTreeSnapshot(t, target), original) || read(t, filepath.Join(service.StateDir, "skill-locations.json")) != registry {
		t.Fatal("cancelled preparation changed original files or registration")
	}
}

func TestInstallContextCancellationPreservesIOFailure(t *testing.T) {
	service := fixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.Source = cancelInstallSource{FS: fstest.MapFS{}, read: func(string) { cancel() }}
	_, err := service.InstallContext(ctx, filepath.Join(t.TempDir(), "skills"), service.Names, true, true)
	if !errors.Is(err, context.Canceled) || !errors.Is(err, fs.ErrNotExist) {
		t.Fatalf("concurrent cancellation lost I/O failure: %v", err)
	}
}
