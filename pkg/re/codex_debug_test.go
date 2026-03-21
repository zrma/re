package re

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCodexExecDebugBundleWritesStructuredFiles(t *testing.T) {
	baseDir := t.TempDir()

	bundle, err := newCodexExecDebugBundle(baseDir, "gpt-5.4-mini")
	if err != nil {
		t.Fatalf("newCodexExecDebugBundle() error = %v", err)
	}

	if err := bundle.WriteRequest([]byte(`{"input":true}`), "prompt body", []byte(`{"type":"object"}`)); err != nil {
		t.Fatalf("WriteRequest() error = %v", err)
	}
	if err := bundle.WriteResult([]byte(`{"decisions":[]}`), "stderr log", nil); err != nil {
		t.Fatalf("WriteResult() error = %v", err)
	}

	for _, path := range []string{
		bundle.metadataPath,
		bundle.requestInputPath,
		bundle.requestPromptPath,
		bundle.requestSchemaPath,
		bundle.responseOutputPath,
		bundle.responseStderrPath,
	} {
		if _, err := os.Stat(path); err != nil {
			t.Fatalf("expected debug file to exist: %s: %v", path, err)
		}
	}

	metadataBytes, err := os.ReadFile(bundle.metadataPath)
	if err != nil {
		t.Fatalf("ReadFile(metadata) error = %v", err)
	}
	metadata := string(metadataBytes)

	if !strings.Contains(metadata, `"status": "success"`) {
		t.Fatalf("metadata must record success status: %s", metadata)
	}
	if !strings.Contains(metadata, filepath.Join("request", "input.json")) {
		t.Fatalf("metadata must include request/input.json path: %s", metadata)
	}
	if !strings.Contains(metadata, filepath.Join("response", "stderr.log")) {
		t.Fatalf("metadata must include response/stderr.log path: %s", metadata)
	}
}
