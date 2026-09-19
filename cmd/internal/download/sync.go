package download

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

type Meta struct {
	URL           string    `json:"url"`
	ETag          string    `json:"etag,omitempty"`
	LastModified  string    `json:"last_modified,omitempty"`
	ContentLength int64     `json:"content_length,omitempty"`
	SHA256        string    `json:"sha256"`
	DownloadedAt  time.Time `json:"downloaded_at"`
}

func MetaPath(dbPath string) string {
	return dbPath + ".meta.json"
}

func LoadMeta(path string) (*Meta, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Meta
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

func SaveMeta(path string, m *Meta) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

type SyncResult struct {
	Skipped   bool
	Path      string
	SHA256    string
	Bytes     int64
	Meta      Meta
}

func Sync(ctx context.Context, url, dbPath string, force bool) (SyncResult, error) {
	var res SyncResult
	res.Path = dbPath

	if err := os.MkdirAll(filepath.Dir(dbPath), 0o755); err != nil {
		return res, fmt.Errorf("mkdir cache: %w", err)
	}

	headReq, err := http.NewRequestWithContext(ctx, http.MethodHead, url, nil)
	if err != nil {
		return res, err
	}
	headResp, err := http.DefaultClient.Do(headReq)
	if err != nil {
		return res, fmt.Errorf("head request: %w", err)
	}
	headResp.Body.Close()

	etag := headResp.Header.Get("ETag")
	lastMod := headResp.Header.Get("Last-Modified")
	contentLen := headResp.ContentLength

	metaPath := MetaPath(dbPath)
	prevMeta, _ := LoadMeta(metaPath)

	if !force && prevMeta != nil {
		if st, err := os.Stat(dbPath); err == nil && st.Size() > 0 {
			localHash, err := fileSHA256(dbPath)
			if err == nil && localHash == prevMeta.SHA256 {
				if etag == "" || etag == prevMeta.ETag {
					res.Skipped = true
					res.SHA256 = localHash
					res.Bytes = st.Size()
					res.Meta = *prevMeta
					return res, nil
				}
				if lastMod != "" && lastMod == prevMeta.LastModified {
					res.Skipped = true
					res.SHA256 = localHash
					res.Bytes = st.Size()
					res.Meta = *prevMeta
					return res, nil
				}
			}
		}
	}

	partPath := dbPath + ".part"
	if err := downloadFile(ctx, url, partPath, contentLen); err != nil {
		return res, err
	}

	hash, err := fileSHA256(partPath)
	if err != nil {
		_ = os.Remove(partPath)
		return res, err
	}

	if err := os.Rename(partPath, dbPath); err != nil {
		return res, fmt.Errorf("rename db: %w", err)
	}

	st, err := os.Stat(dbPath)
	if err != nil {
		return res, err
	}

	m := Meta{
		URL:           url,
		ETag:          etag,
		LastModified:  lastMod,
		ContentLength: st.Size(),
		SHA256:        hash,
		DownloadedAt:  time.Now().UTC(),
	}
	if err := SaveMeta(metaPath, &m); err != nil {
		return res, err
	}

	res.SHA256 = hash
	res.Bytes = st.Size()
	res.Meta = m
	return res, nil
}

func downloadFile(ctx context.Context, url, dest string, expectedLen int64) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}

	var existing int64
	if st, err := os.Stat(dest); err == nil {
		existing = st.Size()
		if existing > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existing))
		}
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("get: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusRequestedRangeNotSatisfiable {
		_ = os.Remove(dest)
		return downloadFile(ctx, url, dest, expectedLen)
	}
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	flags := os.O_CREATE | os.O_WRONLY
	if resp.StatusCode == http.StatusPartialContent {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}

	f, err := os.OpenFile(dest, flags, 0o644)
	if err != nil {
		return err
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return fmt.Errorf("write: %w", err)
	}
	return nil
}

func DBSHA256(dbPath string) (string, error) {
	return fileSHA256(dbPath)
}
