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

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"

	"image/color"
)

const (
	repoAPI   = "https://api.github.com/repos/praydog/REFramework-nightly/releases"
	cacheDir  = ".cache_github"
	cacheBody = cacheDir + "/releases.json"
	cacheEtag = cacheDir + "/etag"
	zipName   = "MHWILDS.zip"
)

type Release struct {
	TagName     string    `json:"tag_name"`
	PublishedAt time.Time `json:"published_at"`
}

type ProgressReader struct {
	io.Reader
	Total      int64
	Current    int64
	OnProgress func(float64)
}

func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.Current += int64(n)
	if pr.Total > 0 && pr.OnProgress != nil {
		pr.OnProgress(float64(pr.Current) / float64(pr.Total))
	}
	return n, err
}

var (
	fyneApp fyne.App
	fyneWin fyne.Window
	statusLabel *widget.Label
	progressBar *widget.ProgressBar
	logText     *widget.Label
)

// setStatus updates the status label on the main window from any goroutine.
func setStatus(msg string) {
	statusLabel.SetText(msg)
}

// setProgress updates the progress bar (0.0–1.0) from any goroutine.
func setProgress(v float64) {
	progressBar.SetValue(v)
}

// showLog appends a line to the log area.
func showLog(msg string) {
	current := logText.Text
	if current != "" {
		logText.SetText(current + "\n" + msg)
	} else {
		logText.SetText(msg)
	}
}

// askEntry shows a blocking text-entry dialog. Returns ("", false) on cancel.
func askEntry(title, label, defaultVal string) (string, bool) {
	ch := make(chan struct{ val string; ok bool }, 1)
	entry := widget.NewEntry()
	entry.SetText(defaultVal)
	entry.Resize(fyne.NewSize(400, 40))
	items := []*widget.FormItem{
		{Text: label, Widget: entry},
	}
	d := dialog.NewForm(title, "OK", "Cancel", items, func(ok bool) {
		ch <- struct{ val string; ok bool }{entry.Text, ok}
	}, fyneWin)
	d.Resize(fyne.NewSize(500, 220))
	d.Show()
	result := <-ch
	return result.val, result.ok
}

// askConfirm shows a blocking yes/no dialog. Returns true on Yes.
func askConfirm(title, msg string) bool {
	ch := make(chan bool, 1)
	d := dialog.NewConfirm(title, msg, func(ok bool) {
		ch <- ok
	}, fyneWin)
	d.Resize(fyne.NewSize(500, 220))
	d.Show()
	return <-ch
}

// askList shows a blocking scrollable list dialog. Returns ("", false) on cancel.
func askList(title string, options []string) (string, bool) {
	ch := make(chan struct{ val string; ok bool }, 1)

	list := widget.NewList(
		func() int { return len(options) },
		func() fyne.CanvasObject {
			lbl := widget.NewLabel("")
			lbl.Wrapping = fyne.TextWrapOff
			return lbl
		},
		func(id widget.ListItemID, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(options[id])
		},
	)

	selected := ""
	list.OnSelected = func(id widget.ListItemID) {
		selected = options[id]
	}

	scroll := container.NewScroll(list)
	scroll.SetMinSize(fyne.NewSize(750, 450))

	var dlg dialog.Dialog
	buildBtn := widget.NewButton("Build Selected", func() {
		if selected == "" && len(options) > 0 {
			selected = options[0]
		}
		ch <- struct{ val string; ok bool }{selected, selected != ""}
		dlg.Hide()
	})
	buildBtn.Importance = widget.HighImportance

	cancelBtn := widget.NewButton("Cancel", func() {
		ch <- struct{ val string; ok bool }{"", false}
		dlg.Hide()
	})

	content := container.NewBorder(
		widget.NewLabelWithStyle("Select a version to build:", fyne.TextAlignLeading, fyne.TextStyle{Bold: true}),
		container.NewHBox(cancelBtn, buildBtn),
		nil, nil,
		scroll,
	)

	dlg = dialog.NewCustomWithoutButtons(title, content, fyneWin)
	dlg.Resize(fyne.NewSize(800, 600))
	dlg.Show()

	result := <-ch
	return result.val, result.ok
}

// showError shows a non-blocking error dialog.
func showError(msg string) {
	d := dialog.NewError(fmt.Errorf("%s", msg), fyneWin)
	d.Resize(fyne.NewSize(500, 220))
	d.Show()
}

// showInfo shows a blocking info dialog.
func showInfo(title, msg string) {
	ch := make(chan struct{}, 1)
	d := dialog.NewInformation(title, msg, fyneWin)
	d.SetOnClosed(func() { ch <- struct{}{} })
	d.Resize(fyne.NewSize(500, 220))
	d.Show()
	<-ch
}

func main() {
	fyneApp = app.New()
	fyneApp.Settings().SetTheme(theme.DarkTheme())

	fyneWin = fyneApp.NewWindow("REFramework Builder — MH Wilds")
	fyneWin.Resize(fyne.NewSize(750, 480))
	fyneWin.CenterOnScreen()
	fyneWin.SetFixedSize(false)

	// Header
	header := canvas.NewText("REFramework Builder", color.RGBA{R: 0xe5, G: 0x60, B: 0x20, A: 0xff})
	header.TextSize = 22
	header.TextStyle = fyne.TextStyle{Bold: true}
	header.Alignment = fyne.TextAlignCenter

	subtitle := canvas.NewText("Monster Hunter Wilds — noVR Edition", color.RGBA{R: 0x99, G: 0x99, B: 0x99, A: 0xff})
	subtitle.TextSize = 13
	subtitle.Alignment = fyne.TextAlignCenter

	// Status + progress
	statusLabel = widget.NewLabelWithStyle("Starting...", fyne.TextAlignLeading, fyne.TextStyle{})
	progressBar = widget.NewProgressBar()
	progressBar.Min = 0
	progressBar.Max = 1

	// Log area (scrollable)
	logText = widget.NewLabel("")
	logText.Wrapping = fyne.TextWrapWord
	logScroll := container.NewScroll(logText)
	logScroll.SetMinSize(fyne.NewSize(700, 200))

	content := container.NewVBox(
		header,
		subtitle,
		widget.NewSeparator(),
		statusLabel,
		progressBar,
		widget.NewSeparator(),
		logScroll,
	)
	padded := container.NewPadded(content)
	fyneWin.SetContent(padded)

	// Run the build logic in the background
	go runBuild()

	fyneWin.ShowAndRun()
}

func runBuild() {
	defer func() {
		if r := recover(); r != nil {
			showError(fmt.Sprintf("Unexpected error: %v", r))
		}
	}()

	// ── Filters and defaults ──────────────────────────────────────────────────
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
		val, ok := askEntry("REFramework Build Setup",
			"How many recent releases to show?",
			strconv.Itoa(maxList))
		if !ok {
			fyneApp.Quit()
			return
		}
		if n, err := strconv.Atoi(strings.TrimSpace(val)); err == nil && n > 0 {
			maxList = n
		}
	}

	// ── Fetch releases ────────────────────────────────────────────────────────
	setStatus("Fetching recent nightly releases...")
	setProgress(0.1)
	showLog("Contacting GitHub API...")

	os.MkdirAll(cacheDir, 0755)
	etag, _ := os.ReadFile(cacheEtag)
	client := &http.Client{Timeout: 30 * time.Second}
	req, _ := http.NewRequest("GET", repoAPI+"?per_page=100", nil)
	if sEtag := strings.TrimSpace(string(etag)); sEtag != "" {
		req.Header.Set("If-None-Match", sEtag)
	}

	resp, err := client.Do(req)
	if err != nil {
		showError(fmt.Sprintf("Error fetching releases:\n%v", err))
		fyneApp.Quit()
		return
	}
	defer resp.Body.Close()

	var releases []Release
	if resp.StatusCode == http.StatusNotModified {
		f, err := os.Open(cacheBody)
		if err == nil {
			defer f.Close()
			json.NewDecoder(f).Decode(&releases)
			showLog("Using cached release data.")
		}
	} else if resp.StatusCode == http.StatusOK {
		data, err := io.ReadAll(resp.Body)
		if err == nil {
			if json.Unmarshal(data, &releases) == nil {
				os.WriteFile(cacheBody, data, 0644)
				if newEtag := resp.Header.Get("ETag"); newEtag != "" {
					os.WriteFile(cacheEtag, []byte(newEtag), 0644)
				}
				showLog("Fetched fresh release data from GitHub.")
			}
		}
	} else {
		if f, err := os.Open(cacheBody); err == nil {
			defer f.Close()
			json.NewDecoder(f).Decode(&releases)
			showLog(fmt.Sprintf("API returned %d, using cached data.", resp.StatusCode))
		} else {
			showError(fmt.Sprintf("API returned %d and no cache available.", resp.StatusCode))
			fyneApp.Quit()
			return
		}
	}

	re := regexp.MustCompile(`^nightly-(\d{4,})-([A-Za-z0-9]+)$`)
	numMap := make(map[string]Release)
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

	type item struct {
		Num string
		Rel Release
	}
	items := make([]item, 0, len(numMap))
	for k, v := range numMap {
		items = append(items, item{Num: k, Rel: v})
	}
	sort.Slice(items, func(i, j int) bool {
		return items[i].Rel.PublishedAt.After(items[j].Rel.PublishedAt)
	})

	setProgress(0.3)

	if len(items) == 0 {
		showError("Could not find any nightly numeric releases.")
		fyneApp.Quit()
		return
	}

	total := len(items)
	limit := maxList
	if limit > total {
		limit = total
	}
	showLog(fmt.Sprintf("Found %d numeric nightly version(s). Showing %d.", total, limit))

	// ── Version selection ─────────────────────────────────────────────────────
	var choice int
	if silent || maxList == 1 {
		choice = 1
	} else {
		options := make([]string, 0, limit)
		for i := 0; i < limit; i++ {
			it := items[i]
			options = append(options, fmt.Sprintf("%s  (%s)  —  %s",
				it.Num, it.Rel.TagName, it.Rel.PublishedAt.Format("2006-01-02 15:04 UTC")))
		}

		selected, ok := askList("Select Version to Build", options)
		if !ok {
			fyneApp.Quit()
			return
		}
		for i, opt := range options {
			if opt == selected {
				choice = i + 1
				break
			}
		}
		if choice == 0 {
			choice = 1
		}
	}

	sel := items[choice-1]
	tag := sel.Rel.TagName
	pubDate := sel.Rel.PublishedAt

	m2 := re.FindStringSubmatch(tag)
	version := tag
	if len(m2) == 3 {
		shortHash := m2[2]
		if len(shortHash) > 6 {
			shortHash = shortHash[:6]
		}
		version = fmt.Sprintf("nightly-%s-%s", m2[1], shortHash)
	}
	finalZip := fmt.Sprintf("REFramework_%s_%s.zip", version, pubDate.Format("02Jan06"))
	showLog(fmt.Sprintf("Selected: %s → %s", tag, finalZip))

	// ── Check if output exists ────────────────────────────────────────────────
	if _, err := os.Stat(finalZip); err == nil {
		if !silent {
			ok := askConfirm("Archive Exists",
				fmt.Sprintf("%s already exists.\nRebuild it anyway?", finalZip))
			if !ok {
				setStatus("Cancelled.")
				showInfo("Cancelled", "Build cancelled. Archive already exists.")
				fyneApp.Quit()
				return
			}
		}
	}

	// ── Temp workspace ────────────────────────────────────────────────────────
	tmpDir, err := os.MkdirTemp("", "reframework-build-*")
	if err != nil {
		showError(fmt.Sprintf("Error creating temp dir:\n%v", err))
		fyneApp.Quit()
		return
	}
	defer os.RemoveAll(tmpDir)

	stagingZip := filepath.Join(tmpDir, zipName)
	stagingFinal := filepath.Join(tmpDir, finalZip)

	// ── Download ──────────────────────────────────────────────────────────────
	if os.Getenv("SKIP_DOWNLOAD") == "1" {
		showLog("SKIP_DOWNLOAD=1: skipping download.")
		goto finalize
	}

	{
		setStatus(fmt.Sprintf("Downloading %s...", tag))
		setProgress(0.0)
		showLog(fmt.Sprintf("Downloading from GitHub releases (%s)...", tag))

		url := fmt.Sprintf("https://github.com/praydog/REFramework-nightly/releases/download/%s/MHWILDS.zip", tag)
		resp2, err := http.Get(url)
		if err != nil {
			showError(fmt.Sprintf("Error downloading:\n%v", err))
			fyneApp.Quit()
			return
		}
		defer resp2.Body.Close()

		if resp2.StatusCode != http.StatusOK {
			showError(fmt.Sprintf("Download failed: HTTP %s", resp2.Status))
			fyneApp.Quit()
			return
		}

		out, err := os.Create(stagingZip)
		if err != nil {
			showError(fmt.Sprintf("Error creating staging file:\n%v", err))
			fyneApp.Quit()
			return
		}

		pr := &ProgressReader{
			Reader: resp2.Body,
			Total:  resp2.ContentLength,
			OnProgress: func(pct float64) {
				setProgress(pct)
			},
		}
		_, err = io.Copy(out, pr)
		out.Close()

		if err != nil {
			showError(fmt.Sprintf("Error saving download:\n%v", err))
			fyneApp.Quit()
			return
		}
		showLog("Download complete.")
	}

	// ── Transcode ─────────────────────────────────────────────────────────────
	setStatus("Creating optimized archive (removing VR/XR files)...")
	setProgress(0.0)
	showLog("Transcoding: filtering VR/XR files and repacking...")

	if err := transcodeZip(stagingZip, stagingFinal, filters, func(pct float64) {
		setProgress(pct)
	}); err != nil {
		showError(fmt.Sprintf("Error creating archive:\n%v", err))
		fyneApp.Quit()
		return
	}
	showLog("Archive created successfully.")

	// ── Move to working directory ─────────────────────────────────────────────
	if err := copyFile(stagingFinal, finalZip); err != nil {
		showError(fmt.Sprintf("Error saving final archive:\n%v", err))
		fyneApp.Quit()
		return
	}

finalize:
	if _, err := os.Stat(finalZip); err != nil {
		showError(fmt.Sprintf("Critical: Final archive not found!\n%s", finalZip))
		fyneApp.Quit()
		return
	}

	setStatus("Build complete ✓")
	setProgress(1.0)
	showLog(fmt.Sprintf("✓ Done: %s", finalZip))

	// ── Offer to copy to Downloads ────────────────────────────────────────────
	home, err := os.UserHomeDir()
	if err == nil {
		winDownloads := filepath.Join(home, "Downloads")
		if _, err := os.Stat(winDownloads); err == nil {
			dest := filepath.Join(winDownloads, finalZip)
			if silent {
				atomicCopy(finalZip, dest)
				showLog(fmt.Sprintf("Copied to Downloads: %s", finalZip))
			} else {
				ok := askConfirm("Copy to Downloads",
					fmt.Sprintf("Copy %s to your Downloads folder?", finalZip))
				if ok {
					if err := atomicCopy(finalZip, dest); err == nil {
						showLog("✓ Copied to Downloads folder.")
						showInfo("Build Complete", fmt.Sprintf("Successfully built and copied:\n%s", finalZip))
					} else {
						showError(fmt.Sprintf("Error copying to Downloads:\n%v", err))
					}
				} else {
					showInfo("Build Complete", fmt.Sprintf("Build complete!\n%s is in the current directory.", finalZip))
				}
			}
		} else {
			showInfo("Build Complete", fmt.Sprintf("Build complete!\n%s is in the current directory.", finalZip))
		}
	}

	fyneApp.Quit()
}

func atomicCopy(src, dst string) error {
	absSrc, _ := filepath.Abs(src)
	absDst, _ := filepath.Abs(dst)
	if absSrc == absDst {
		return nil
	}
	return copyFile(src, dst)
}

func transcodeZip(src, dest string, filters []string, onProgress func(float64)) error {
	sReader, err := zip.OpenReader(src)
	if err != nil {
		return fmt.Errorf("open source: %w", err)
	}
	defer sReader.Close()

	dFile, err := os.Create(dest)
	if err != nil {
		return fmt.Errorf("create dest: %w", err)
	}
	defer dFile.Close()

	dWriter := zip.NewWriter(dFile)
	defer dWriter.Close()

	_, err = dWriter.Create("MHWILDS/")
	if err != nil {
		return fmt.Errorf("create root dir: %w", err)
	}

	totalFiles := len(sReader.File)
	processedFiles := 0

	for _, f := range sReader.File {
		processedFiles++
		if onProgress != nil {
			onProgress(float64(processedFiles) / float64(totalFiles))
		}

		skip := false
		for _, p := range filters {
			if strings.Contains(f.Name, p) {
				skip = true
				break
			}
		}
		if skip {
			continue
		}

		srcFile, err := f.Open()
		if err != nil {
			return fmt.Errorf("open entry %s: %w", f.Name, err)
		}

		header := &zip.FileHeader{
			Name:     "MHWILDS/" + f.Name,
			Method:   zip.Deflate,
			Modified: f.Modified,
		}
		destFile, err := dWriter.CreateHeader(header)
		if err != nil {
			srcFile.Close()
			return fmt.Errorf("create header %s: %w", f.Name, err)
		}

		_, err = io.Copy(destFile, srcFile)
		srcFile.Close()
		if err != nil {
			return fmt.Errorf("copy entry %s: %w", f.Name, err)
		}
	}

	if err := dWriter.Close(); err != nil {
		return fmt.Errorf("close zip writer: %w", err)
	}

	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}

	return out.Close()
}
