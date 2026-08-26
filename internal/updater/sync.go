package updater

import (
	"strings"
	"archive/zip"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	"faangjobs/internal/httpapi"
	"faangjobs/internal/store"
)

// SyncData downloads the zip release, extracts to a temp directory,
// atomically swaps targetDir, transitions store to disk, and reloads the index.
func SyncData(downloadURL, targetDir string, idx *httpapi.Index, st *store.Store) error {
	tmpZip := targetDir + "_download.zip"
	tmpExtractDir := targetDir + "_tmp"

	log.Printf("[Sync] Downloading latest release from %s...", downloadURL)
	resp, err := http.Get(downloadURL)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download failed with status: %s", resp.Status)
	}

	out, err := os.Create(tmpZip)
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	_, err = io.Copy(out, resp.Body)
	out.Close()
	if err != nil {
		os.Remove(tmpZip)
		return fmt.Errorf("failed to save zip: %w", err)
	}
	defer os.Remove(tmpZip)

	os.RemoveAll(tmpExtractDir)
	if err := unzip(tmpZip, tmpExtractDir); err != nil {
		os.RemoveAll(tmpExtractDir)
		return fmt.Errorf("unzip failed: %w", err)
	}

	oldDir := targetDir + "_old"
	os.RemoveAll(oldDir)

	if err := os.Rename(targetDir, oldDir); err != nil && !os.IsNotExist(err) {
		os.RemoveAll(tmpExtractDir)
		return fmt.Errorf("failed to backup current target dir: %w", err)
	}

	if err := os.Rename(tmpExtractDir, targetDir); err != nil {
		_ = os.Rename(oldDir, targetDir) // Rollback if swap fails
		return fmt.Errorf("failed to swap new target dir: %w", err)
	}
	os.RemoveAll(oldDir)
	log.Println("[Sync] Disk update completed successfully.")

	if st != nil && st.IsEmbedded() {
		log.Println("[Sync] Transitioning store from embedded fallback mode to fresh disk mode...")
		if err := st.SwitchToDisk(targetDir); err != nil {
			log.Printf("[Sync] Warning: failed to switch store to disk mode: %v", err)
		}
	}

	log.Println("[Sync] Rebuilding in-memory index snapshot...")
	idx.Reload()
	log.Println("[Sync] Index reload complete!")

	return nil
}

// Helper to safely extract zip entries
func unzip(src string, dest string) error {
	r, err := zip.OpenReader(src)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		cleanName := strings.TrimPrefix(f.Name, "data/")
		fpath := filepath.Join(dest, cleanName)

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
		rc.Close()
		outFile.Close()

		if err != nil {
			return err
		}
	}
	return nil
}
