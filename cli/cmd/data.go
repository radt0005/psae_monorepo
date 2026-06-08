package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"
)

// ---- parent command ----

var dataCmd = &cobra.Command{
	Use:   "data",
	Short: "Manage data files in the cloud registry",
	Long:  `Upload, download, and list data files stored in the Spade cloud registry.`,
}

func init() {
	rootCmd.AddCommand(dataCmd)
}

// ---- shared response types ----

type dataFileResponse struct {
	ID          string    `json:"id"`
	OwnerID     string    `json:"ownerId"`
	Name        string    `json:"name"`
	SizeBytes   int64     `json:"sizeBytes"`
	MimeType    string    `json:"mimeType"`
	S3Key       string    `json:"s3Key"`
	Visibility  string    `json:"visibility"`
	CreatedAt   time.Time `json:"createdAt"`
	DownloadURL string    `json:"downloadUrl,omitempty"`
}

// ---- spade data upload ----

var (
	uploadDisplayName string
	uploadVisibility  string
)

var dataUploadCmd = &cobra.Command{
	Use:   "upload <file>",
	Short: "Upload a data file to the cloud registry",
	Long: `Upload a local file to the Spade cloud registry.

The file is transferred directly to object storage via a presigned URL.
A database record is created after the transfer completes.

Examples:
  spade data upload imagery.tif
  spade data upload results.geojson --name "June survey" --visibility shared`,
	Args: cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, err := LoadCredentials()
		if err != nil {
			return err
		}
		return runDataUpload(http.DefaultClient, creds, args[0], uploadDisplayName, uploadVisibility)
	},
}

func init() {
	dataUploadCmd.Flags().StringVar(&uploadDisplayName, "name", "", "Display name (defaults to filename)")
	dataUploadCmd.Flags().StringVar(&uploadVisibility, "visibility", "private", "Visibility: private, shared, or public")
	dataCmd.AddCommand(dataUploadCmd)
}

type initUploadRequest struct {
	Name      string `json:"name"`
	MimeType  string `json:"mimeType"`
	SizeBytes int64  `json:"sizeBytes"`
}

type initUploadResponse struct {
	DataFileID string `json:"dataFileId"`
	S3Key      string `json:"s3Key"`
	UploadURL  string `json:"uploadUrl"`
	ExpiresIn  int    `json:"expiresIn"`
}

type completeUploadRequest struct {
	DataFileID string `json:"dataFileId"`
	S3Key      string `json:"s3Key"`
	Name       string `json:"name"`
	MimeType   string `json:"mimeType"`
	SizeBytes  int64  `json:"sizeBytes"`
	Visibility string `json:"visibility"`
}

func runDataUpload(client *http.Client, creds Credentials, filePath, displayName, visibility string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file: %w", err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("stat: %w", err)
	}

	if displayName == "" {
		displayName = filepath.Base(filePath)
	}
	mimeType := detectMIME(filePath)

	initResp, err := initUpload(client, creds, initUploadRequest{
		Name:      displayName,
		MimeType:  mimeType,
		SizeBytes: info.Size(),
	})
	if err != nil {
		return fmt.Errorf("initializing upload: %w", err)
	}

	fmt.Printf("Uploading %s (%s)...\n", displayName, formatSize(info.Size()))
	pr := &progressReader{r: f, total: info.Size(), label: "Uploading:"}

	req, err := http.NewRequest(http.MethodPut, initResp.UploadURL, pr)
	if err != nil {
		return err
	}
	req.ContentLength = info.Size()
	req.Header.Set("Content-Type", mimeType)

	putResp, err := client.Do(req)
	fmt.Println()
	if err != nil {
		return fmt.Errorf("uploading to storage: %w", err)
	}
	putResp.Body.Close()

	if putResp.StatusCode != http.StatusOK && putResp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("storage returned %d", putResp.StatusCode)
	}

	row, err := completeUpload(client, creds, completeUploadRequest{
		DataFileID: initResp.DataFileID,
		S3Key:      initResp.S3Key,
		Name:       displayName,
		MimeType:   mimeType,
		SizeBytes:  info.Size(),
		Visibility: visibility,
	})
	if err != nil {
		return fmt.Errorf("completing upload: %w", err)
	}

	fmt.Printf("Uploaded. File ID: %s\n", row.ID)
	return nil
}

func initUpload(client *http.Client, creds Credentials, req initUploadRequest) (initUploadResponse, error) {
	b, _ := json.Marshal(req)
	resp, err := authedDo(client, http.MethodPost,
		creds.ServerURL+"/api/uploads/data/init", creds.Token,
		bytes.NewReader(b), "application/json")
	if err != nil {
		return initUploadResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return initUploadResponse{}, fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
	}
	var out initUploadResponse
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

func completeUpload(client *http.Client, creds Credentials, req completeUploadRequest) (dataFileResponse, error) {
	b, _ := json.Marshal(req)
	resp, err := authedDo(client, http.MethodPost,
		creds.ServerURL+"/api/uploads/data/complete", creds.Token,
		bytes.NewReader(b), "application/json")
	if err != nil {
		return dataFileResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return dataFileResponse{}, fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
	}
	var out dataFileResponse
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

// ---- spade data download ----

var dataDownloadCmd = &cobra.Command{
	Use:   "download <id> [output]",
	Short: "Download a data file from the cloud registry",
	Long: `Download a data file by its ID. The file is streamed directly from
object storage via a presigned URL.

If [output] is omitted the file is saved using its registered name in the
current directory.

Examples:
  spade data download 0196e4a1-dead-beef-cafe-000000000001
  spade data download 0196e4a1-dead-beef-cafe-000000000001 local.tif`,
	Args: cobra.RangeArgs(1, 2),
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, err := LoadCredentials()
		if err != nil {
			return err
		}
		outPath := ""
		if len(args) > 1 {
			outPath = args[1]
		}
		return runDataDownload(http.DefaultClient, creds, args[0], outPath)
	},
}

func init() {
	dataCmd.AddCommand(dataDownloadCmd)
}

func runDataDownload(client *http.Client, creds Credentials, id, outPath string) error {
	meta, err := getFileMetadata(client, creds, id)
	if err != nil {
		return fmt.Errorf("fetching file info: %w", err)
	}

	if outPath == "" {
		outPath = meta.Name
	}

	f, err := os.Create(outPath)
	if err != nil {
		return fmt.Errorf("creating output file: %w", err)
	}
	defer f.Close()

	resp, err := client.Get(meta.DownloadURL)
	if err != nil {
		return fmt.Errorf("downloading from storage: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("storage returned %d", resp.StatusCode)
	}

	fmt.Printf("Downloading %s (%s)...\n", meta.Name, formatSize(meta.SizeBytes))
	pr := &progressReader{r: resp.Body, total: meta.SizeBytes, label: "Downloading:"}
	if _, err := io.Copy(f, pr); err != nil {
		fmt.Println()
		return fmt.Errorf("writing file: %w", err)
	}
	fmt.Println()
	fmt.Printf("Saved to %s\n", outPath)
	return nil
}

func getFileMetadata(client *http.Client, creds Credentials, id string) (dataFileResponse, error) {
	resp, err := authedDo(client, http.MethodGet,
		creds.ServerURL+"/api/data/"+id, creds.Token, nil, "")
	if err != nil {
		return dataFileResponse{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return dataFileResponse{}, fmt.Errorf("file %s not found", id)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return dataFileResponse{}, fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
	}
	var out dataFileResponse
	return out, json.NewDecoder(resp.Body).Decode(&out)
}

// ---- spade data list ----

var dataListScope string

var dataListCmd = &cobra.Command{
	Use:   "list",
	Short: "List data files in the cloud registry",
	Long: `List data files in the cloud registry.

--scope controls which files are shown:
  mine    (default) files you own
  shared  files shared with you
  public  all public files`,
	RunE: func(cmd *cobra.Command, args []string) error {
		creds, err := LoadCredentials()
		if err != nil {
			return err
		}
		return runDataList(http.DefaultClient, creds, dataListScope)
	},
}

func init() {
	dataListCmd.Flags().StringVar(&dataListScope, "scope", "mine", "Scope: mine, shared, or public")
	dataCmd.AddCommand(dataListCmd)
}

type dataFileListResponse struct {
	Items []dataFileResponse `json:"items"`
}

func runDataList(client *http.Client, creds Credentials, scope string) error {
	resp, err := authedDo(client, http.MethodGet,
		creds.ServerURL+"/api/data?scope="+scope, creds.Token, nil, "")
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("server returned %d: %s", resp.StatusCode, body)
	}

	var list dataFileListResponse
	if err := json.NewDecoder(resp.Body).Decode(&list); err != nil {
		return err
	}

	if len(list.Items) == 0 {
		fmt.Println("No files found.")
		return nil
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tName\tSize\tVisibility\tCreated")
	for _, file := range list.Items {
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\n",
			file.ID,
			file.Name,
			formatSize(file.SizeBytes),
			file.Visibility,
			file.CreatedAt.Format("2006-01-02 15:04"),
		)
	}
	return w.Flush()
}

// ---- shared helpers ----

// authedDo makes an HTTP request with a Bearer token in the Authorization header.
func authedDo(client *http.Client, method, url, token string, body io.Reader, contentType string) (*http.Response, error) {
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	return client.Do(req)
}

// detectMIME returns the MIME type for a file path, covering common geospatial
// formats that the standard library doesn't know about.
func detectMIME(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".geojson":
		return "application/geo+json"
	case ".gpkg":
		return "application/geopackage+sqlite3"
	case ".shp":
		return "application/x-shapefile"
	case ".kml":
		return "application/vnd.google-earth.kml+xml"
	case ".kmz":
		return "application/vnd.google-earth.kmz"
	}
	if t := mime.TypeByExtension(strings.ToLower(filepath.Ext(path))); t != "" {
		return t
	}
	return "application/octet-stream"
}

// formatSize returns a human-readable byte count (e.g. "142.3 MB").
func formatSize(b int64) string {
	if b < 1024 {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(1024), 0
	for n := b / 1024; n >= 1024; n /= 1024 {
		div *= 1024
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// progressReader wraps an io.Reader and prints a live progress line.
type progressReader struct {
	r         io.Reader
	total     int64
	read      int64
	label     string
	lastPrint time.Time
}

func (p *progressReader) Read(b []byte) (int, error) {
	n, err := p.r.Read(b)
	p.read += int64(n)
	if time.Since(p.lastPrint) >= 100*time.Millisecond {
		if p.total > 0 {
			pct := float64(p.read) / float64(p.total) * 100
			fmt.Printf("\r%s %s / %s (%.0f%%)  ",
				p.label, formatSize(p.read), formatSize(p.total), pct)
		} else {
			fmt.Printf("\r%s %s  ", p.label, formatSize(p.read))
		}
		p.lastPrint = time.Now()
	}
	return n, err
}
