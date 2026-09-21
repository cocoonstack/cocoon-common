package snapshot

import (
	"archive/tar"
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"

	"github.com/cocoonstack/cocoon-common/manifest"
)

const engineEnvelope = `{
  "config": {
    "cpu": 4,
    "memory": 8589934592,
    "storage": 21474836480,
    "queue_size": 512,
    "disk_queue_size": 256,
    "image": "reg/base:v1",
    "image_digest": "sha256:aa",
    "image_type": "oci",
    "network": "cocoon0",
    "no_direct_io": true,
    "no_watchdog": true,
    "no_balloon": true,
    "windows": true,
    "shared_memory": true,
    "hugepages": true,
    "mergeable": true,
    "pci": true,
    "id": "snap-1",
    "name": "vm-a",
    "description": "d",
    "image_blob_ids": {"deadbeef": {}},
    "hypervisor": "firecracker",
    "nics": 2,
    "nic_mtus": [9000, 9000]
  },
  "version": 1
}`

func TestRoundTripKeepsEveryEngineConfigField(t *testing.T) {
	uploader := newFakeUploader()
	pushCorpus(t, uploader, engineExportTar(t), PushOptions{})
	got := readTarEntries(t, bytes.NewReader(pullTar(t, uploader, StreamOptions{LocalName: "vm-b"})))
	want := decodeEnvelope(t, []byte(engineEnvelope))
	want["name"] = "vm-b"
	if envelope := decodeEnvelope(t, got[0].body); !reflect.DeepEqual(envelope, want) {
		t.Errorf("round-trip envelope config = %v, want %v", envelope, want)
	}
	raw, _, err := uploader.GetManifest(t.Context(), "myvm", "v2test")
	if err != nil {
		t.Fatal(err)
	}
	var m manifest.OCIManifest
	if err := json.Unmarshal(raw, &m); err != nil {
		t.Fatal(err)
	}
	var cfg manifest.SnapshotConfig
	if err := json.Unmarshal(uploader.blobs[m.Config.Digest], &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.SnapshotID != "snap-1" || cfg.Hypervisor != "firecracker" || cfg.CPU != 4 || !cfg.Windows {
		t.Errorf("typed config fields not indexed: %+v", cfg)
	}
}

func TestPushRejectsANonObjectEngineConfig(t *testing.T) {
	for _, envelope := range []string{`{"config":null,"version":1}`, `{"version":1}`} {
		pusher := &Pusher{Uploader: newFakeUploader(), Cocoon: &fakeCocoon{exportTar: exportTarWithEnvelope(t, envelope)}}
		err := pusher.Push(t.Context(), PushOptions{Name: "myvm", Tag: "v2test"})
		if err == nil || !strings.Contains(err.Error(), "config must be a JSON object") {
			t.Errorf("Push(%s) err = %v, want the object check", envelope, err)
		}
	}
}

func TestMarshalEnvelopeWithoutEngineConfigUsesTypedFields(t *testing.T) {
	data, err := MarshalEnvelope(&manifest.SnapshotConfig{SnapshotID: "snap-2", CPU: 3}, "vm-c")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"id": "snap-2", "name": "vm-c", "cpu": json.Number("3")}
	if got := decodeEnvelope(t, data); !reflect.DeepEqual(got, want) {
		t.Errorf("envelope config = %v, want %v", got, want)
	}
}

func decodeEnvelope(t *testing.T, data []byte) map[string]any {
	t.Helper()
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var envelope struct {
		Version int            `json:"version"`
		Config  map[string]any `json:"config"`
	}
	if err := dec.Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	if envelope.Version != 1 {
		t.Fatalf("envelope version = %d", envelope.Version)
	}
	return envelope.Config
}

func engineExportTar(t *testing.T) []byte {
	t.Helper()
	return exportTarWithEnvelope(t, engineEnvelope)
}

func exportTarWithEnvelope(t *testing.T, envelope string) []byte {
	t.Helper()
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for _, entry := range []struct {
		name string
		data []byte
	}{
		{snapshotJSONName, []byte(envelope)},
		{"config.json", bytes.Repeat([]byte("c"), 64)},
		{"memory-ranges", bytes.Repeat([]byte("m"), 4096)},
	} {
		if err := tw.WriteHeader(&tar.Header{Name: entry.name, Size: int64(len(entry.data)), Mode: 0o640}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(entry.data); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
