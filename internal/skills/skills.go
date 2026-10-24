// Package skills installs the agent skills of the ujibox repository into the
// skill folders of Claude Code, Codex and Gemini CLI.
//
// The skills live in skills/ of the repository, with skills/index.json that
// lists each skill and its files. uji downloads them from GitHub, or reads
// them from a local folder.
package skills

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

// RawBase is where uji downloads the skills from, before the git ref.
const RawBase = "https://raw.githubusercontent.com/ujizin/ujibox"

// Index is skills/index.json.
type Index struct {
	Schema int     `json:"schema"`
	Skills []Skill `json:"skills"`
}

// Skill is one folder under skills/.
type Skill struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Files       []string `json:"files"`
}

// Find returns the skill with this name.
func (ix *Index) Find(name string) (Skill, bool) {
	for _, s := range ix.Skills {
		if s.Name == name {
			return s, true
		}
	}
	return Skill{}, false
}

// Source reads a file of the skills folder.
type Source interface {
	Read(ctx context.Context, rel string) ([]byte, error)
	String() string
}

// GitHub reads the skills of a git ref of the ujibox repository.
type GitHub struct{ Ref string }

func (g GitHub) String() string { return "github.com/ujizin/ujibox@" + g.Ref }

func (g GitHub) Read(ctx context.Context, rel string) ([]byte, error) {
	url := RawBase + "/" + g.Ref + "/skills/" + rel
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("download %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("download %s: %s", url, resp.Status)
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

// Dir reads the skills from a local folder, for example a clone of the repo.
type Dir struct{ Path string }

func (d Dir) String() string { return d.Path }

func (d Dir) Read(_ context.Context, rel string) ([]byte, error) {
	return os.ReadFile(filepath.Join(d.Path, filepath.FromSlash(rel)))
}

var nameRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]*$`)

// LoadIndex reads and checks index.json.
func LoadIndex(ctx context.Context, src Source) (*Index, error) {
	data, err := src.Read(ctx, "index.json")
	if err != nil {
		return nil, err
	}
	ix := &Index{}
	if err := json.Unmarshal(data, ix); err != nil {
		return nil, fmt.Errorf("skills/index.json: %w", err)
	}
	if ix.Schema > 1 {
		return nil, errors.New("skills/index.json is newer than this uji; update uji")
	}
	for _, s := range ix.Skills {
		if err := s.check(); err != nil {
			return nil, err
		}
	}
	return ix, nil
}

// check refuses names and file paths that could write outside the skill folder.
func (s Skill) check() error {
	if !nameRe.MatchString(s.Name) {
		return fmt.Errorf("skill name %q is not valid", s.Name)
	}
	hasMain := false
	for _, f := range s.Files {
		clean := path.Clean(f)
		if f == "" || path.IsAbs(f) || clean != f || clean == ".." || strings.HasPrefix(clean, "../") {
			return fmt.Errorf("skill %s: file path %q is not valid", s.Name, f)
		}
		if f == "SKILL.md" {
			hasMain = true
		}
	}
	if !hasMain {
		return fmt.Errorf("skill %s has no SKILL.md", s.Name)
	}
	return nil
}

// Agent is a coding agent and the folders where it reads skills, relative to
// the repository root and to the home folder.
type Agent struct {
	Name string
	Dir  string
	// Tool is the command of the agent, to detect it on this machine.
	Tool string
	// Home is the folder of its settings in the home folder.
	Home string
}

// Agents are the agents that uji installs skills for. Codex reads
// .agents/skills; Gemini CLI reads .gemini/skills and also .agents/skills.
var Agents = []Agent{
	{Name: "claude", Dir: ".claude/skills", Tool: "claude", Home: ".claude"},
	{Name: "codex", Dir: ".agents/skills", Tool: "codex", Home: ".codex"},
	{Name: "gemini", Dir: ".gemini/skills", Tool: "gemini", Home: ".gemini"},
}

// AgentByName returns the agent with this name.
func AgentByName(name string) (Agent, bool) {
	for _, a := range Agents {
		if a.Name == name {
			return a, true
		}
	}
	return Agent{}, false
}

// Install writes a skill into dir/<name>. The skill folder appears in one
// rename, so an agent never reads a half-written skill. An existing folder is
// an error unless force is set.
func Install(ctx context.Context, src Source, s Skill, dir string, force bool) (string, error) {
	if err := s.check(); err != nil {
		return "", err
	}
	dest := filepath.Join(dir, s.Name)
	if _, err := os.Stat(dest); err == nil && !force {
		return dest, fmt.Errorf("%s exists; pass --force to replace it", dest)
	}
	files := map[string][]byte{}
	for _, f := range s.Files {
		data, err := src.Read(ctx, s.Name+"/"+f)
		if err != nil {
			return dest, err
		}
		files[f] = data
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return dest, err
	}
	tmp, err := os.MkdirTemp(dir, "."+s.Name+"-*")
	if err != nil {
		return dest, err
	}
	defer os.RemoveAll(tmp)
	for f, data := range files {
		p := filepath.Join(tmp, filepath.FromSlash(f))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return dest, err
		}
		if err := os.WriteFile(p, data, 0o644); err != nil {
			return dest, err
		}
	}
	if err := os.Chmod(tmp, 0o755); err != nil {
		return dest, err
	}
	if force {
		if err := os.RemoveAll(dest); err != nil {
			return dest, err
		}
	}
	return dest, os.Rename(tmp, dest)
}
