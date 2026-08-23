package vibe

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/vdbergkevin/vibe-dock/internal/model"
)

const (
	maxSkillFileSize = 512 * 1024
	maxImportBytes   = 10 * 1024 * 1024
	maxImportFiles   = 200
)

var (
	skillNamePattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)
	toolNamePattern  = regexp.MustCompile(`^[A-Za-z0-9_.*?-]{1,80}$`)
)

type skillConfig struct {
	SkillPaths     []string `toml:"skill_paths"`
	EnabledSkills  []string `toml:"enabled_skills"`
	DisabledSkills []string `toml:"disabled_skills"`
}

type skillLocation struct {
	root      string
	scope     string
	source    string
	projectID string
	editable  bool
	config    skillConfig
}

func DiscoverSkills(vibeHome, projectID, projectName, projectRoot string) model.SkillInventory {
	globalRoot := filepath.Join(vibeHome, "skills")
	inventory := model.SkillInventory{
		Skills:      []model.Skill{},
		ProjectID:   projectID,
		ProjectName: projectName,
		GlobalPath:  globalRoot,
		Errors:      []string{},
	}
	globalConfig, globalErr := readSkillConfig(filepath.Join(vibeHome, "config.toml"))
	if globalErr != nil {
		inventory.Errors = append(inventory.Errors, globalErr.Error())
	}
	locations := []skillLocation{{root: globalRoot, scope: "global", source: "vibe", editable: true, config: globalConfig}}

	if home, err := os.UserHomeDir(); err == nil {
		locations = append(locations, skillLocation{root: filepath.Join(home, ".agents", "skills"), scope: "global", source: "agents", editable: false, config: globalConfig})
	}
	for _, customPath := range globalConfig.SkillPaths {
		if strings.TrimSpace(customPath) == "" {
			continue
		}
		if !filepath.IsAbs(customPath) {
			customPath = filepath.Join(vibeHome, customPath)
		}
		locations = append(locations, skillLocation{root: filepath.Clean(customPath), scope: "global", source: "custom", editable: false, config: globalConfig})
	}

	if projectRoot != "" {
		projectSkillRoot := filepath.Join(projectRoot, ".vibe", "skills")
		inventory.ProjectPath = projectSkillRoot
		projectConfig, configErr := readSkillConfig(filepath.Join(projectRoot, ".vibe", "config.toml"))
		if configErr != nil {
			inventory.Errors = append(inventory.Errors, configErr.Error())
		}
		locations = append([]skillLocation{
			{root: projectSkillRoot, scope: "project", source: "vibe", projectID: projectID, editable: true, config: projectConfig},
			{root: filepath.Join(projectRoot, ".agents", "skills"), scope: "project", source: "agents", projectID: projectID, editable: false, config: projectConfig},
		}, locations...)
	}

	for _, location := range locations {
		skills, err := discoverLocation(location)
		if err != nil {
			inventory.Errors = append(inventory.Errors, err.Error())
			continue
		}
		inventory.Skills = append(inventory.Skills, skills...)
	}
	sort.SliceStable(inventory.Skills, func(i, j int) bool {
		if inventory.Skills[i].Scope != inventory.Skills[j].Scope {
			return inventory.Skills[i].Scope == "project"
		}
		return strings.ToLower(inventory.Skills[i].Name) < strings.ToLower(inventory.Skills[j].Name)
	})
	return inventory
}

func SaveSkill(vibeHome, projectRoot string, input model.Skill) (model.Skill, error) {
	if err := validateSkill(input); err != nil {
		return model.Skill{}, err
	}
	root, err := editableSkillRoot(vibeHome, projectRoot, input.Scope)
	if err != nil {
		return model.Skill{}, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return model.Skill{}, fmt.Errorf("create skills folder: %w", err)
	}
	originalName := strings.TrimSpace(input.OriginalName)
	if originalName != "" && !skillNamePattern.MatchString(originalName) {
		return model.Skill{}, errors.New("the original skill name is invalid")
	}
	destination, err := childPath(root, input.Name)
	if err != nil {
		return model.Skill{}, err
	}
	if originalName == "" {
		if _, statErr := os.Stat(destination); statErr == nil {
			return model.Skill{}, errors.New("a skill with this name already exists")
		} else if !os.IsNotExist(statErr) {
			return model.Skill{}, statErr
		}
		if err := os.Mkdir(destination, 0o700); err != nil {
			return model.Skill{}, fmt.Errorf("create skill: %w", err)
		}
	} else if originalName != input.Name {
		original, pathErr := childPath(root, originalName)
		if pathErr != nil {
			return model.Skill{}, pathErr
		}
		if _, statErr := os.Stat(destination); statErr == nil {
			return model.Skill{}, errors.New("a skill with this name already exists")
		}
		if err := os.Rename(original, destination); err != nil {
			return model.Skill{}, fmt.Errorf("rename skill: %w", err)
		}
	} else if info, statErr := os.Stat(destination); statErr != nil || !info.IsDir() {
		return model.Skill{}, errors.New("the skill folder is no longer available")
	}

	content := renderSkill(input)
	if len(content) > maxSkillFileSize {
		return model.Skill{}, errors.New("SKILL.md is too large")
	}
	if err := writeAtomic(filepath.Join(destination, "SKILL.md"), []byte(content), 0o600); err != nil {
		return model.Skill{}, fmt.Errorf("save skill: %w", err)
	}
	config, _ := readSkillConfig(skillConfigPath(vibeHome, projectRoot, input.Scope))
	return readSkill(filepath.Join(destination, "SKILL.md"), skillLocation{root: root, scope: input.Scope, source: "vibe", projectID: input.ProjectID, editable: true, config: config})
}

func DeleteSkill(vibeHome, projectRoot, scope, name string) error {
	if !skillNamePattern.MatchString(name) {
		return errors.New("invalid skill name")
	}
	root, err := editableSkillRoot(vibeHome, projectRoot, scope)
	if err != nil {
		return err
	}
	target, err := childPath(root, name)
	if err != nil {
		return err
	}
	info, err := os.Stat(target)
	if err != nil {
		if os.IsNotExist(err) {
			return errors.New("skill not found")
		}
		return err
	}
	if !info.IsDir() {
		return errors.New("skill path is not a directory")
	}
	return os.RemoveAll(target)
}

func SetSkillEnabled(vibeHome, projectRoot, scope, name string, enabled bool) error {
	if !skillNamePattern.MatchString(name) {
		return errors.New("invalid skill name")
	}
	configPath := skillConfigPath(vibeHome, projectRoot, scope)
	if configPath == "" {
		return errors.New("select a project before changing a project skill")
	}
	config, err := readSkillConfig(configPath)
	if err != nil {
		return err
	}
	if enabled {
		config.DisabledSkills = removeExact(config.DisabledSkills, name)
		if len(config.EnabledSkills) > 0 && !matchesSkillPattern(name, config.EnabledSkills) {
			config.EnabledSkills = append(config.EnabledSkills, name)
		}
	} else if !contains(config.DisabledSkills, name) {
		config.DisabledSkills = append(config.DisabledSkills, name)
	}
	sort.Strings(config.EnabledSkills)
	sort.Strings(config.DisabledSkills)
	return writeSkillFilters(configPath, config.EnabledSkills, config.DisabledSkills)
}

func ImportSkill(vibeHome, projectRoot, sourcePath, scope, projectID string) (model.Skill, error) {
	sourcePath = filepath.Clean(strings.TrimSpace(sourcePath))
	info, err := os.Stat(sourcePath)
	if err != nil || !info.IsDir() {
		return model.Skill{}, errors.New("choose a folder containing SKILL.md")
	}
	parsed, err := readSkill(filepath.Join(sourcePath, "SKILL.md"), skillLocation{scope: scope, source: "vibe", projectID: projectID, editable: true})
	if err != nil {
		return model.Skill{}, err
	}
	if err := validateSkill(parsed); err != nil {
		return model.Skill{}, err
	}
	root, err := editableSkillRoot(vibeHome, projectRoot, scope)
	if err != nil {
		return model.Skill{}, err
	}
	if err := os.MkdirAll(root, 0o700); err != nil {
		return model.Skill{}, err
	}
	destination, err := childPath(root, parsed.Name)
	if err != nil {
		return model.Skill{}, err
	}
	if _, statErr := os.Stat(destination); statErr == nil {
		return model.Skill{}, errors.New("a skill with this name already exists")
	}
	if err := copySkillDirectory(sourcePath, destination); err != nil {
		_ = os.RemoveAll(destination)
		return model.Skill{}, err
	}
	config, _ := readSkillConfig(skillConfigPath(vibeHome, projectRoot, scope))
	return readSkill(filepath.Join(destination, "SKILL.md"), skillLocation{root: root, scope: scope, source: "vibe", projectID: projectID, editable: true, config: config})
}

func discoverLocation(location skillLocation) ([]model.Skill, error) {
	entries, err := os.ReadDir(location.root)
	if err != nil {
		if os.IsNotExist(err) {
			return []model.Skill{}, nil
		}
		return nil, fmt.Errorf("read skills from %s: %w", location.root, err)
	}
	result := make([]model.Skill, 0, len(entries))
	if _, err := os.Stat(filepath.Join(location.root, "SKILL.md")); err == nil {
		if skill, readErr := readSkill(filepath.Join(location.root, "SKILL.md"), location); readErr == nil {
			result = append(result, skill)
		}
	}
	for _, entry := range entries {
		if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		skill, readErr := readSkill(filepath.Join(location.root, entry.Name(), "SKILL.md"), location)
		if readErr == nil {
			result = append(result, skill)
		}
	}
	return result, nil
}

func readSkill(path string, location skillLocation) (model.Skill, error) {
	info, err := os.Stat(path)
	if err != nil {
		return model.Skill{}, fmt.Errorf("read SKILL.md: %w", err)
	}
	if info.Size() > maxSkillFileSize {
		return model.Skill{}, errors.New("SKILL.md exceeds the 512 KB limit")
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return model.Skill{}, err
	}
	metadata, instructions, err := parseSkill(string(content))
	if err != nil {
		return model.Skill{}, fmt.Errorf("parse %s: %w", path, err)
	}
	name := metadata["name"]
	if name == "" {
		name = filepath.Base(filepath.Dir(path))
	}
	tools := splitMetadataList(metadata["allowed-tools"])
	skill := model.Skill{
		ID:            location.scope + ":" + location.source + ":" + name,
		Name:          name,
		OriginalName:  name,
		Description:   metadata["description"],
		Instructions:  instructions,
		Scope:         location.scope,
		Source:        location.source,
		ProjectID:     location.projectID,
		Path:          path,
		UserInvocable: parseBool(metadata["user-invocable"]),
		AllowedTools:  tools,
		Enabled:       skillEnabled(name, location.config),
		Editable:      location.editable,
		Risk:          skillRisk(tools),
		UpdatedAt:     info.ModTime().UTC(),
	}
	return skill, nil
}

func parseSkill(content string) (map[string]string, string, error) {
	content = strings.ReplaceAll(content, "\r\n", "\n")
	if !strings.HasPrefix(content, "---\n") {
		return nil, "", errors.New("SKILL.md must begin with YAML frontmatter")
	}
	end := strings.Index(content[4:], "\n---")
	if end < 0 {
		return nil, "", errors.New("SKILL.md frontmatter is not closed")
	}
	end += 4
	frontmatter := content[4:end]
	instructions := strings.TrimSpace(content[end+4:])
	metadata := map[string]string{}
	currentList := ""
	scanner := bufio.NewScanner(strings.NewReader(frontmatter))
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		if currentList != "" && strings.HasPrefix(trimmed, "-") {
			value := unquoteYAML(strings.TrimSpace(strings.TrimPrefix(trimmed, "-")))
			if value != "" {
				if metadata[currentList] != "" {
					metadata[currentList] += "\n"
				}
				metadata[currentList] += value
			}
			continue
		}
		key, value, found := strings.Cut(trimmed, ":")
		if !found {
			currentList = ""
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		value = strings.TrimSpace(value)
		if value == "" {
			currentList = key
			continue
		}
		currentList = ""
		metadata[key] = unquoteYAML(value)
	}
	return metadata, instructions, scanner.Err()
}

func renderSkill(skill model.Skill) string {
	var builder strings.Builder
	builder.WriteString("---\nname: ")
	builder.WriteString(strconv.Quote(skill.Name))
	builder.WriteString("\ndescription: ")
	builder.WriteString(strconv.Quote(strings.TrimSpace(skill.Description)))
	builder.WriteString("\nuser-invocable: ")
	builder.WriteString(strconv.FormatBool(skill.UserInvocable))
	if len(skill.AllowedTools) > 0 {
		builder.WriteString("\nallowed-tools:\n")
		for _, tool := range skill.AllowedTools {
			builder.WriteString("  - ")
			builder.WriteString(strconv.Quote(tool))
			builder.WriteByte('\n')
		}
	}
	builder.WriteString("---\n\n")
	builder.WriteString(strings.TrimSpace(skill.Instructions))
	builder.WriteByte('\n')
	return builder.String()
}

func validateSkill(skill model.Skill) error {
	if !skillNamePattern.MatchString(strings.TrimSpace(skill.Name)) {
		return errors.New("skill names must use lowercase letters, numbers, and hyphens")
	}
	if strings.TrimSpace(skill.Description) == "" {
		return errors.New("skill description is required")
	}
	if len(skill.Description) > 500 {
		return errors.New("skill description is too long")
	}
	if strings.TrimSpace(skill.Instructions) == "" {
		return errors.New("skill instructions are required")
	}
	if skill.Scope != "global" && skill.Scope != "project" {
		return errors.New("skill scope must be global or project")
	}
	if len(skill.AllowedTools) > 64 {
		return errors.New("a skill can allow at most 64 tools")
	}
	for _, tool := range skill.AllowedTools {
		if !toolNamePattern.MatchString(tool) {
			return fmt.Errorf("invalid allowed tool %q", tool)
		}
	}
	return nil
}

func editableSkillRoot(vibeHome, projectRoot, scope string) (string, error) {
	switch scope {
	case "global":
		return filepath.Join(vibeHome, "skills"), nil
	case "project":
		if projectRoot == "" {
			return "", errors.New("select a project before managing project skills")
		}
		return filepath.Join(projectRoot, ".vibe", "skills"), nil
	default:
		return "", errors.New("unsupported skill scope")
	}
}

func skillConfigPath(vibeHome, projectRoot, scope string) string {
	if scope == "global" {
		return filepath.Join(vibeHome, "config.toml")
	}
	if scope == "project" && projectRoot != "" {
		return filepath.Join(projectRoot, ".vibe", "config.toml")
	}
	return ""
}

func readSkillConfig(path string) (skillConfig, error) {
	var config skillConfig
	if path == "" {
		return config, nil
	}
	if _, err := toml.DecodeFile(path, &config); err != nil {
		if os.IsNotExist(err) {
			return config, nil
		}
		return config, fmt.Errorf("read skill filters from %s: %w", path, err)
	}
	return config, nil
}

func skillEnabled(name string, config skillConfig) bool {
	if matchesSkillPattern(name, config.DisabledSkills) {
		return false
	}
	return len(config.EnabledSkills) == 0 || matchesSkillPattern(name, config.EnabledSkills)
}

func matchesSkillPattern(name string, patterns []string) bool {
	for _, pattern := range patterns {
		if strings.HasPrefix(pattern, "re:") {
			if expression, err := regexp.Compile(strings.TrimPrefix(pattern, "re:")); err == nil && expression.MatchString(name) {
				return true
			}
			continue
		}
		if matched, err := filepath.Match(pattern, name); err == nil && matched {
			return true
		}
	}
	return false
}

func writeSkillFilters(path string, enabled, disabled []string) error {
	content := ""
	if existing, err := os.ReadFile(path); err == nil {
		content = string(existing)
	} else if !os.IsNotExist(err) {
		return err
	}
	content = removeTopLevelArrays(content, "enabled_skills", "disabled_skills")
	var filters []string
	if len(enabled) > 0 {
		filters = append(filters, "enabled_skills = "+formatTOMLArray(enabled))
	}
	if len(disabled) > 0 {
		filters = append(filters, "disabled_skills = "+formatTOMLArray(disabled))
	}
	if len(filters) > 0 {
		lines := strings.Split(strings.TrimRight(content, "\n"), "\n")
		insertAt := len(lines)
		for index, line := range lines {
			if strings.HasPrefix(strings.TrimSpace(line), "[") {
				insertAt = index
				break
			}
		}
		addition := append(filters, "")
		lines = append(lines[:insertAt], append(addition, lines[insertAt:]...)...)
		content = strings.TrimLeft(strings.Join(lines, "\n"), "\n")
	}
	content = strings.TrimRight(content, "\n")
	if content != "" {
		content += "\n"
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return err
	}
	return writeAtomic(path, []byte(content), 0o600)
}

func removeTopLevelArrays(content string, keys ...string) string {
	keySet := map[string]bool{}
	for _, key := range keys {
		keySet[key] = true
	}
	lines := strings.Split(strings.ReplaceAll(content, "\r\n", "\n"), "\n")
	result := make([]string, 0, len(lines))
	for index := 0; index < len(lines); index++ {
		trimmed := strings.TrimSpace(lines[index])
		key, value, found := strings.Cut(trimmed, "=")
		if !found || !keySet[strings.TrimSpace(key)] {
			result = append(result, lines[index])
			continue
		}
		depth := bracketDelta(value)
		for depth > 0 && index+1 < len(lines) {
			index++
			depth += bracketDelta(lines[index])
		}
	}
	return strings.Join(result, "\n")
}

func bracketDelta(value string) int {
	depth := 0
	inString := false
	escaped := false
	for _, char := range value {
		if escaped {
			escaped = false
			continue
		}
		if char == '\\' && inString {
			escaped = true
			continue
		}
		if char == '"' {
			inString = !inString
			continue
		}
		if inString {
			continue
		}
		if char == '[' {
			depth++
		} else if char == ']' {
			depth--
		}
	}
	return depth
}

func copySkillDirectory(source, destination string) error {
	fileCount := 0
	totalBytes := int64(0)
	err := filepath.WalkDir(source, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if info.Mode()&os.ModeSymlink != 0 {
			return fmt.Errorf("skill imports cannot contain symbolic links: %s", entry.Name())
		}
		relative, err := filepath.Rel(source, path)
		if err != nil {
			return err
		}
		target := filepath.Join(destination, relative)
		if entry.IsDir() {
			return os.MkdirAll(target, 0o700)
		}
		if !info.Mode().IsRegular() {
			return fmt.Errorf("unsupported file in skill: %s", entry.Name())
		}
		fileCount++
		totalBytes += info.Size()
		if fileCount > maxImportFiles || totalBytes > maxImportBytes {
			return errors.New("skill import exceeds the 200 file or 10 MB limit")
		}
		return copyFile(path, target)
	})
	return err
}

func copyFile(source, destination string) error {
	input, err := os.Open(source)
	if err != nil {
		return err
	}
	defer input.Close()
	output, err := os.OpenFile(destination, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if err != nil {
		return err
	}
	if _, err := io.Copy(output, input); err != nil {
		_ = output.Close()
		return err
	}
	return output.Close()
}

func writeAtomic(path string, content []byte, mode os.FileMode) error {
	temporary, err := os.CreateTemp(filepath.Dir(path), ".vibe-skill-*")
	if err != nil {
		return err
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return err
	}
	if _, err := temporary.Write(content); err != nil {
		_ = temporary.Close()
		return err
	}
	if err := temporary.Close(); err != nil {
		return err
	}
	return os.Rename(temporaryPath, path)
}

func childPath(root, name string) (string, error) {
	root = filepath.Clean(root)
	path := filepath.Join(root, name)
	relative, err := filepath.Rel(root, path)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) {
		return "", errors.New("skill path escapes its managed folder")
	}
	return path, nil
}

func splitMetadataList(value string) []string {
	value = strings.TrimSpace(value)
	if strings.HasPrefix(value, "[") && strings.HasSuffix(value, "]") {
		value = strings.TrimSpace(value[1 : len(value)-1])
	}
	parts := strings.FieldsFunc(value, func(char rune) bool { return char == '\n' || char == ',' })
	result := make([]string, 0, len(parts))
	seen := map[string]bool{}
	for _, part := range parts {
		part = unquoteYAML(strings.TrimSpace(part))
		if part != "" && !seen[part] {
			seen[part] = true
			result = append(result, part)
		}
	}
	return result
}

func skillRisk(tools []string) string {
	for _, tool := range tools {
		normalized := strings.ToLower(tool)
		if strings.Contains(normalized, "bash") || strings.Contains(normalized, "shell") || strings.Contains(normalized, "terminal") {
			return "shell"
		}
	}
	for _, tool := range tools {
		normalized := strings.ToLower(tool)
		if strings.Contains(normalized, "write") || strings.Contains(normalized, "edit") || strings.Contains(normalized, "delete") || strings.Contains(normalized, "replace") {
			return "write"
		}
	}
	return "limited"
}

func unquoteYAML(value string) string {
	value = strings.TrimSpace(value)
	if len(value) >= 2 && ((value[0] == '"' && value[len(value)-1] == '"') || (value[0] == '\'' && value[len(value)-1] == '\'')) {
		if value[0] == '"' {
			if unquoted, err := strconv.Unquote(value); err == nil {
				return unquoted
			}
		}
		return value[1 : len(value)-1]
	}
	return value
}

func parseBool(value string) bool {
	parsed, _ := strconv.ParseBool(strings.TrimSpace(value))
	return parsed
}

func formatTOMLArray(values []string) string {
	quoted := make([]string, 0, len(values))
	for _, value := range values {
		quoted = append(quoted, strconv.Quote(value))
	}
	return "[" + strings.Join(quoted, ", ") + "]"
}

func contains(values []string, target string) bool {
	for _, value := range values {
		if value == target {
			return true
		}
	}
	return false
}

func removeExact(values []string, target string) []string {
	result := values[:0]
	for _, value := range values {
		if value != target {
			result = append(result, value)
		}
	}
	return result
}
