package container

import (
	"strings"
)

// Resolving {{.container.host}} and {{.container.port.NNNN}} in a container's
// own env, which can refer to containers started before it.

// resolveEnvTemplates resolves template variables in environment values
// Supports {{.container_name.host}} and {{.container_name.port.XXXX}} patterns
func (m *Manager) resolveEnvTemplates(env map[string]string) map[string]string {
	if env == nil {
		return nil
	}

	result := make(map[string]string)
	for key, value := range env {
		resolved := value

		// Find all template patterns. Scanning resumes after each replacement:
		// a template that does not resolve is returned unchanged, and searching
		// from the start again would find it forever.
		for pos := 0; pos < len(resolved); {
			start := strings.Index(resolved[pos:], "{{")
			if start == -1 {
				break
			}
			start += pos
			end := strings.Index(resolved[start:], "}}")
			if end == -1 {
				break
			}
			end += start + 2

			template := resolved[start:end]
			replacement := m.resolveTemplate(template)
			resolved = resolved[:start] + replacement + resolved[end:]
			pos = start + len(replacement)
		}

		result[key] = resolved
	}

	return result
}

// resolveTemplate resolves a single template like {{.zookeeper.host}}
func (m *Manager) resolveTemplate(template string) string {
	// Remove {{ and }} and trim spaces
	inner := strings.TrimSpace(template[2 : len(template)-2])

	// Expected format: .container_name.host or .container_name.port.XXXX
	if !strings.HasPrefix(inner, ".") {
		return template
	}

	parts := strings.Split(inner[1:], ".")
	if len(parts) < 2 {
		return template
	}

	containerName := parts[0]
	infoType := parts[1]

	switch infoType {
	case "host":
		// Return container DNS name (for internal Docker network communication)
		return containerName
	case "port":
		if len(parts) < 3 {
			return template
		}
		// Return the internal port (not mapped) for container-to-container communication
		port := parts[2]
		// Strip /tcp suffix if present
		if idx := strings.Index(port, "/"); idx > 0 {
			port = port[:idx]
		}
		return port
	}

	return template
}
