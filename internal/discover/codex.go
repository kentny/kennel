package discover

import (
	"os"
	"path/filepath"
	"strings"
)

// CodexSkill is one user-installed skill under ~/.codex/skills/. Codex has no
// marketplace/registry concept, so we only surface the name and the local
// path; kennel's .kennel.yaml does not auto-clone these.
type CodexSkill struct {
	Name string
	Path string // absolute
}

// CodexSkillsAt enumerates ~/.codex/skills/<name> directories, skipping any
// that begin with `.` (e.g. `.system` which bundles codex's own internals).
func CodexSkillsAt(codexHome string) ([]CodexSkill, error) {
	root := filepath.Join(codexHome, "skills")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	out := make([]CodexSkill, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		out = append(out, CodexSkill{
			Name: name,
			Path: filepath.Join(root, name),
		})
	}
	return out, nil
}

// CodexSkills scans the user's default Codex home (~/.codex).
func CodexSkills() ([]CodexSkill, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}
	return CodexSkillsAt(filepath.Join(home, ".codex"))
}
