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

func pause() {
	if os.Getenv("SILENT") == "1" {
		return
	}
	fmt.Print("\nPress Enter to exit...")
	fmt.Scanln()
}

func main() {
	defer pause()

	// Direct variable declarations to avoid goto scope issues
	var stagingZip, stagingFinal, tmpDir string
	var choice int
	var err error

	// 1. Fetching releases and allow selection
	fmt.Println("==> Fetching recent dev releases...")
	devPrefix := os.Getenv("DEV_PREFIX")
	filters := []string{"RE", "vr", "xr", "VR", "XR", "DELETE", "OpenVR", "OpenXR"}
	maxList := 20
	if v := os.Getenv("MAX_LIST"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			maxList = n
		}
	}
	
	silent := os.Getenv("SILENT") == "1"
	if !silent {
		if fi, _ := os.Stdin.Stat(); (fi.Mode() & os.ModeCharDevice) != 0 {
			fmt.Printf("How many releases to display? [%d]: ", maxList)
			var input string
			fmt.Scanln(&input)
			if input != "" {
				if n, err := strconv.Atoi(input); err == nil && n > 0 {
					maxList = n
				}
			}
		}
	}

	// Fetching releases
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
		return
	}
	defer resp.Body.Close()

	var releases []Release
	if resp.StatusCode == http.StatusNotModified {
		f, err := os.Open(cacheBody)
		if err == nil {
			defer f.Close()
			json.NewDecoder(f).Decode(&releases)
		}
	} else if resp.StatusCode == http.StatusOK {
		data, err := io.ReadAll(resp.Body)
		if err == nil {
			if json.Unmarshal(data, &releases) == nil {
				os.WriteFile(cacheBody, data, 0644)
				if newEtag := resp.Header.Get("ETag"); newEtag != "" {
					os.WriteFile(cacheEtag, []byte(newEtag), 0644)
				}
			}
		}
	} else {
		if f, err := os.Open(cacheBody); err == nil {
			defer f.Close()
			json.NewDecoder(f).Decode(&releases)
		} else {
			fmt.Printf("Error: API returned status %d and no cache available.\n", resp.StatusCode)
			return
		}
	}

	re := regexp.MustCompile(`^nightly-(\d{4,})-([A-Za-z0-9]+)$`)
	numMap := make(map[string]Release)
	for _, r := range releases {
		m := re.FindStringSubmatch(r.TagName)
		if len(m) == 0 { continue }
		num := m[1]
		if devPrefix != "" && !strings.HasPrefix(num, devPrefix) { continue }
		cur, ok := numMap[num]
		if !ok || r.PublishedAt.After(cur.PublishedAt) {
			numMap[num] = r
		}
	}

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
		return
	}

	total := len(items)
	fmt.Printf("Found %d numeric nightly version(s).\n", total)
	limit := maxList
	if limit > total { limit = total }
	for i := 0; i < limit; i++ {
		it := items[i]
		fmt.Printf(" %d. %s  (%s)  %s\n", i+1, it.Num, it.Rel.TagName, it.Rel.PublishedAt.Format("2006-01-02 15:04:05"))
	}

	if silent {
		choice = 1
		fmt.Printf("Silent Mode: Automatically chose version 1 (%s)\n", items[0].Num)
	} else if maxList == 1 && limit >= 1 {
		choice = 1
		fmt.Printf("Display limit is 1: Automatically selecting latest version (%s)\n", items[0].Num)
	} else {
		fmt.Printf("Choose numeric version (1-%d) [1] (or 0 to exit): ", limit)
		var input string
		fmt.Scanln(&input)
		if input == "" {
			choice = 1
		} else if input == "0" {
			fmt.Println("Exiting as requested.")
			os.Exit(2)
		} else {
			choice, _ = strconv.Atoi(input)
			if choice < 1 || choice > limit {
				choice = 1
			}
		}
	}
	sel := items[choice-1]
	tag := sel.Rel.TagName
	pubDate := sel.Rel.PublishedAt

	m := re.FindStringSubmatch(tag)
	version := tag
	if len(m) == 3 {
		shortHash := m[2]
		if len(shortHash) > 6 { shortHash = shortHash[:6] }
		version = fmt.Sprintf("nightly-%s-%s", m[1], shortHash)
	}
	finalZip := fmt.Sprintf("REFramework_%s_%s.zip", version, pubDate.Format("02Jan06"))

	if _, err := os.Stat(finalZip); err == nil {
		fmt.Printf("==> Archive %s already exists.\n", finalZip)
		if silent {
			fmt.Println("Silent Mode: Rebuilding existing archive.")
		} else {
			fmt.Print("Do you want to rebuild it anyway? (y/N): ")
			var confirm string
			fmt.Scanln(&confirm)
			if strings.ToLower(confirm) != "y" {
				fmt.Println("==> Skipping rebuild.")
				if silent { return }
				goto finalize
			}
		}
	}

	// 2. Setup Temporary Workspace
	tmpDir, err = os.MkdirTemp("", "reframework-build-*")
	if err != nil {
		fmt.Printf("Error creating temp dir: %v\n", err)
		return
	}
	defer os.RemoveAll(tmpDir)

	stagingZip = filepath.Join(tmpDir, zipName)
	stagingFinal = filepath.Join(tmpDir, finalZip)

	// 3. Downloading
	fmt.Printf("==> Found tag: %s\n", tag)
	if os.Getenv("SKIP_DOWNLOAD") == "1" {
		fmt.Println("SKIP_DOWNLOAD=1 - test mode")
		goto finalize
	}

	{
		url := fmt.Sprintf("https://github.com/praydog/REFramework-nightly/releases/download/%s/MHWILDS.zip", tag)
		resp, err = http.Get(url)
		if err != nil {
			fmt.Printf("(!) Error downloading: %v\n", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			fmt.Printf("(!) Error: API returned status %s\n", resp.Status)
			return
		}

		out, err := os.Create(stagingZip)
		if err != nil {
			fmt.Printf("(!) Error creating staging file: %v\n", err)
			return
		}

		progressReader := &ProgressReader{Reader: resp.Body, Total: resp.ContentLength}
		_, err = io.Copy(out, progressReader)
		if closeErr := out.Close(); closeErr != nil && err == nil {
			err = closeErr
		}
		fmt.Println()

		if err != nil {
			fmt.Printf("(!) Error saving staging file: %v\n", err)
			return
		}
	}

	// 4. Transcoding (Staging)
	fmt.Printf("==> Creating optimized archive: %s\n", finalZip)
	if err := transcodeZip(stagingZip, stagingFinal, filters); err != nil {
		fmt.Printf("(!) Error creating archive: %v\n", err)
		return
	}

	// 5. Atomic Move to current directory
	if err := copyFile(stagingFinal, finalZip); err != nil {
		fmt.Printf("(!) Error moving final archive: %v\n", err)
		return
	}

finalize:
	if _, err := os.Stat(finalZip); err != nil {
		fmt.Printf("(!) Critical Error: Final archive %s not found!\n", finalZip)
		return
	}

	fmt.Printf("\n==> Successfully created: %s\n", finalZip)
	fmt.Println("Archive Summary:")
	zf, err := zip.OpenReader(finalZip)
	if err == nil {
		count := 0
		for _, f := range zf.File {
			fmt.Printf("  %s\n", f.Name)
			if !f.FileInfo().IsDir() { count++ }
		}
		zf.Close()
		fmt.Printf("Total files: %d\n", count)
	}

	// 6. Windows-specific: Offer to copy to Downloads
	home, err := os.UserHomeDir()
	if err == nil {
		winDownloads := filepath.Join(home, "Downloads")
		if _, err := os.Stat(winDownloads); err == nil {
			dest := filepath.Join(winDownloads, finalZip)
			if silent {
				if err := atomicCopy(finalZip, dest); err == nil {
					fmt.Printf("Silent Mode: Archive ensured in %s\n", winDownloads)
				}
			} else {
				fmt.Printf("\nDo you want to copy the archive to your Downloads folder? (y/N): ")
				var confirm string
				fmt.Scanln(&confirm)
				if strings.ToLower(confirm) == "y" {
					if err := atomicCopy(finalZip, dest); err == nil {
						fmt.Printf("==> Successfully updated/copied to %s\n", winDownloads)
					} else {
						fmt.Printf("(!) Error copying: %v\n", err)
					}
				}
			}
		}
	}
}

func atomicCopy(src, dst string) error {
	absSrc, _ := filepath.Abs(src)
	absDst, _ := filepath.Abs(dst)

	if absSrc == absDst {
		// Files are already the same, skip to avoid truncation!
		return nil
	}

	return copyFile(src, dst)
}

func transcodeZip(src, dest string, filters []string) error {
	sReader, err := zip.OpenReader(src)
	if err != nil { return fmt.Errorf("open source: %w", err) }
	defer sReader.Close()

	dFile, err := os.Create(dest)
	if err != nil { return fmt.Errorf("create dest: %w", err) }
	defer dFile.Close()

	dWriter := zip.NewWriter(dFile)
	// IMPORTANT: Explicit Close to flush headers before the file stream closes
	defer dWriter.Close()

	_, err = dWriter.Create("MHWILDS/")
	if err != nil { return fmt.Errorf("create root dir: %w", err) }

	for _, f := range sReader.File {
		skip := false
		for _, p := range filters {
			if strings.Contains(f.Name, p) {
				skip = true
				break
			}
		}
		if skip { continue }

		srcFile, err := f.Open()
		if err != nil { return fmt.Errorf("open entry %s: %w", f.Name, err) }

		header := &zip.FileHeader{Name: "MHWILDS/" + f.Name, Method: zip.Deflate, Modified: f.Modified}
		destFile, err := dWriter.CreateHeader(header)
		if err != nil {
			srcFile.Close()
			return fmt.Errorf("create header %s: %w", f.Name, err)
		}

		_, err = io.Copy(destFile, srcFile)
		srcFile.Close()
		if err != nil { return fmt.Errorf("copy entry %s: %w", f.Name, err) }
	}
	
	// Finalize zip central directory explicitly
	if err := dWriter.Close(); err != nil {
		return fmt.Errorf("close zip writer: %w", err)
	}
	
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil { return err }
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil { return err }
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil { return err }
	
	return out.Close()
}
