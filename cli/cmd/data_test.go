package cmd

import (
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ---- initUpload ----

func TestInitUpload_Success(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost || r.URL.Path != "/api/uploads/data/init" {
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.Path)
		}
		if r.Header.Get("Authorization") != "Bearer test-token" {
			t.Errorf("missing or wrong Authorization header: %q", r.Header.Get("Authorization"))
		}
		var body initUploadRequest
		json.NewDecoder(r.Body).Decode(&body)
		if body.Name != "file.tif" {
			t.Errorf("Name = %q, want %q", body.Name, "file.tif")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(initUploadResponse{
			DataFileID: "file-id-1",
			S3Key:      "data/user/file-id-1/file.tif",
			UploadURL:  srv.URL + "/s3/upload",
			ExpiresIn:  1800,
		})
	}))
	defer srv.Close()

	creds := Credentials{Token: "test-token", ServerURL: srv.URL}
	resp, err := initUpload(srv.Client(), creds, initUploadRequest{
		Name: "file.tif", MimeType: "image/tiff", SizeBytes: 1024,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.DataFileID != "file-id-1" {
		t.Errorf("DataFileID = %q, want %q", resp.DataFileID, "file-id-1")
	}
}

func TestInitUpload_Unauthorized(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"not authenticated"}`, http.StatusUnauthorized)
	}))
	defer srv.Close()

	creds := Credentials{Token: "bad-token", ServerURL: srv.URL}
	_, err := initUpload(srv.Client(), creds, initUploadRequest{Name: "f.tif", MimeType: "image/tiff", SizeBytes: 1})
	if err == nil {
		t.Fatal("expected error for 401")
	}
	if !strings.Contains(err.Error(), "401") {
		t.Errorf("expected 401 in error, got: %v", err)
	}
}

// ---- completeUpload ----

func TestCompleteUpload_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/uploads/data/complete" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dataFileResponse{
			ID:        "file-id-1",
			Name:      "file.tif",
			SizeBytes: 1024,
			MimeType:  "image/tiff",
		})
	}))
	defer srv.Close()

	creds := Credentials{Token: "test-token", ServerURL: srv.URL}
	row, err := completeUpload(srv.Client(), creds, completeUploadRequest{
		DataFileID: "file-id-1",
		S3Key:      "data/user/file-id-1/file.tif",
		Name:       "file.tif",
		MimeType:   "image/tiff",
		SizeBytes:  1024,
		Visibility: "private",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if row.ID != "file-id-1" {
		t.Errorf("ID = %q, want %q", row.ID, "file-id-1")
	}
}

// ---- getFileMetadata ----

func TestGetFileMetadata_Success(t *testing.T) {
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/data/file-id-1" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dataFileResponse{
			ID:          "file-id-1",
			Name:        "result.tif",
			SizeBytes:   512,
			MimeType:    "image/tiff",
			DownloadURL: srv.URL + "/s3/result.tif",
		})
	}))
	defer srv.Close()

	creds := Credentials{Token: "test-token", ServerURL: srv.URL}
	meta, err := getFileMetadata(srv.Client(), creds, "file-id-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if meta.Name != "result.tif" {
		t.Errorf("Name = %q, want %q", meta.Name, "result.tif")
	}
	if meta.DownloadURL == "" {
		t.Error("expected non-empty DownloadURL")
	}
}

func TestGetFileMetadata_NotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"not found"}`, http.StatusNotFound)
	}))
	defer srv.Close()

	creds := Credentials{Token: "test-token", ServerURL: srv.URL}
	_, err := getFileMetadata(srv.Client(), creds, "missing-id")
	if err == nil {
		t.Fatal("expected error for 404")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Errorf("expected 'not found' in error, got: %v", err)
	}
}

// ---- runDataList ----

func TestRunDataList_WithItems(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/data" {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if r.URL.Query().Get("scope") != "mine" {
			t.Errorf("scope = %q, want %q", r.URL.Query().Get("scope"), "mine")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dataFileListResponse{
			Items: []dataFileResponse{
				{ID: "id-1", Name: "a.tif", SizeBytes: 1024, Visibility: "private", CreatedAt: time.Now()},
				{ID: "id-2", Name: "b.gpkg", SizeBytes: 2048, Visibility: "shared", CreatedAt: time.Now()},
			},
		})
	}))
	defer srv.Close()

	creds := Credentials{Token: "test-token", ServerURL: srv.URL}
	if err := runDataList(srv.Client(), creds, "mine"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunDataList_Empty(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(dataFileListResponse{Items: []dataFileResponse{}})
	}))
	defer srv.Close()

	creds := Credentials{Token: "test-token", ServerURL: srv.URL}
	if err := runDataList(srv.Client(), creds, "mine"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// ---- runDataUpload (end-to-end with mock servers) ----

func TestRunDataUpload_Success(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SPADE_DIR", dir)

	// Create a small test file
	filePath := filepath.Join(dir, "test.tif")
	content := []byte("fake tiff content")
	if err := os.WriteFile(filePath, content, 0644); err != nil {
		t.Fatal(err)
	}

	var gotUploadBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/uploads/data/init":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(initUploadResponse{
				DataFileID: "file-id-1",
				S3Key:      "data/user/file-id-1/test.tif",
				UploadURL:  "http://" + r.Host + "/s3/put",
				ExpiresIn:  1800,
			})
		case "/s3/put":
			gotUploadBody, _ = io.ReadAll(r.Body)
			w.WriteHeader(http.StatusNoContent)
		case "/api/uploads/data/complete":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(dataFileResponse{
				ID: "file-id-1", Name: "test.tif", SizeBytes: int64(len(content)),
			})
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	creds := Credentials{Token: "test-token", ServerURL: srv.URL}
	if err := runDataUpload(srv.Client(), creds, filePath, "", "private"); err != nil {
		t.Fatalf("runDataUpload: %v", err)
	}
	if string(gotUploadBody) != string(content) {
		t.Errorf("uploaded bytes = %q, want %q", gotUploadBody, content)
	}
}

// ---- runDataDownload (end-to-end with mock servers) ----

func TestRunDataDownload_Success(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("SPADE_DIR", dir)

	fileContent := []byte("downloaded content")
	outPath := filepath.Join(dir, "out.tif")

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/data/file-id-1":
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(dataFileResponse{
				ID:          "file-id-1",
				Name:        "result.tif",
				SizeBytes:   int64(len(fileContent)),
				DownloadURL: "http://" + r.Host + "/s3/get",
			})
		case "/s3/get":
			w.Write(fileContent)
		default:
			t.Errorf("unexpected path: %s", r.URL.Path)
			http.Error(w, "not found", http.StatusNotFound)
		}
	}))
	defer srv.Close()

	creds := Credentials{Token: "test-token", ServerURL: srv.URL}
	if err := runDataDownload(srv.Client(), creds, "file-id-1", outPath); err != nil {
		t.Fatalf("runDataDownload: %v", err)
	}

	got, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("reading output file: %v", err)
	}
	if string(got) != string(fileContent) {
		t.Errorf("downloaded bytes = %q, want %q", got, fileContent)
	}
}

// ---- detectMIME ----

func TestDetectMIME(t *testing.T) {
	cases := []struct {
		path string
		want string
	}{
		{"data.geojson", "application/geo+json"},
		{"tiles.gpkg", "application/geopackage+sqlite3"},
		{"boundaries.shp", "application/x-shapefile"},
		{"view.kml", "application/vnd.google-earth.kml+xml"},
		{"archive.kmz", "application/vnd.google-earth.kmz"},
		{"image.tif", "image/tiff"},
		{"image.tiff", "image/tiff"},
		{"archive.zip", "application/zip"},
		{"unknown.xyz", "application/octet-stream"},
	}
	for _, c := range cases {
		got := detectMIME(c.path)
		if got != c.want {
			t.Errorf("detectMIME(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}

// ---- formatSize ----

func TestFormatSize(t *testing.T) {
	cases := []struct {
		bytes int64
		want  string
	}{
		{0, "0 B"},
		{512, "512 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1536, "1.5 KB"},
		{1024 * 1024, "1.0 MB"},
		{149212979, "142.3 MB"}, // 142.3 * 1024 * 1024
		{1024 * 1024 * 1024, "1.0 GB"},
	}
	for _, c := range cases {
		got := formatSize(c.bytes)
		if got != c.want {
			t.Errorf("formatSize(%d) = %q, want %q", c.bytes, got, c.want)
		}
	}
}
