package skills

import "fmt"

var SkillsIndex []string

var DenySkills map[string]Skills

func InitSkillsIndex() {
	SkillsIndex = nil
	skillslist := GetSkillsList()
	for _, skill := range skillslist {
		for _, skill := range skill {
			if skill.Skills.Name == "" {
				continue
			}
			if skill.Skills.Name == DenySkills[skill.Skills.Name].Name {
				continue
			}
			index := fmt.Sprintf("- name: %s, description: %s, path: %s\n", skill.Skills.Name, skill.Skills.Description, skill.Path)
			SkillsIndex = append(SkillsIndex, index)
		}
	}
}
