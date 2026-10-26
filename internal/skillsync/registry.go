package skillsync

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const markerName = ".contract-cli-managed.json"
const sourceName = "qfeius/contract-cli"

type Location struct {
	Target      string   `json:"target"`
	ID          string   `json:"id"`
	Version     string   `json:"version"`
	Managed     []string `json:"managed_skills"`
	Status      string   `json:"status"`
	Error       string   `json:"error,omitempty"`
	Transaction string   `json:"transaction,omitempty"`
	Historical  bool     `json:"historical,omitempty"`
}
type Registry struct {
	Schema    int        `json:"schema_version"`
	Locations []Location `json:"locations"`
}
type marker struct {
	ID     string `json:"id"`
	Source string `json:"source"`
}

// RegisteredLocations reads a snapshot only; it does not lock, migrate, repair or
// create state. The registry is atomically replaced by the existing write path.
func (s *Service) RegisteredLocations() ([]Location, error) {
	s.log().Info("Skill registration snapshot started")
	r, err := s.load()
	if err != nil {
		s.log().Error("Skill registration snapshot failed", "error", err.Error())
		return nil, err
	}
	s.log().Info("Skill registration snapshot completed", "locations", len(r.Locations))
	return r.Locations, nil
}

func (s *Service) load() (Registry, error) {
	r := Registry{Schema: 1}
	b, e := os.ReadFile(filepath.Join(s.StateDir, "skill-locations.json"))
	if errors.Is(e, os.ErrNotExist) {
		return r, nil
	}
	if e != nil {
		return r, e
	}
	if e = json.Unmarshal(b, &r); e != nil {
		return r, fmt.Errorf("invalid skill registry: %w", e)
	}
	if r.Schema != 1 {
		return r, fmt.Errorf("unsupported skill registry schema %d", r.Schema)
	}
	seen := map[string]bool{}
	for _, l := range r.Locations {
		if l.ID == "" || !filepath.IsAbs(l.Target) || seen[l.Target] {
			return r, errors.New("invalid skill location record")
		}
		if _, e := Plan(nil, l.Managed); e != nil {
			return r, e
		}
		seen[l.Target] = true
	}
	return r, nil
}

func atomicJSON(name string, value any) error {
	b, e := json.MarshalIndent(value, "", "  ")
	if e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(name), ".skill-state-*")
	if e != nil {
		return e
	}
	temp := f.Name()
	defer func() { _ = os.Remove(temp) }()
	if _, e = f.Write(b); e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e == nil {
		e = closeErr
	}
	if e != nil {
		return e
	}
	return os.Rename(temp, name)
}

func (s *Service) save(r Registry) error {
	return atomicJSON(filepath.Join(s.StateDir, "skill-locations.json"), r)
}

func upsert(r Registry, l Location) Registry {
	result := Registry{Schema: 1, Locations: append([]Location{}, r.Locations...)}
	for i, v := range result.Locations {
		if v.Target == l.Target {
			result.Locations[i] = l
			return result
		}
	}
	result.Locations = append(result.Locations, l)
	return result
}

func markerMatches(target, id string) bool {
	b, e := os.ReadFile(filepath.Join(target, markerName))
	if e != nil {
		return false
	}
	var m marker
	return json.Unmarshal(b, &m) == nil && m.ID == id && m.Source == sourceName
}
