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

	"github.com/ncruces/zenity"
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

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.Current += int64(n)
	if pr.Total > 0 && pr.OnProgress != nil {
		pct := int(float64(pr.Current) * 100 / float64(pr.Total))
		pr.OnProgress(pct)
	}
	return n, err
}

type ProgressReader struct {
	io.Reader
	Total      int64
	Current    int64
	OnProgress func(int)
}

func pause() {
	// Removed for GUI version
}

func main() {
	defer pause()

	// Direct variable declarations to avoid goto scope issues
	var stagingZip, stagingFinal, tmpDir string
	var choice int
	var err error
	var dlg, dlgTrans, dlgFetch zenity.ProgressDialog

	// filters and defaults
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
		res, err := zenity.Entry("How many recent releases should I fetch and list?", 
			zenity.Title("REFramework Build Setup"), 
			zenity.EntryText(strconv.Itoa(maxList)),
			zenity.Width(500), zenity.Height(200))
		if err == nil && res != "" {
			if n, err := strconv.Atoi(res); err == nil && n > 0 {
				maxList = n
			}
		}
	}

	// Fetching releases
	dlgFetch, _ = zenity.Progress(zenity.Title("REFramework Build"), zenity.NoCancel(),
		zenity.Width(600), zenity.Height(150))
	dlgFetch.Text("Fetching recent nightly releases...")
	defer dlgFetch.Close()

	os.MkdirAll(cacheDir, 0755)
	etag, _ := os.ReadFile(cacheEtag)
	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("GET", repoAPI+"?per_page=100", nil)
	if sEtag := strings.TrimSpace(string(etag)); sEtag != "" {
		req.Header.Set("If-None-Match", sEtag)
	}

	resp, err := client.Do(req)
	if err != nil {
		zenity.Error(fmt.Sprintf("Error fetching releases: %v", err), 
			zenity.Title("REFramework Build Error"),
			zenity.Width(500), zenity.Height(200))
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
			zenity.Error(fmt.Sprintf("Error: API returned status %d and no cache available.", resp.StatusCode), 
				zenity.Title("REFramework Build Error"),
				zenity.Width(500), zenity.Height(200))
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

	dlgFetch.Value(100)
	dlgFetch.Close()

	if len(items) == 0 {
		zenity.Error("Error: Could not find any nightly numeric releases.", 
			zenity.Title("REFramework Build Error"),
			zenity.Width(500), zenity.Height(200))
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
	} else if maxList == 1 && limit >= 1 {
		choice = 1
	} else {
		options := make([]string, 0, limit)
		for i := 0; i < limit; i++ {
			it := items[i]
			options = append(options, fmt.Sprintf("%s (%s) - %s", it.Num, it.Rel.TagName, it.Rel.PublishedAt.Format("2006-01-02 15:04")))
		}

		selected, err := zenity.List("Select a numeric version to build:", options, 
			zenity.Title("REFramework Build"),
			zenity.Width(800), zenity.Height(600))
		if err != nil || selected == "" {
			os.Exit(2) // Cancel
		}

		// Map selection back to index
		for i, opt := range options {
			if opt == selected {
				choice = i + 1
				break
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
		if silent {
			// Auto rebuild in silent mode
		} else {
			err := zenity.Question(fmt.Sprintf("Archive %s already exists. Rebuild it anyway?", finalZip), 
				zenity.Title("REFramework Build"), 
				zenity.NoWrap(),
				zenity.Width(500), zenity.Height(200))
			if err != nil {
				return
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
	if os.Getenv("SKIP_DOWNLOAD") == "1" {
		goto finalize
	}

	{
		dlg, _ = zenity.Progress(zenity.Title("REFramework Build"),
			zenity.Width(600), zenity.Height(150))
		dlg.Text("Downloading nightly release...")
		defer dlg.Close()

		url := fmt.Sprintf("https://github.com/praydog/REFramework-nightly/releases/download/%s/MHWILDS.zip", tag)
		resp, err = http.Get(url)
		if err != nil {
			zenity.Error(fmt.Sprintf("Error downloading: %v", err), 
				zenity.Title("REFramework Build Error"),
				zenity.Width(500), zenity.Height(200))
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			zenity.Error(fmt.Sprintf("Error: API returned status %s", resp.Status), 
				zenity.Title("REFramework Build Error"),
				zenity.Width(500), zenity.Height(200))
			return
		}

		out, err := os.Create(stagingZip)
		if err != nil {
			zenity.Error(fmt.Sprintf("Error creating staging file: %v", err), 
				zenity.Title("REFramework Build Error"),
				zenity.Width(500), zenity.Height(200))
			return
		}

		progressReader := &ProgressReader{
			Reader: resp.Body, 
			Total: resp.ContentLength,
			OnProgress: func(pct int) {
				dlg.Value(pct)
			},
		}
		_, err = io.Copy(out, progressReader)
		out.Close()

		if err != nil {
			zenity.Error(fmt.Sprintf("Error saving staging file: %v", err), 
				zenity.Title("REFramework Build Error"),
				zenity.Width(500), zenity.Height(200))
			return
		}
	}

	// 4. Transcoding (Staging)
	dlgTrans, _ = zenity.Progress(zenity.Title("REFramework Build"),
		zenity.Width(600), zenity.Height(150))
	dlgTrans.Text("Creating optimized archive...")
	if err := transcodeZip(stagingZip, stagingFinal, filters, func(pct int) {
		dlgTrans.Value(pct)
	}); err != nil {
		dlgTrans.Close()
		zenity.Error(fmt.Sprintf("Error creating archive: %v", err), 
			zenity.Title("REFramework Build Error"),
			zenity.Width(500), zenity.Height(200))
		return
	}
	dlgTrans.Close()

	// 5. Atomic Move to current directory
	if err := copyFile(stagingFinal, finalZip); err != nil {
		zenity.Error(fmt.Sprintf("Error moving final archive: %v", err), 
			zenity.Title("REFramework Build Error"),
			zenity.Width(500), zenity.Height(200))
		return
	}

finalize:
	if _, err := os.Stat(finalZip); err != nil {
		zenity.Error(fmt.Sprintf("Critical Error: Final archive %s not found!", finalZip), 
			zenity.Title("REFramework Build Error"),
			zenity.Width(500), zenity.Height(200))
		return
	}

	// Summary omitted for GUI version to avoid console noise

	// 6. Windows-specific: Offer to copy to Downloads
	home, err := os.UserHomeDir()
	if err == nil {
		winDownloads := filepath.Join(home, "Downloads")
		if _, err := os.Stat(winDownloads); err == nil {
			dest := filepath.Join(winDownloads, finalZip)
			if silent {
				atomicCopy(finalZip, dest)
			} else {
				err := zenity.Question("Would you like to copy the resulting archive to your Windows Downloads folder?", 
					zenity.Title("REFramework Build"),
					zenity.Width(500), zenity.Height(200))
				if err == nil {
					if err := atomicCopy(finalZip, dest); err == nil {
						zenity.Info("Successfully updated/copied to Downloads folder.", 
							zenity.Title("REFramework Build"),
							zenity.Width(500), zenity.Height(200))
					} else {
						zenity.Error(fmt.Sprintf("Error copying to Downloads: %v", err), 
							zenity.Title("REFramework Build Error"),
							zenity.Width(500), zenity.Height(200))
					}
				} else {
					zenity.Info("Build complete. Archive is in the current directory.", 
						zenity.Title("REFramework Build"),
						zenity.Width(500), zenity.Height(200))
				}
			}
		} else {
			zenity.Info("Build complete. Archive is in the current directory.", 
				zenity.Title("REFramework Build"),
				zenity.Width(500), zenity.Height(200))
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

func transcodeZip(src, dest string, filters []string, onProgress func(int)) error {
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

	totalFiles := len(sReader.File)
	processedFiles := 0

	for _, f := range sReader.File {
		processedFiles++
		if onProgress != nil {
			onProgress(int(float64(processedFiles) * 100 / float64(totalFiles)))
		}

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
