package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	repoAPI    = "https://api.github.com/repos/praydog/REFramework-nightly/releases"
	cacheDir   = ".cache_github"
	cacheBody  = cacheDir + "/releases.json"
	cacheEtag  = cacheDir + "/etag"
	zipName    = "MHWILDS.zip"
)

type Release struct {
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
}

type ProgressReader struct {
	io.Reader
	Total   int64
	Current int64
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.Current += int64(n)
	if pr.Total > 0 {
		fmt.Printf("\r==> Downloading %s... [%.2f%%]", zipName, float64(pr.Current)*100/float64(pr.Total))
	}
	return n, err
}

func main() {
	// 1. Fetching releases and allow selection like the shell script
	fmt.Println("==> Fetching recent dev releases...")
	// Read env overrides
	devPrefix := os.Getenv("DEV_PREFIX")
	maxList := 20
	if v := os.Getenv("MAX_LIST"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxList = n
		}
	}
	// If interactive terminal, prompt for MAX_LIST
	if fi, _ := os.Stdin.Stat(); (fi.Mode() & os.ModeCharDevice) != 0 {
		fmt.Printf("How many releases to display? [%d]: ", maxList)
		var input string
		fmt.Scanln(&input)
		if input != "" {
			if n, err := strconv.Atoi(input); err == nil && n > 0 {
				maxList = n
			} else {
				fmt.Printf("Invalid number, using %d\n", maxList)
			}
		}
	}

	// 1. Fetching releases with ETag caching
	os.MkdirAll(cacheDir, 0755)
	etag, _ := os.ReadFile(cacheEtag)
	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("GET", repoAPI+"?per_page=100", nil)
	if sEtag := strings.TrimSpace(string(etag)); sEtag != "" {
		req.Header.Set("If-None-Match", sEtag)
	}

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("Error fetching releases: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var releases []Release
	if resp.StatusCode == http.StatusNotModified {
		// Use cache
		data, err := os.ReadFile(cacheBody)
		if err != nil {
			fmt.Printf("Error reading cache: %v\n", err)
			os.Exit(1)
		}
		if err := json.Unmarshal(data, &releases); err != nil {
			fmt.Printf("Error parsing cached JSON: %v\n", err)
			os.Exit(1)
		}
	} else if resp.StatusCode == http.StatusOK {
		// Update cache
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("Error reading response: %v\n", err)
			os.Exit(1)
		}
		if err := json.Unmarshal(data, &releases); err != nil {
			fmt.Printf("Error decoding JSON: %v\n", err)
			os.Exit(1)
		}
		os.WriteFile(cacheBody, data, 0644)
		if newEtag := resp.Header.Get("ETag"); newEtag != "" {
			os.WriteFile(cacheEtag, []byte(newEtag), 0644)
		}
	} else {
		// Fail if no cache, or use old cache if available
		if data, err := os.ReadFile(cacheBody); err == nil {
			json.Unmarshal(data, &releases)
		} else {
			fmt.Printf("Error: API returned status %d and no cache available.\n", resp.StatusCode)
			os.Exit(1)
		}
	}

	var tag string
	var pubDate time.Time
	// Build map of numeric -> (published_at, tag) keeping most recent per numeric
	numMap := make(map[string]Release)
	re := regexp.MustCompile(`^nightly-(\d{4,})-([A-Za-z0-9]+)$`)
	for _, r := range releases {
		m := re.FindStringSubmatch(r.TagName)
		if len(m) == 0 {
			continue
		}
		num := m[1]
		if devPrefix != "" && !strings.HasPrefix(num, devPrefix) {
			continue
		}
		cur, ok := numMap[num]
		if !ok || r.PublishedAt.After(cur.PublishedAt) {
			numMap[num] = r
		}
	}

	// Create sorted list by publish date desc
	type item struct {
		Num string
		Rel Release
	}
	items := make([]item, 0, len(numMap))
	for k, v := range numMap {
		items = append(items, item{Num: k, Rel: v})
	}
	sort.Slice(items, func(i, j int) bool { return items[i].Rel.PublishedAt.After(items[j].Rel.PublishedAt) })

	if len(items) == 0 {
		fmt.Println("Error: Could not find any nightly numeric releases.")
		os.Exit(1)
	}

	// Print summary and menu (limit to maxList)
	total := len(items)
	fmt.Printf("Found %d numeric nightly version(s).\n", total)
	fmt.Printf("Available numeric nightly versions (showing up to %d newest -> oldest):\n", maxList)
	limit := maxList
	if limit > total {
		limit = total
	}
	for i := 0; i < limit; i++ {
		it := items[i]
		fmt.Printf(" %d. %s  (%s)  %s\n", i+1, it.Num, it.Rel.TagName, it.Rel.PublishedAt.Format("2006-01-02 15:04:05"))
	}

	// Prompt selection
	var choice int
	fmt.Printf("Choose numeric version (1-%d) [1]: ", limit)
	_, err = fmt.Scanln(&choice)
	if err != nil || choice < 1 || choice > limit {
		choice = 1
	}
	sel := items[choice-1]
	tag = sel.Rel.TagName
	pubDate = sel.Rel.PublishedAt

	// Build version string for filename: nightly-<num>-<6chars>
	m := re.FindStringSubmatch(tag)
	version := tag
	if len(m) == 3 {
		shortHash := m[2]
		if len(shortHash) > 6 {
			shortHash = shortHash[:6]
		}
		// include the 'nightly-' prefix to match the shell script
		version = fmt.Sprintf("nightly-%s-%s", m[1], shortHash)
	}
	finalZip := fmt.Sprintf("REFramework_%s_%s.zip", version, pubDate.Format("02Jan06"))

	if _, err := os.Stat(finalZip); err == nil {
		fmt.Printf("==> Archive %s already exists.\n", finalZip)
		fmt.Print("Do you want to rebuild it anyway? (y/N): ")
		var confirm string
		fmt.Scanln(&confirm)
		if strings.ToLower(confirm) != "y" {
			fmt.Println("==> Skipping rebuild. Exiting.")
			os.Exit(0)
		}
	}

	// 2. Downloading with progress
	url := fmt.Sprintf("https://github.com/praydog/REFramework-nightly/releases/download/%s/MHWILDS.zip", tag)
	fmt.Printf("==> Found tag: %s\n", tag)

	// Support SKIP_DOWNLOAD env for testing
	if os.Getenv("SKIP_DOWNLOAD") == "1" {
		fmt.Println("SKIP_DOWNLOAD=1 - test mode")
		fmt.Printf("Selected TAG: %s\nPublish date: %s\nWould create: %s\n", tag, pubDate.Format(time.RFC3339), finalZip)
		return
	}

	out, err := os.Create(zipName)
	if err != nil {
		fmt.Printf("Error creating file: %v\n", err)
		os.Exit(1)
	}
	defer out.Close()

	resp, err = http.Get(url)
	if err != nil {
		fmt.Printf("Error downloading file: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	progressReader := &ProgressReader{
		Reader: resp.Body,
		Total:  resp.ContentLength,
	}

	_, err = io.Copy(out, progressReader)
	fmt.Println() // New line after progress
	if err != nil {
		fmt.Printf("Error saving file: %v\n", err)
		os.Exit(1)
	}

	// 3. Unzipping with integrated filtering
	// Use RAM disk if available for better performance
	tmpRoot, err := os.MkdirTemp("/dev/shm", "reframework-*")
	if err != nil {
		tmpRoot, _ = os.MkdirTemp("", "reframework-*")
	}
	defer os.RemoveAll(tmpRoot)

	extractDir := filepath.Join(tmpRoot, "MHWILDS")
	fmt.Println("==> Extracting and filtering...")
	// Patterns matching shell: -x "*RE*" "*vr*" "*xr*" "*VR*" "*XR*" "*DELETE*" "*OpenVR*" "*OpenXR*"
	filters := []string{"RE", "vr", "xr", "VR", "XR", "DELETE", "OpenVR", "OpenXR"}
	if err := unzipFiltered(zipName, extractDir, filters); err != nil {
		fmt.Printf("Error unzipping: %v\n", err)
		os.Exit(1)
	}

	// 5. Zipping
	fmt.Printf("==> Creating optimized archive: %s\n", finalZip)
	if err := createZip(finalZip, extractDir); err != nil {
		fmt.Printf("Error creating final zip: %v\n", err)
		os.Exit(1)
	}

	// 6. Final Cleanup
	os.Remove(zipName)

	statusLine := fmt.Sprintf("==> Finished! Created: %s", finalZip)
	fmt.Printf("\033[1;34m==>\033[0m %s\n", statusLine[4:])

	// 7. Show summary of archive contents
	fmt.Printf("Archive Summary (%s):\n", finalZip)
	zf, err := zip.OpenReader(finalZip)
	if err == nil {
		count := 0
		for _, f := range zf.File {
			fmt.Printf("  %s\n", f.Name)
			if !f.FileInfo().IsDir() {
				count++
			}
		}
		zf.Close()
		fmt.Printf("Total files: %d\n", count)
	}
}

func unzipFiltered(src, dest string, filters []string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Filter out files matching any of the patterns (case-sensitive like shell unzip -x)
		skip := false
		for _, pattern := range filters {
			if strings.Contains(f.Name, pattern) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		fpath := filepath.Join(dest, f.Name)
		if !strings.HasPrefix(fpath, filepath.Clean(dest)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}
	return nil
}

func createZip(filename, sourceDir string) error {
	newZipFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer newZipFile.Close()

	zipWriter := zip.NewWriter(newZipFile)
	defer zipWriter.Close()

	return filepath.Walk(sourceDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		// Maintain the "MHWILDS/" prefix in the final zip
		zipPath := filepath.Join("MHWILDS", relPath)
		if info.IsDir() {
			_, err = zipWriter.Create(zipPath + "/")
			return err
		}

		zipFile, err := zipWriter.Create(zipPath)
		if err != nil {
			return err
		}

		fsFile, err := os.Open(path)
		if err != nil {
			return err
		}
		defer fsFile.Close()

		_, err = io.Copy(zipFile, fsFile)
		return err
	})
}
