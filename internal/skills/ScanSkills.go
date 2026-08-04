package skills

import (
	"bufio"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type Skills struct {
	Name        string `json:"name"`
	RunAs       string `json:"runas,omitempty"`
	Description string `json:"description"`
}
type SkillsList struct {
	Skills Skills
	Path   string
}

var SkillsLists []SkillsList

func GetSkillsList() map[string][]SkillsList {
	SkillsLists = nil
	seenNames := make(map[string]struct{})

	for _, path := range SkillsPath {
		skills, err := ScanSkills(path)
		if err != nil {
			continue
		}

		for _, groupedSkills := range ParseSkills(skills) {
			for _, skill := range groupedSkills {
				if _, exists := seenNames[skill.Skills.Name]; exists {
					continue
				}
				seenNames[skill.Skills.Name] = struct{}{}
				SkillsLists = append(SkillsLists, skill)
			}
		}
	}

	sort.Slice(SkillsLists, func(i, j int) bool {
		return SkillsLists[i].Skills.Name < SkillsLists[j].Skills.Name
	})

	groupedSkills := make(map[string][]SkillsList)
	for _, skill := range SkillsLists {
		groupedSkills[skill.Skills.RunAs] = append(groupedSkills[skill.Skills.RunAs], skill)
	}
	return groupedSkills
}

// skills.md文件路径
func ScanSkills(paths ...string) ([]SkillsList, error) {
	skills := make([]SkillsList, 0)
	for _, root := range paths {
		err := filepath.WalkDir(root, func(filePath string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type().IsRegular() && strings.EqualFold(entry.Name(), "SKILL.md") {
				skills = append(skills, SkillsList{Path: filePath})
			}
			return nil
		})
		if err != nil {
			return skills, err
		}
	}
	return skills, nil
}

func ParseSkills(skillsList []SkillsList) map[string][]SkillsList {
	var skillslist = make(map[string][]SkillsList)
	for _, skill := range skillsList {
		f, err := os.Open(skill.Path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(f)
		if scanner.Scan() && scanner.Text() == "---" {
			for scanner.Scan() {
				Line := scanner.Text()
				if Line == "---" {
					break
				}
				if strings.HasPrefix(Line, "name:") {
					skill.Skills.Name = strings.TrimSpace(Line[5:])
				}
				if strings.HasPrefix(Line, "runas:") {
					skill.Skills.RunAs = strings.TrimSpace(Line[6:])
					//skills方式//待改
				}
				if strings.HasPrefix(Line, "description:") {
					skill.Skills.Description = strings.TrimSpace(Line[12:])
				}
			}
		}
		scanErr := scanner.Err()
		f.Close()
		if scanErr != nil {
			continue
		}
		if skill.Skills.Name != "" {
			skillslist[skill.Skills.RunAs] = append(skillslist[skill.Skills.RunAs], skill)
		}

	}
	return skillslist
}
