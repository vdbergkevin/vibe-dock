package vibe

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"

	"github.com/BurntSushi/toml"
)

var (
	connectorConfigMu = sync.Mutex{}
	disabledLine      = regexp.MustCompile(`^(\s*disabled\s*=\s*)(true|false)(\s*(?:#.*)?)$`)
	disabledInline    = regexp.MustCompile(`\bdisabled(\s*=\s*)(true|false)\b`)
	masterLine        = regexp.MustCompile(`^(\s*enable_connectors\s*=\s*)(true|false)(\s*(?:#.*)?)$`)
	connectorArray    = regexp.MustCompile(`(?m)^[\t ]*connectors[\t ]*=[\t ]*\[`)
)

// SetConnectorEnabled persists the same connector selection managed by Vibe's
// /mcp panel. It edits only the relevant TOML table so unrelated settings,
// comments, and provider configuration remain byte-for-byte intact.
func SetConnectorEnabled(configPath, connectorName string, enabled bool) error {
	name := normalizeName(connectorName)
	if name == "" || sourceKey(name) != sourceKey(connectorName) {
		return errors.New("invalid connector name")
	}

	connectorConfigMu.Lock()
	defer connectorConfigMu.Unlock()

	resolvedPath, err := resolveConfigPath(configPath)
	if err != nil {
		return err
	}
	contents, mode, err := readConfigText(resolvedPath)
	if err != nil {
		return err
	}
	if strings.TrimSpace(contents) != "" {
		var document map[string]any
		if _, err := toml.Decode(contents, &document); err != nil {
			return fmt.Errorf("parse Vibe config before updating connector: %w", err)
		}
	}

	updated, err := updateConnectorConfig(contents, name, enabled)
	if err != nil {
		return err
	}
	var updatedDocument map[string]any
	if _, err := toml.Decode(updated, &updatedDocument); err != nil {
		return fmt.Errorf("validate Vibe config after updating connector: %w", err)
	}
	if updated == contents {
		return nil
	}
	return writeConfigAtomically(resolvedPath, updated, mode)
}

func resolveConfigPath(path string) (string, error) {
	info, err := os.Lstat(path)
	if err == nil && info.Mode()&os.ModeSymlink != 0 {
		resolved, resolveErr := filepath.EvalSymlinks(path)
		if resolveErr != nil {
			return "", fmt.Errorf("resolve Vibe config symlink: %w", resolveErr)
		}
		return resolved, nil
	}
	if err != nil && !os.IsNotExist(err) {
		return "", fmt.Errorf("inspect Vibe config: %w", err)
	}
	return path, nil
}

func readConfigText(path string) (string, os.FileMode, error) {
	contents, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return "", 0o600, nil
		}
		return "", 0, fmt.Errorf("read Vibe config: %w", err)
	}
	mode := os.FileMode(0o600)
	if info, statErr := os.Stat(path); statErr == nil {
		mode = info.Mode().Perm()
	}
	return string(contents), mode, nil
}

func updateConnectorConfig(contents, name string, enabled bool) (string, error) {
	arrayStart, arrayEnd, foundArray, err := inlineConnectorArray(contents)
	if err != nil {
		return "", err
	}
	if foundArray {
		updated, updateErr := updateInlineConnectorArray(contents, arrayStart, arrayEnd, name, enabled)
		if updateErr != nil {
			return "", updateErr
		}
		if enabled {
			updated = setMasterConnectorSwitchText(updated)
		}
		return updated, nil
	}

	newline := "\n"
	if strings.Contains(contents, "\r\n") {
		newline = "\r\n"
	}
	lines := strings.Split(strings.ReplaceAll(contents, "\r\n", "\n"), "\n")
	hadFinalNewline := strings.HasSuffix(contents, "\n")

	start, end := connectorTable(lines, name)
	if start >= 0 {
		value := "true"
		if enabled {
			value = "false"
		}
		replaced := false
		for index := start + 1; index < end; index++ {
			if disabledLine.MatchString(lines[index]) {
				lines[index] = disabledLine.ReplaceAllString(lines[index], "${1}"+value+"${3}")
				replaced = true
				break
			}
		}
		if !replaced {
			insertAt := start + 1
			for index := start + 1; index < end; index++ {
				if strings.HasPrefix(strings.TrimSpace(lines[index]), "name") {
					insertAt = index + 1
					break
				}
			}
			lines = insertLine(lines, insertAt, "disabled = "+value)
		}
	} else {
		for len(lines) > 0 && lines[len(lines)-1] == "" {
			lines = lines[:len(lines)-1]
		}
		if len(lines) > 0 {
			lines = append(lines, "")
		}
		lines = append(lines, "[[connectors]]", `name = "`+name+`"`, "disabled = "+map[bool]string{true: "false", false: "true"}[enabled])
	}

	if enabled {
		setMasterConnectorSwitch(lines)
	}
	result := strings.Join(lines, newline)
	if hadFinalNewline || contents == "" {
		result = strings.TrimRight(result, "\r\n") + newline
	}
	return result, nil
}

// inlineConnectorArray locates the array syntax emitted by current Vibe
// versions: connectors = [{ name = "…", disabled = true }]. Older versions
// used [[connectors]] tables, which remain supported below.
func inlineConnectorArray(contents string) (int, int, bool, error) {
	location := connectorArray.FindStringIndex(contents)
	if location == nil {
		return 0, 0, false, nil
	}
	relative := strings.LastIndex(contents[location[0]:location[1]], "[")
	if relative < 0 {
		return 0, 0, false, errors.New("could not locate Vibe connector array")
	}
	start := location[0] + relative
	end := matchingDelimiter(contents, start, '[', ']')
	if end < 0 {
		return 0, 0, false, errors.New("could not find the end of Vibe connector array")
	}
	return start, end, true, nil
}

func updateInlineConnectorArray(contents string, start, end int, name string, enabled bool) (string, error) {
	value := "true"
	if enabled {
		value = "false"
	}
	for _, objectRange := range inlineObjectRanges(contents, start+1, end) {
		object := contents[objectRange[0]:objectRange[1]]
		var parsed configFile
		if _, err := toml.Decode("connectors = ["+object+"]", &parsed); err != nil || len(parsed.Connectors) != 1 || sourceKey(parsed.Connectors[0].Name) != sourceKey(name) {
			continue
		}
		updatedObject := object
		if disabledInline.MatchString(object) {
			updatedObject = disabledInline.ReplaceAllString(object, "disabled${1}"+value)
		} else {
			closing := strings.LastIndex(object, "}")
			body := strings.TrimRight(object[:closing], " \t\r\n")
			separator := ", "
			if strings.HasSuffix(body, "{") {
				separator = " "
			}
			updatedObject = body + separator + "disabled = " + value + object[len(body):]
		}
		return contents[:objectRange[0]] + updatedObject + contents[objectRange[1]:], nil
	}

	entry := `{ name = "` + name + `", disabled = ` + value + ` }`
	body := contents[start+1 : end]
	if strings.Contains(body, "\n") || strings.Contains(body, "\r") {
		lineStart := strings.LastIndexAny(body, "\r\n") + 1
		closingIndent := body[lineStart:]
		if strings.TrimSpace(closingIndent) != "" {
			lineStart = len(body)
			closingIndent = ""
		}
		indent := "    "
		if match := regexp.MustCompile(`(?m)^([\t ]*)\{`).FindStringSubmatchIndex(body); match != nil {
			indent = body[match[2]:match[3]]
		}
		prefix := body[:lineStart]
		if prefix != "" && !strings.HasSuffix(prefix, "\n") && !strings.HasSuffix(prefix, "\r") {
			prefix += detectNewline(contents)
		}
		body = prefix + indent + entry + "," + detectNewline(contents) + closingIndent
	} else {
		trimmed := strings.TrimSpace(body)
		trailing := body[len(strings.TrimRight(body, " \t")):]
		separator := ""
		if trimmed != "" {
			separator = ", "
			if strings.HasSuffix(trimmed, ",") {
				separator = " "
			}
		}
		body = strings.TrimRight(body, " \t") + separator + entry + trailing
	}
	return contents[:start+1] + body + contents[end:], nil
}

func inlineObjectRanges(contents string, start, end int) [][2]int {
	ranges := [][2]int{}
	depth := 0
	objectStart := -1
	quote := byte(0)
	escaped := false
	comment := false
	for index := start; index < end; index++ {
		character := contents[index]
		if comment {
			if character == '\n' || character == '\r' {
				comment = false
			}
			continue
		}
		if quote != 0 {
			if quote == '"' && character == '\\' && !escaped {
				escaped = true
				continue
			}
			if character == quote && !escaped {
				quote = 0
			}
			escaped = false
			continue
		}
		switch character {
		case '#':
			comment = true
		case '\'', '"':
			quote = character
		case '{':
			if depth == 0 {
				objectStart = index
			}
			depth++
		case '}':
			if depth > 0 {
				depth--
				if depth == 0 && objectStart >= 0 {
					ranges = append(ranges, [2]int{objectStart, index + 1})
					objectStart = -1
				}
			}
		}
	}
	return ranges
}

func matchingDelimiter(contents string, start int, opening, closing byte) int {
	depth := 0
	quote := byte(0)
	escaped := false
	comment := false
	for index := start; index < len(contents); index++ {
		character := contents[index]
		if comment {
			if character == '\n' || character == '\r' {
				comment = false
			}
			continue
		}
		if quote != 0 {
			if quote == '"' && character == '\\' && !escaped {
				escaped = true
				continue
			}
			if character == quote && !escaped {
				quote = 0
			}
			escaped = false
			continue
		}
		switch character {
		case '#':
			comment = true
		case '\'', '"':
			quote = character
		case opening:
			depth++
		case closing:
			depth--
			if depth == 0 {
				return index
			}
		}
	}
	return -1
}

func detectNewline(contents string) string {
	if strings.Contains(contents, "\r\n") {
		return "\r\n"
	}
	return "\n"
}

func setMasterConnectorSwitchText(contents string) string {
	newline := detectNewline(contents)
	lines := strings.Split(strings.ReplaceAll(contents, "\r\n", "\n"), "\n")
	setMasterConnectorSwitch(lines)
	return strings.Join(lines, newline)
}

func connectorTable(lines []string, name string) (int, int) {
	for index := 0; index < len(lines); index++ {
		if strings.TrimSpace(lines[index]) != "[[connectors]]" {
			continue
		}
		end := len(lines)
		for next := index + 1; next < len(lines); next++ {
			trimmed := strings.TrimSpace(lines[next])
			if strings.HasPrefix(trimmed, "[") {
				end = next
				break
			}
		}
		block := strings.Join(lines[index:end], "\n")
		var parsed configFile
		if _, err := toml.Decode(block, &parsed); err == nil && len(parsed.Connectors) == 1 && sourceKey(parsed.Connectors[0].Name) == sourceKey(name) {
			return index, end
		}
		index = end - 1
	}
	return -1, -1
}

func setMasterConnectorSwitch(lines []string) {
	for index, line := range lines {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[") {
			break
		}
		if masterLine.MatchString(line) {
			lines[index] = masterLine.ReplaceAllString(line, "${1}true${3}")
			return
		}
	}
}

func insertLine(lines []string, index int, value string) []string {
	lines = append(lines, "")
	copy(lines[index+1:], lines[index:])
	lines[index] = value
	return lines
}

func writeConfigAtomically(path, contents string, mode os.FileMode) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create Vibe config directory: %w", err)
	}
	temporary, err := os.CreateTemp(filepath.Dir(path), ".config.toml.vibedock-*")
	if err != nil {
		return fmt.Errorf("create temporary Vibe config: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err := temporary.Chmod(mode); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("secure temporary Vibe config: %w", err)
	}
	if _, err := temporary.WriteString(contents); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary Vibe config: %w", err)
	}
	if err := temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary Vibe config: %w", err)
	}
	if err := temporary.Close(); err != nil {
		return fmt.Errorf("close temporary Vibe config: %w", err)
	}
	if err := os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace Vibe config: %w", err)
	}
	return nil
}
