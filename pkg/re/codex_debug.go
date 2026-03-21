package re

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

type codexExecDebugBundle struct {
	rootDir            string
	metadataPath       string
	requestInputPath   string
	requestPromptPath  string
	requestSchemaPath  string
	responseOutputPath string
	responseStderrPath string
	runID              string
	model              string
	createdAt          string
}

type codexExecDebugMetadata struct {
	RunID     string                    `json:"run_id"`
	CreatedAt string                    `json:"created_at"`
	Model     string                    `json:"model"`
	Status    string                    `json:"status"`
	Error     string                    `json:"error,omitempty"`
	Files     codexExecDebugMetadataSet `json:"files"`
}

type codexExecDebugMetadataSet struct {
	InputJSON    string `json:"input_json"`
	PromptText   string `json:"prompt_text"`
	SchemaJSON   string `json:"schema_json"`
	ResponseJSON string `json:"response_json"`
	StderrLog    string `json:"stderr_log"`
}

func newCodexExecDebugBundle(baseDir string, model string) (*codexExecDebugBundle, error) {
	runID := "run-" + time.Now().UTC().Format("20060102T150405.000000000Z")
	rootDir := filepath.Join(baseDir, runID)
	requestDir := filepath.Join(rootDir, "request")
	responseDir := filepath.Join(rootDir, "response")

	if err := os.MkdirAll(requestDir, 0755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(responseDir, 0755); err != nil {
		return nil, err
	}

	bundle := &codexExecDebugBundle{
		rootDir:            rootDir,
		metadataPath:       filepath.Join(rootDir, "metadata.json"),
		requestInputPath:   filepath.Join(requestDir, "input.json"),
		requestPromptPath:  filepath.Join(requestDir, "prompt.txt"),
		requestSchemaPath:  filepath.Join(requestDir, "schema.json"),
		responseOutputPath: filepath.Join(responseDir, "output.json"),
		responseStderrPath: filepath.Join(responseDir, "stderr.log"),
		runID:              runID,
		model:              model,
		createdAt:          time.Now().UTC().Format(time.RFC3339Nano),
	}

	if err := bundle.writeMetadata("started", ""); err != nil {
		return nil, err
	}

	return bundle, nil
}

func (b *codexExecDebugBundle) WriteRequest(inputBytes []byte, prompt string, schemaBytes []byte) error {
	if err := os.WriteFile(b.requestInputPath, inputBytes, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(b.requestPromptPath, []byte(prompt), 0644); err != nil {
		return err
	}
	if err := os.WriteFile(b.requestSchemaPath, schemaBytes, 0644); err != nil {
		return err
	}
	return nil
}

func (b *codexExecDebugBundle) WriteResult(responseBytes []byte, stderrText string, resultErr error) error {
	if err := os.WriteFile(b.responseOutputPath, responseBytes, 0644); err != nil {
		return err
	}
	if err := os.WriteFile(b.responseStderrPath, []byte(stderrText), 0644); err != nil {
		return err
	}

	status := "success"
	errorMessage := ""
	if resultErr != nil {
		status = "error"
		errorMessage = resultErr.Error()
	}

	return b.writeMetadata(status, errorMessage)
}

func (b *codexExecDebugBundle) writeMetadata(status string, errorMessage string) error {
	metadata := codexExecDebugMetadata{
		RunID:     b.runID,
		CreatedAt: b.createdAt,
		Model:     b.model,
		Status:    status,
		Error:     errorMessage,
		Files: codexExecDebugMetadataSet{
			InputJSON:    filepath.Join("request", "input.json"),
			PromptText:   filepath.Join("request", "prompt.txt"),
			SchemaJSON:   filepath.Join("request", "schema.json"),
			ResponseJSON: filepath.Join("response", "output.json"),
			StderrLog:    filepath.Join("response", "stderr.log"),
		},
	}

	metadataBytes, err := json.MarshalIndent(metadata, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(b.metadataPath, metadataBytes, 0644)
}
