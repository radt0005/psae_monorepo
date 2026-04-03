package spade

import (
	"io/fs"
	"os"
	"path/filepath"
)

// ManifestInfo holds metadata returned by SpadeType.ManifestEntry().
type ManifestInfo struct {
	TypeName string
	Format   string // empty string = not set
	ItemType string // empty string = not set
}

// SpadeType provides metadata about a Spade type for manifest generation and output naming.
type SpadeType interface {
	TypeName() string
	DefaultOutputName() string
	ManifestEntry() ManifestInfo
}

// FromInput constructs a typed value from raw filesystem input data.
type FromInput interface {
	FromSingleFile(path string) error
	FromMultipleFiles(paths []string) error
	FromDirectory(path string) error
}

// IntoOutput writes a typed value to an output subdirectory.
type IntoOutput interface {
	WriteTo(outputDir string) error
	DefaultOutputName() string
}

// copyFile copies a single file from src to dst.
func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0644)
}

// copyDirRecursive recursively copies directory contents from src into dst.
func copyDirRecursive(src, dst string) error {
	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())
		if entry.IsDir() {
			if err := os.MkdirAll(dstPath, 0755); err != nil {
				return err
			}
			if err := copyDirRecursive(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}
	return nil
}

// writeFileOutput is shared logic for single-file types' WriteTo.
func writeFileOutput(path, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	filename := filepath.Base(path)
	return copyFile(path, filepath.Join(outputDir, filename))
}

// writeCollectionOutput is shared logic for collection types' WriteTo.
func writeCollectionOutput(paths []string, outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	for _, p := range paths {
		filename := filepath.Base(p)
		if err := copyFile(p, filepath.Join(outputDir, filename)); err != nil {
			return err
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// File
// ---------------------------------------------------------------------------

// File represents a generic single-file input or output.
type File struct {
	Path string
}

func NewFile(path string) File                    { return File{Path: path} }
func (f *File) TypeName() string                  { return "file" }
func (f *File) DefaultOutputName() string         { return "file" }
func (f *File) ManifestEntry() ManifestInfo       { return ManifestInfo{TypeName: "file"} }
func (f *File) WriteTo(outputDir string) error    { return writeFileOutput(f.Path, outputDir) }
func (f *File) FromSingleFile(path string) error  { f.Path = path; return nil }
func (f *File) FromMultipleFiles(paths []string) error {
	f.Path = paths[0]
	return nil
}
func (f *File) FromDirectory(_ string) error {
	return &ErrTypeMismatch{Expected: "file", Found: "directory"}
}

// ---------------------------------------------------------------------------
// RasterFile
// ---------------------------------------------------------------------------

// RasterFile represents a raster data file (e.g., GeoTIFF).
type RasterFile struct {
	Path string
}

func NewRasterFile(path string) RasterFile          { return RasterFile{Path: path} }
func (f *RasterFile) TypeName() string              { return "file" }
func (f *RasterFile) DefaultOutputName() string     { return "raster" }
func (f *RasterFile) ManifestEntry() ManifestInfo {
	return ManifestInfo{TypeName: "file", Format: "GeoTIFF"}
}
func (f *RasterFile) WriteTo(outputDir string) error    { return writeFileOutput(f.Path, outputDir) }
func (f *RasterFile) FromSingleFile(path string) error  { f.Path = path; return nil }
func (f *RasterFile) FromMultipleFiles(paths []string) error {
	f.Path = paths[0]
	return nil
}
func (f *RasterFile) FromDirectory(_ string) error {
	return &ErrTypeMismatch{Expected: "file", Found: "directory"}
}

// ---------------------------------------------------------------------------
// VectorFile
// ---------------------------------------------------------------------------

// VectorFile represents a vector data file (e.g., GeoJSON).
type VectorFile struct {
	Path string
}

func NewVectorFile(path string) VectorFile          { return VectorFile{Path: path} }
func (f *VectorFile) TypeName() string              { return "file" }
func (f *VectorFile) DefaultOutputName() string     { return "vector" }
func (f *VectorFile) ManifestEntry() ManifestInfo {
	return ManifestInfo{TypeName: "file", Format: "GeoJSON"}
}
func (f *VectorFile) WriteTo(outputDir string) error    { return writeFileOutput(f.Path, outputDir) }
func (f *VectorFile) FromSingleFile(path string) error  { f.Path = path; return nil }
func (f *VectorFile) FromMultipleFiles(paths []string) error {
	f.Path = paths[0]
	return nil
}
func (f *VectorFile) FromDirectory(_ string) error {
	return &ErrTypeMismatch{Expected: "file", Found: "directory"}
}

// ---------------------------------------------------------------------------
// TabularFile
// ---------------------------------------------------------------------------

// TabularFile represents a tabular data file (e.g., CSV).
type TabularFile struct {
	Path string
}

func NewTabularFile(path string) TabularFile          { return TabularFile{Path: path} }
func (f *TabularFile) TypeName() string               { return "file" }
func (f *TabularFile) DefaultOutputName() string      { return "tabular" }
func (f *TabularFile) ManifestEntry() ManifestInfo {
	return ManifestInfo{TypeName: "file", Format: "CSV"}
}
func (f *TabularFile) WriteTo(outputDir string) error    { return writeFileOutput(f.Path, outputDir) }
func (f *TabularFile) FromSingleFile(path string) error  { f.Path = path; return nil }
func (f *TabularFile) FromMultipleFiles(paths []string) error {
	f.Path = paths[0]
	return nil
}
func (f *TabularFile) FromDirectory(_ string) error {
	return &ErrTypeMismatch{Expected: "file", Found: "directory"}
}

// ---------------------------------------------------------------------------
// JsonFile
// ---------------------------------------------------------------------------

// JsonFile represents a JSON data file.
type JsonFile struct {
	Path string
}

func NewJsonFile(path string) JsonFile            { return JsonFile{Path: path} }
func (f *JsonFile) TypeName() string              { return "json" }
func (f *JsonFile) DefaultOutputName() string     { return "json" }
func (f *JsonFile) ManifestEntry() ManifestInfo   { return ManifestInfo{TypeName: "json"} }
func (f *JsonFile) WriteTo(outputDir string) error    { return writeFileOutput(f.Path, outputDir) }
func (f *JsonFile) FromSingleFile(path string) error  { f.Path = path; return nil }
func (f *JsonFile) FromMultipleFiles(paths []string) error {
	f.Path = paths[0]
	return nil
}
func (f *JsonFile) FromDirectory(_ string) error {
	return &ErrTypeMismatch{Expected: "json", Found: "directory"}
}

// ---------------------------------------------------------------------------
// Directory
// ---------------------------------------------------------------------------

// Directory represents a directory-based input or output.
type Directory struct {
	Path string
}

func NewDirectory(path string) Directory          { return Directory{Path: path} }
func (d *Directory) TypeName() string             { return "directory" }
func (d *Directory) DefaultOutputName() string    { return "directory" }
func (d *Directory) ManifestEntry() ManifestInfo  { return ManifestInfo{TypeName: "directory"} }
func (d *Directory) FromSingleFile(_ string) error {
	return &ErrTypeMismatch{Expected: "directory", Found: "file"}
}
func (d *Directory) FromMultipleFiles(_ []string) error {
	return &ErrTypeMismatch{Expected: "directory", Found: "file"}
}
func (d *Directory) FromDirectory(path string) error { d.Path = path; return nil }
func (d *Directory) WriteTo(outputDir string) error {
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return err
	}
	return copyDirRecursive(d.Path, outputDir)
}

// Ensure Directory satisfies fs.FileInfo expectations indirectly — this is just
// for compile-time interface checks.
var _ fs.FileInfo = nil

// ---------------------------------------------------------------------------
// FileCollection
// ---------------------------------------------------------------------------

// FileCollection represents a collection of generic files.
type FileCollection struct {
	Paths []string
}

func NewFileCollection(paths []string) FileCollection { return FileCollection{Paths: paths} }
func (c *FileCollection) TypeName() string            { return "collection" }
func (c *FileCollection) DefaultOutputName() string   { return "files" }
func (c *FileCollection) ManifestEntry() ManifestInfo {
	return ManifestInfo{TypeName: "collection", ItemType: "file"}
}
func (c *FileCollection) WriteTo(outputDir string) error {
	return writeCollectionOutput(c.Paths, outputDir)
}
func (c *FileCollection) FromSingleFile(path string) error {
	c.Paths = []string{path}
	return nil
}
func (c *FileCollection) FromMultipleFiles(paths []string) error {
	c.Paths = paths
	return nil
}
func (c *FileCollection) FromDirectory(_ string) error {
	return &ErrTypeMismatch{Expected: "collection", Found: "directory"}
}

// ---------------------------------------------------------------------------
// RasterFileCollection
// ---------------------------------------------------------------------------

// RasterFileCollection represents a collection of raster data files.
type RasterFileCollection struct {
	Paths []string
}

func NewRasterFileCollection(paths []string) RasterFileCollection {
	return RasterFileCollection{Paths: paths}
}
func (c *RasterFileCollection) TypeName() string          { return "collection" }
func (c *RasterFileCollection) DefaultOutputName() string { return "rasters" }
func (c *RasterFileCollection) ManifestEntry() ManifestInfo {
	return ManifestInfo{TypeName: "collection", Format: "GeoTIFF", ItemType: "file"}
}
func (c *RasterFileCollection) WriteTo(outputDir string) error {
	return writeCollectionOutput(c.Paths, outputDir)
}
func (c *RasterFileCollection) FromSingleFile(path string) error {
	c.Paths = []string{path}
	return nil
}
func (c *RasterFileCollection) FromMultipleFiles(paths []string) error {
	c.Paths = paths
	return nil
}
func (c *RasterFileCollection) FromDirectory(_ string) error {
	return &ErrTypeMismatch{Expected: "collection", Found: "directory"}
}

// ---------------------------------------------------------------------------
// VectorFileCollection
// ---------------------------------------------------------------------------

// VectorFileCollection represents a collection of vector data files.
type VectorFileCollection struct {
	Paths []string
}

func NewVectorFileCollection(paths []string) VectorFileCollection {
	return VectorFileCollection{Paths: paths}
}
func (c *VectorFileCollection) TypeName() string          { return "collection" }
func (c *VectorFileCollection) DefaultOutputName() string { return "vectors" }
func (c *VectorFileCollection) ManifestEntry() ManifestInfo {
	return ManifestInfo{TypeName: "collection", Format: "GeoJSON", ItemType: "file"}
}
func (c *VectorFileCollection) WriteTo(outputDir string) error {
	return writeCollectionOutput(c.Paths, outputDir)
}
func (c *VectorFileCollection) FromSingleFile(path string) error {
	c.Paths = []string{path}
	return nil
}
func (c *VectorFileCollection) FromMultipleFiles(paths []string) error {
	c.Paths = paths
	return nil
}
func (c *VectorFileCollection) FromDirectory(_ string) error {
	return &ErrTypeMismatch{Expected: "collection", Found: "directory"}
}

// ---------------------------------------------------------------------------
// TabularFileCollection
// ---------------------------------------------------------------------------

// TabularFileCollection represents a collection of tabular data files.
type TabularFileCollection struct {
	Paths []string
}

func NewTabularFileCollection(paths []string) TabularFileCollection {
	return TabularFileCollection{Paths: paths}
}
func (c *TabularFileCollection) TypeName() string          { return "collection" }
func (c *TabularFileCollection) DefaultOutputName() string { return "tables" }
func (c *TabularFileCollection) ManifestEntry() ManifestInfo {
	return ManifestInfo{TypeName: "collection", Format: "CSV", ItemType: "file"}
}
func (c *TabularFileCollection) WriteTo(outputDir string) error {
	return writeCollectionOutput(c.Paths, outputDir)
}
func (c *TabularFileCollection) FromSingleFile(path string) error {
	c.Paths = []string{path}
	return nil
}
func (c *TabularFileCollection) FromMultipleFiles(paths []string) error {
	c.Paths = paths
	return nil
}
func (c *TabularFileCollection) FromDirectory(_ string) error {
	return &ErrTypeMismatch{Expected: "collection", Found: "directory"}
}

// ---------------------------------------------------------------------------
// Scalar manifest-only types (for ManifestBuilder)
// ---------------------------------------------------------------------------

// StringType is a manifest-only type representing a string parameter.
type StringType struct{}

func (StringType) TypeName() string            { return "string" }
func (StringType) DefaultOutputName() string   { return "string" }
func (StringType) ManifestEntry() ManifestInfo { return ManifestInfo{TypeName: "string"} }

// NumberType is a manifest-only type representing a numeric parameter.
type NumberType struct{}

func (NumberType) TypeName() string            { return "number" }
func (NumberType) DefaultOutputName() string   { return "number" }
func (NumberType) ManifestEntry() ManifestInfo { return ManifestInfo{TypeName: "number"} }

// BoolType is a manifest-only type representing a boolean parameter.
type BoolType struct{}

func (BoolType) TypeName() string            { return "boolean" }
func (BoolType) DefaultOutputName() string   { return "boolean" }
func (BoolType) ManifestEntry() ManifestInfo { return ManifestInfo{TypeName: "boolean"} }
