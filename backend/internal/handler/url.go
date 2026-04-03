package handler

import "strings"

func normalizeURL(raw string) string {
	raw = strings.TrimSpace(raw)
	if !strings.HasPrefix(raw, "http://") && !strings.HasPrefix(raw, "https://") {
		raw = "https://" + raw
	}
	return strings.TrimSuffix(raw, ".git")
}

func extractRepoName(url string) string {
	parts := strings.Split(strings.TrimSuffix(url, "/"), "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return url
}
