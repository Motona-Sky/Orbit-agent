package skills

import (
	"orbit/internal/utils"
	"path/filepath"
)

var SkillsPath []string

func init() {
	SkillsPath = append(SkillsPath, filepath.Join(utils.ProejctConfigPath, "skills"))
	SkillsPath = append(SkillsPath, filepath.Join(utils.ConfigFolderPath, "skills"))
	SkillsPath = append(SkillsPath, filepath.Join(utils.UserPath, ".agents", "skills"))
	SkillsPath = append(SkillsPath, filepath.Join(utils.UserPath, ".agent", "skills"))
	SkillsPath = append(SkillsPath, filepath.Join(utils.UserPath, ".codex", "skills"))
	SkillsPath = append(SkillsPath, filepath.Join(utils.UserPath, ".claude", "skills"))
	InitSkillsIndex()

}
