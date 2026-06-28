package ai

import (
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"
	"sync"
)

type PromptTemplate struct {
	Name    string `json:"name"`
	Version int    `json:"version"`
	Body    string `json:"body"`
	SHA256  string `json:"sha256"`
}

func NewPromptTemplate(name, body string, version int) PromptTemplate {
	return PromptTemplate{
		Name:    name,
		Version: version,
		Body:    body,
		SHA256:  hashPrompt(body),
	}
}

func hashPrompt(body string) string {
	h := sha256.Sum256([]byte(body))
	return fmt.Sprintf("%x", h[:8])
}

type PromptManager struct {
	mu       sync.RWMutex
	templates map[string][]PromptTemplate // name → versions (sorted, latest last)
}

func NewPromptManager() *PromptManager {
	return &PromptManager{
		templates: make(map[string][]PromptTemplate),
	}
}

func (m *PromptManager) Register(name, body string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	versions := m.templates[name]
	nextVersion := 1
	if len(versions) > 0 {
		latest := versions[len(versions)-1]
		if latest.Body == body {
			return nil
		}
		nextVersion = latest.Version + 1
	}

	m.templates[name] = append(versions, NewPromptTemplate(name, body, nextVersion))
	return nil
}

func (m *PromptManager) Get(name string, version int) (PromptTemplate, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	versions, ok := m.templates[name]
	if !ok {
		return PromptTemplate{}, false
	}

	if version <= 0 {
		return versions[len(versions)-1], true
	}

	for _, t := range versions {
		if t.Version == version {
			return t, true
		}
	}
	return PromptTemplate{}, false
}

func (m *PromptManager) GetLatest(name string) (PromptTemplate, bool) {
	return m.Get(name, 0)
}

func (m *PromptManager) GetByHash(hash string) (PromptTemplate, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, versions := range m.templates {
		for _, t := range versions {
			if t.SHA256 == hash {
				return t, true
			}
		}
	}
	return PromptTemplate{}, false
}

func (m *PromptManager) List() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	names := make([]string, 0, len(m.templates))
	for n := range m.templates {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func (m *PromptManager) Resolve(ref string) (body string, hash string, ok bool) {
	// Format: "name" (latest) or "name@2" (specific version) or "name#sha256" (by hash)
	name := ref
	version := 0

	if atIdx := strings.Index(ref, "@"); atIdx > 0 {
		name = ref[:atIdx]
		fmt.Sscanf(ref[atIdx+1:], "%d", &version)
	} else if hashIdx := strings.Index(ref, "#"); hashIdx > 0 {
		name = ref[:hashIdx]
		t, found := m.GetByHash(ref[hashIdx+1:])
		if found {
			return t.Body, t.SHA256, true
		}
		return "", "", false
	}

	t, found := m.Get(name, version)
	if !found {
		return "", "", false
	}
	return t.Body, t.SHA256, true
}
