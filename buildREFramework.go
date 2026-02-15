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
	"strings"
	"time"
)

const (
	repoAPI    = "https://api.github.com/repos/praydog/REFramework-nightly/releases"
	zipName    = "MHWILDS.zip"
	extractDir = "MHWILDS"
)

type Release struct {
	TagName string `json:"tag_name"`
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
	// 1. Fetching latest tag
	fmt.Println("==> Fetching latest nightly tag...")
	resp, err := http.Get(repoAPI)
	if err != nil {
		fmt.Printf("Error fetching releases: %v\n", err)
		os.Exit(1)
	}
	defer resp.Body.Close()

	var releases []Release
	if err := json.NewDecoder(resp.Body).Decode(&releases); err != nil {
		fmt.Printf("Error decoding JSON: %v\n", err)
		os.Exit(1)
	}

	var tag string
	for _, r := range releases {
		if strings.HasPrefix(r.TagName, "nightly-") {
			tag = r.TagName
			break
		}
	}

	if tag == "" {
		fmt.Println("Error: Could not find the latest nightly tag.")
		os.Exit(1)
	}

	// 2. Downloading with progress
	url := fmt.Sprintf("https://github.com/praydog/REFramework-nightly/releases/download/%s/MHWILDS.zip", tag)
	fmt.Printf("==> Found tag: %s\n", tag)

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

	// 3. Unzipping
	fmt.Println("==> Extracting MHWILDS.zip...")
	if err := unzip(zipName, extractDir); err != nil {
		fmt.Printf("Error unzipping: %v\n", err)
		os.Exit(1)
	}

	// 4. Cleaning
	fmt.Println("==> Filtering files...")
	filterRegex := regexp.MustCompile(`(?i)RE|vr|DELETE|xr`)
	err = filepath.Walk(extractDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			if os.IsNotExist(err) {
				return nil
			}
			return err
		}
		if path == extractDir {
			return nil
		}
		if filterRegex.MatchString(info.Name()) {
			err := os.RemoveAll(path)
			if err != nil {
				return err
			}
			if info.IsDir() {
				return filepath.SkipDir
			}
		}
		return nil
	})
	if err != nil {
		fmt.Printf("Error filtering files: %v\n", err)
	}

	// 5. Zipping
	hash := ""
	parts := strings.Split(tag, "-")
	if len(parts) >= 3 {
		hash = parts[2]
		if len(hash) > 6 {
			hash = hash[:6]
		}
	} else {
		hash = "unknown"
	}

	finalZip := fmt.Sprintf("REFramework_%s_%s.zip", hash, time.Now().Format("02Jan06"))
	fmt.Printf("==> Creating optimized archive: %s\n", finalZip)
	if err := createZip(finalZip, extractDir); err != nil {
		fmt.Printf("Error creating final zip: %v\n", err)
		os.Exit(1)
	}

	// 6. Final Cleanup
	fmt.Println("==> Cleaning up temporary files...")
	os.Remove(zipName)
	os.RemoveAll(extractDir)

	fmt.Printf("==> Finished! Created: %s\n", finalZip)
}

func unzip(src, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
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
		if info.IsDir() {
			return nil
		}

		relPath, err := filepath.Rel(sourceDir, path)
		if err != nil {
			return err
		}

		zipFile, err := zipWriter.Create(filepath.Join(sourceDir, relPath))
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
