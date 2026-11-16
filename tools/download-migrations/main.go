package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
)

const apiUrl = "https://git.jgj52.dev/api/v1/repos/MCAuth/api/contents/migration"

type contentsEntry struct {
	Name        string `json:"name"`
	Type        string `json:"type"`
	DownloadURL string `json:"download_url"`
}

func main() {
	resp, err := http.Get(apiUrl)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	var entries []contentsEntry
	if err := json.NewDecoder(resp.Body).Decode(&entries); err != nil {
		panic(err)
	}

	dir := filepath.Join("build", "sqlc-migrations")
	if err := os.MkdirAll(dir, 0755); err != nil {
		panic(err)
	}

	for _, entry := range entries {
		if entry.Type != "file" || !strings.HasSuffix(entry.Name, ".sql") {
			continue
		}

		downloadResp, err := http.Get(entry.DownloadURL)
		if err != nil {
			panic(err)
		}

		out, err := os.Create(filepath.Join(dir, entry.Name))
		if err != nil {
			panic(err)
		}

		if _, err := io.Copy(out, downloadResp.Body); err != nil {
			panic(err)
		}
		out.Close()
		downloadResp.Body.Close()
		fmt.Println("Downloaded " + entry.Name)
	}
}
