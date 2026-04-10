// Package cocoonpaths describes cocoon's on-disk storage layout (snapshot
// DB, snapshot data dir, cloudimg blob dir) so callers that need to peek
// at the cocoon root without going through the cocoon CLI can use a
// shared, vetted set of types.
//
// This package replaces the old github.com/cocoonstack/epoch/cocoon
// package: epoch became a vendor-agnostic OCI registry and dropped any
// direct knowledge of cocoon's filesystem layout, so the types live here
// in cocoon-common where every cocoonstack consumer (vk-cocoon today,
// cocoon-operator tomorrow, ...) can import them without depending on
// epoch.
//
// Only the small subset of cocoon's storage tree that callers actually
// need is mirrored. Push and pull flows no longer touch this package —
// they go through cocoon CLI pipes (cocoon snapshot export -o - /
// cocoon snapshot import) so cocoon stays the single source of truth
// for the on-disk format.
package cocoonpaths

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// DefaultRootDir is the conventional cocoon data root.
const DefaultRootDir = "/var/lib/cocoon"

// SnapshotDB matches Cocoon's snapshots.json on-disk format.
type SnapshotDB struct {
	Snapshots map[string]*SnapshotRecord `json:"snapshots"`
	Names     map[string]string          `json:"names"`
}

// SnapshotRecord matches Cocoon's snapshot DB record.
type SnapshotRecord struct {
	ID           string              `json:"id"`
	Name         string              `json:"name"`
	Description  string              `json:"description,omitempty"`
	Image        string              `json:"image,omitempty"`
	ImageBlobIDs map[string]struct{} `json:"image_blob_ids,omitempty"`
	CPU          int                 `json:"cpu,omitempty"`
	Memory       int64               `json:"memory,omitempty"`
	Storage      int64               `json:"storage,omitempty"`
	NICs         int                 `json:"nics,omitempty"`
	CreatedAt    time.Time           `json:"created_at"`
	Pending      bool                `json:"pending,omitempty"`
	DataDir      string              `json:"data_dir,omitempty"`
}

// Paths gives the well-known cocoon storage subdirectories anchored at a
// configurable root.
type Paths struct {
	RootDir string
}

// NewPaths returns a Paths anchored at rootDir, defaulting to DefaultRootDir
// when empty.
func NewPaths(rootDir string) *Paths {
	if rootDir == "" {
		rootDir = DefaultRootDir
	}
	return &Paths{RootDir: rootDir}
}

// SnapshotDBFile returns the path to cocoon's snapshots.json.
func (p *Paths) SnapshotDBFile() string {
	return filepath.Join(p.RootDir, "snapshot", "db", "snapshots.json")
}

// SnapshotDataDir returns the directory holding a single snapshot's blobs.
func (p *Paths) SnapshotDataDir(id string) string {
	return filepath.Join(p.RootDir, "snapshot", "localfile", id)
}

// CloudimgBlobDir returns the directory holding cloud image base blobs.
func (p *Paths) CloudimgBlobDir() string {
	return filepath.Join(p.RootDir, "cloudimg", "blobs")
}

// ReadSnapshotDB reads cocoon's snapshots.json. A missing file returns an
// empty DB so callers can treat first-run as a clean slate.
func (p *Paths) ReadSnapshotDB() (*SnapshotDB, error) {
	data, err := os.ReadFile(p.SnapshotDBFile()) //nolint:gosec // path comes from trusted config
	if err != nil {
		if os.IsNotExist(err) {
			return &SnapshotDB{
				Snapshots: make(map[string]*SnapshotRecord),
				Names:     make(map[string]string),
			}, nil
		}
		return nil, err
	}
	var db SnapshotDB
	if err := json.Unmarshal(data, &db); err != nil {
		return nil, fmt.Errorf("unmarshal snapshot db: %w", err)
	}
	if db.Snapshots == nil {
		db.Snapshots = make(map[string]*SnapshotRecord)
	}
	if db.Names == nil {
		db.Names = make(map[string]string)
	}
	return &db, nil
}

// WriteSnapshotDB writes cocoon's snapshots.json atomically. Used by tests
// that need to seed a fake cocoon storage tree.
func (p *Paths) WriteSnapshotDB(db *SnapshotDB) error {
	data, err := json.MarshalIndent(db, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal snapshot db: %w", err)
	}
	dir := filepath.Dir(p.SnapshotDBFile())
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	tmp := p.SnapshotDBFile() + ".tmp"
	if err := os.WriteFile(tmp, data, 0o600); err != nil {
		return err
	}
	return os.Rename(tmp, p.SnapshotDBFile())
}

// ResolveSnapshotID resolves a snapshot name to its ID via the names index.
// If the input is already an ID, it is returned unchanged.
func (p *Paths) ResolveSnapshotID(name string) (string, error) {
	db, err := p.ReadSnapshotDB()
	if err != nil {
		return "", err
	}
	if id, ok := db.Names[name]; ok {
		return id, nil
	}
	if _, ok := db.Snapshots[name]; ok {
		return name, nil
	}
	return "", fmt.Errorf("snapshot %q not found in cocoon DB", name)
}

// SnapshotExists reports whether a snapshot name is present in the DB.
func SnapshotExists(paths *Paths, name string) bool {
	db, err := paths.ReadSnapshotDB()
	if err != nil {
		return false
	}
	_, ok := db.Names[name]
	return ok
}
