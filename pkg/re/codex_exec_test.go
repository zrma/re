package re

import (
	"errors"
	"os/exec"
	"strings"
	"testing"
)

func TestAIOutputSchemaDisallowsAdditionalProperties(t *testing.T) {
	schema := aiOutputSchema()

	if schema["additionalProperties"] != false {
		t.Fatalf("root schema must set additionalProperties=false")
	}

	properties, ok := schema["properties"].(map[string]any)
	if !ok {
		t.Fatalf("root properties must be a map")
	}

	decisions, ok := properties["decisions"].(map[string]any)
	if !ok {
		t.Fatalf("decisions schema must be a map")
	}

	items, ok := decisions["items"].(map[string]any)
	if !ok {
		t.Fatalf("decisions.items schema must be a map")
	}

	if items["additionalProperties"] != false {
		t.Fatalf("decision item schema must set additionalProperties=false")
	}
}

func TestSummarizeCodexExecErrorExtractsStructuredAPIMessage(t *testing.T) {
	stderr := strings.TrimSpace(`
2026-03-21T14:23:41.477711Z  WARN something noisy
ERROR: {
  "type": "error",
  "error": {
    "type": "invalid_request_error",
    "message": "Invalid schema for response_format 'codex_output_schema': additionalProperties must be false."
  },
  "status": 400
}
`)

	err := summarizeCodexExecError(errors.New("exit status 1"), stderr)
	if got := err.Error(); got != "codex exec request failed: Invalid schema for response_format 'codex_output_schema': additionalProperties must be false." {
		t.Fatalf("unexpected error summary: %s", got)
	}
}

func TestSummarizeCodexExecErrorHandlesMissingBinary(t *testing.T) {
	err := summarizeCodexExecError(exec.ErrNotFound, "")
	if got := err.Error(); got != "codex CLI was not found in PATH" {
		t.Fatalf("unexpected missing-binary summary: %s", got)
	}
}
