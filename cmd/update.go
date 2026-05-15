package cmd

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

func fetchLatestVersion(current string) string {
	if current == "dev" {
		return ""
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Get("https://api.github.com/repos/webbhalsa/accessboss-cli/releases/latest")
	if err != nil || resp.StatusCode != http.StatusOK {
		return ""
	}
	defer resp.Body.Close()

	var release struct {
		TagName string `json:"tag_name"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return ""
	}

	latest := strings.TrimPrefix(release.TagName, "v")
	cur := strings.TrimPrefix(current, "v")
	if latest == cur {
		return ""
	}
	return release.TagName
}
