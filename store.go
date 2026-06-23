package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type Store struct {
	dir       string
	mu        sync.Mutex
	nextToken int64
	versions  map[string]int64
}

func (s *Store) DeleteEvent(uid string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.Remove(filepath.Join(s.dir, uid+FILEEXT)); err != nil {
		return err
	}
	s.saveVersions()
	delete(s.versions, uid)
	return nil
}

func (s *Store) Save(e Event) error {
	if e.UID == "" {
		e.UID = uuid.New().String()
	}
	return s.SaveRaw(e.UID, []byte(fmt.Sprintf(
		"BEGIN:VCALENDAR\nBEGIN:VEVENT\nUID:%s\nDTSTART:%s\nDTEND:%s\nSUMMARY:%s\nLOCATION:%s\nDESCRIPTION:%s\nEND:VEVENT\nEND:VCALENDAR\n",
		e.UID, e.DTSTART, e.DTEND, e.SUMMARY, e.LOCATION, e.DESCRIPTION)))
}

func (s *Store) SaveRaw(uid string, raw []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := os.WriteFile(filepath.Join(s.dir, uid+FILEEXT), raw, 0644); err != nil {
		return err
	}
	s.saveVersions()
	s.versions[uid] = s.nextToken
	return nil
}

func (s *Store) ListEvents() ([]Event, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, fmt.Errorf("readDir: %w", err)
	}
	var events []Event
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != FILEEXT {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		e, err := parseEvent(string(raw))
		if err != nil {
			continue
		}
		events = append(events, e)
	}
	return events, nil
}

func (s *Store) SyncEvents(sinceToken int64) ([]Event, int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, 0, fmt.Errorf("readDir: %w", err)
	}
	var events []Event
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != FILEEXT {
			continue
		}
		uid := strings.TrimSuffix(entry.Name(), FILEEXT)
		v, ok := s.versions[uid]
		if !ok || v <= sinceToken {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(s.dir, entry.Name()))
		if err != nil {
			continue
		}
		e, err := parseEvent(string(raw))
		if err != nil {
			continue
		}
		events = append(events, e)
	}
	return events, s.nextToken, nil
}

func NewStore(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	s := &Store{dir: dir, versions: make(map[string]int64)}
	s.loadVersions()
	return s, nil
}

func (s *Store) syncPath() string { return filepath.Join(s.dir, "sync.json") }

func (s *Store) loadVersions() {
	raw, err := os.ReadFile(s.syncPath())
	if err != nil {
		return
	}
	var data struct {
		NextToken int64            `json:"next_token"`
		Versions  map[string]int64 `json:"versions"`
	}
	if json.Unmarshal(raw, &data) == nil {
		s.nextToken = data.NextToken
		s.versions = data.Versions
		if s.versions == nil {
			s.versions = make(map[string]int64)
		}
	}
}

func (s *Store) saveVersions() {
	s.nextToken++
	f, err := os.Create(s.syncPath())
	if err != nil {
		log.Println("sync save:", err)
		return
	}
	defer f.Close()
	json.NewEncoder(f).Encode(struct {
		NextToken int64            `json:"next_token"`
		Versions  map[string]int64 `json:"versions"`
	}{s.nextToken, s.versions})
}
