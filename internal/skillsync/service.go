package skillsync

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gofrs/flock"
)

type Service struct {
	StateDir            string
	Source              fs.FS
	Names               []string
	Version             string
	Logger              *slog.Logger
	Rename              func(string, string) error // Filesystem boundary, injectable for fault tests.
	historyFingerprints map[string][]string        // Private test fixture; production uses embedded release fingerprints.
}
type InstallResult struct {
	Installed, Skipped []string
	Confirmed          bool
	Target             string
}
type Result struct{ Target, Status, Error, Recovery string }

func (s *Service) log() *slog.Logger {
	if s.Logger != nil {
		return s.Logger
	}
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func (s *Service) lock() (func(), error) {
	if e := os.MkdirAll(s.StateDir, 0700); e != nil {
		return nil, e
	}
	l := flock.New(filepath.Join(s.StateDir, "skill-sync.lock"))
	ok, e := l.TryLock()
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, errors.New("skill synchronization already running")
	}
	return func() { _ = l.Unlock() }, nil
}

func targetLock(target string) (func(), error) {
	l := flock.New(filepath.Join(filepath.Dir(target), ".contract-cli-lock-"+targetKey(target)))
	ok, e := l.TryLock()
	if e != nil {
		return nil, e
	}
	if !ok {
		return nil, errors.New("skill location already being updated")
	}
	return func() { _ = l.Unlock() }, nil
}

// Install preserves the existing skip/force and single-Skill interface.
// Only a complete, unskipped installation confirms a synchronization location.
func (s *Service) Install(target string, names []string, force, confirm bool) (result InstallResult, err error) {
	return s.InstallContext(context.Background(), target, names, force, confirm)
}

// InstallContext stops new installation work on cancellation. Recovery of a
// previous transaction and rollback of live changes always finish before unlock.
func (s *Service) InstallContext(ctx context.Context, target string, names []string, force, confirm bool) (result InstallResult, err error) {
	s.log().Info("Skill installation started", "complete", confirm, "force", force)
	defer func() {
		if err != nil {
			if ctx.Err() != nil && !errors.Is(err, ctx.Err()) {
				err = errors.Join(err, ctx.Err())
			}
			s.log().Error("Skill installation failed", "error", err.Error())
		}
	}()
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if _, err = Plan(names, nil); err != nil {
		return result, err
	}
	target, err = s.target(target)
	if err != nil {
		return result, err
	}
	result.Target = target
	if err = ctx.Err(); err != nil {
		return result, err
	}
	unlock, err := s.lock()
	if err != nil {
		return result, err
	}
	defer unlock()
	r, err := s.load()
	if err != nil {
		return result, err
	}
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if err = os.MkdirAll(target, 0755); err != nil {
		return result, err
	}
	release, err := targetLock(target)
	if err != nil {
		return result, err
	}
	defer release()
	if err = ctx.Err(); err != nil {
		return result, err
	}
	if err = s.recover(target, r); err != nil {
		return result, err
	}
	for _, name := range names {
		if err = ctx.Err(); err != nil {
			return result, err
		}
		if !force && exists(filepath.Join(target, name)) {
			result.Skipped = append(result.Skipped, name)
		} else {
			result.Installed = append(result.Installed, name)
		}
	}
	confirm = confirm && len(result.Skipped) == 0
	var l *Location
	var remove []string
	if confirm {
		id, e := randomID()
		if e != nil {
			return result, e
		}
		v := Location{Target: target, ID: id, Version: s.Version, Managed: append([]string{}, names...), Status: "success"}
		for _, old := range r.Locations {
			if old.Target == target {
				v.ID = old.ID
				p, e := Plan(names, old.Managed)
				if e != nil {
					return result, e
				}
				remove = p.Remove
			}
		}
		l = &v
	}
	commit := func(id string) error {
		if l == nil {
			return nil
		}
		l.Transaction = id
		return s.save(upsert(r, *l))
	}
	if len(result.Installed) > 0 || confirm {
		err = s.replaceContext(ctx, target, result.Installed, remove, l, commit)
	} else {
		err = ctx.Err()
	}
	if err == nil {
		result.Confirmed = confirm
		s.log().Info("Skill installation completed", "installed", len(result.Installed), "skipped", len(result.Skipped), "confirmed", confirm)
	}
	return result, err
}

// Sync updates each confirmed location independently. Historical candidates must
// be supplied by a verified client allowlist, never discovered by disk scanning.
func (s *Service) Sync(history []string) (results []Result, err error) {
	s.log().Info("Skill synchronization started", "version", s.Version)
	defer func() {
		if err != nil {
			s.log().Error("Skill synchronization failed", "error", err.Error())
		}
	}()
	if _, err = Plan(s.Names, nil); err != nil {
		return nil, err
	}
	unlock, err := s.lock()
	if err != nil {
		return nil, err
	}
	defer unlock()
	r, err := s.load()
	if err != nil {
		return nil, err
	}
	seen := map[string]bool{}
	for _, l := range r.Locations {
		seen[l.Target] = true
	}
	for _, candidate := range history {
		target, e := s.target(candidate)
		if e != nil || seen[target] {
			continue
		}
		if info, e := os.Stat(target); e != nil || !info.IsDir() {
			continue
		}
		found, verifyErr := s.verifyHistory(target)
		if verifyErr != nil {
			s.log().Warn("Historical Skill location not adopted", "location", targetKey(target), "reason", verifyErr.Error())
			results = append(results, Result{Target: target, Status: "unverified", Error: verifyErr.Error()})
			seen[target] = true
			continue
		}
		if !found {
			continue
		}
		id, e := randomID()
		if e != nil {
			return results, e
		}
		r.Locations = append(r.Locations, Location{Target: target, ID: id, Managed: append([]string{}, s.Names...), Status: "historical", Historical: true})
		seen[target] = true
	}
	// Persist migration candidates before touching live files, so a process kill
	// after the first backup move still leaves enough information for recovery.
	if len(r.Locations) > 0 {
		if err = s.save(r); err != nil {
			return results, err
		}
	}
	for i := range r.Locations {
		l := r.Locations[i]
		status := "success"
		failure := ""
		target, e := s.target(l.Target)
		if e == nil && target != l.Target {
			e = errors.New("skill target identity changed")
		}
		if e == nil {
			info, statErr := os.Stat(target)
			if statErr != nil || !info.IsDir() {
				status = "invalid"
				failure = "Skill directory no longer exists; reinstall the complete Skill bundle"
			}
		}
		if e == nil && status == "success" {
			release, lockErr := targetLock(target)
			e = lockErr
			if e == nil {
				func() {
					defer release()
					e = s.recover(target, r)
					if e == nil && l.Historical && !markerMatches(target, l.ID) {
						found, verifyErr := s.verifyHistory(target)
						if verifyErr != nil || !found {
							status = "unverified"
							failure = "Historical Skill ownership could not be verified; confirm the directory and explicitly reinstall the complete bundle"
							if verifyErr != nil {
								failure = verifyErr.Error()
							}
							s.log().Warn("Historical Skill retry not adopted", "location", targetKey(target), "reason", failure)
						}
					}
					if e == nil && !l.Historical && !markerMatches(target, l.ID) {
						status = "invalid"
						failure = "Skill directory management marker missing; reinstall the complete Skill bundle"
					}
					if e == nil && status == "success" {
						p, planErr := Plan(s.Names, l.Managed)
						e = planErr
						if e == nil {
							// Registration authorizes replacement only of previously managed names.
							// Verify occupied newly introduced names before any live files are moved.
							_, e = s.verifyHistoricalNames(target, unmanagedNames(s.Names, l.Managed))
							if e != nil {
								e = fmt.Errorf("新增官方 Skill 与未管理内容冲突；该位置未更新。请先备份并核实同名内容，确认允许覆盖后再完整安装: %w", e)
							}
						}
						if e == nil {
							next := l
							next.Version = s.Version
							next.Managed = append([]string{}, s.Names...)
							next.Status = "success"
							next.Error = ""
							next.Historical = false
							e = s.replace(target, p.Install, p.Remove, &next, func(id string) error {
								next.Transaction = id
								updated := upsert(r, next)
								if saveErr := s.save(updated); saveErr != nil {
									return saveErr
								}
								r = updated
								return nil
							})
							if e == nil {
								l = next
							}
						}
					}
				}()
			}
		}
		if e != nil {
			status = "failed"
			failure = e.Error()
			s.log().Error("Skill location synchronization failed", "location", targetKey(l.Target), "error", failure)
		}
		l.Status = status
		l.Error = failure
		r.Locations[i] = l
		if saveErr := s.save(r); saveErr != nil {
			return append(results, Result{Target: l.Target, Status: "failed", Error: fmt.Sprintf("record synchronization result: %v", saveErr), Recovery: RecoveryStatus(e)}), saveErr
		}
		results = append(results, Result{Target: l.Target, Status: status, Error: failure, Recovery: RecoveryStatus(e)})
	}
	s.log().Info("Skill synchronization completed", "locations", len(results))
	return results, nil
}

func (s *Service) Pending() []Location {
	r, e := s.load()
	if e != nil {
		return []Location{{Status: "failed", Error: e.Error()}}
	}
	var pending []Location
	for _, l := range r.Locations {
		unfinished := false
		if target, e := s.target(l.Target); e == nil && target == l.Target {
			var j journal
			if b, e := os.ReadFile(filepath.Join(transactionDir(target), "journal.json")); e == nil && json.Unmarshal(b, &j) == nil && j.ID != l.Transaction {
				unfinished = true
			}
		}
		if unfinished {
			l.Error = "interrupted Skill synchronization; retry a global installation or repair this location"
		}
		if l.Status == "failed" || unfinished {
			pending = append(pending, l)
		}
	}
	return pending
}
