package re

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

type CodexExecResolver struct {
	Model           string
	DebugOutputPath string
}

func (r CodexExecResolver) Resolve(ctx context.Context, input AIInput) (AIOutput, error) {
	model := r.Model
	if model == "" {
		model = "gpt-5.4-mini"
	}

	var debugBundle *codexExecDebugBundle
	if r.DebugOutputPath != "" {
		var err error
		debugBundle, err = newCodexExecDebugBundle(r.DebugOutputPath, model)
		if err != nil {
			return AIOutput{}, err
		}
	}

	tempDir, err := os.MkdirTemp("", "re-codex-exec-*")
	if err != nil {
		return AIOutput{}, err
	}
	defer func() { _ = os.RemoveAll(tempDir) }()

	var responseBytes []byte
	var stderrText string
	var resultErr error
	defer func() {
		if debugBundle != nil {
			_ = debugBundle.WriteResult(responseBytes, stderrText, resultErr)
		}
	}()

	inputBytes, err := json.MarshalIndent(input, "", "  ")
	if err != nil {
		resultErr = err
		return AIOutput{}, resultErr
	}

	schemaBytes, err := json.MarshalIndent(aiOutputSchema(), "", "  ")
	if err != nil {
		resultErr = err
		return AIOutput{}, resultErr
	}

	schemaPath := filepath.Join(tempDir, "schema.json")
	if err := os.WriteFile(schemaPath, schemaBytes, 0644); err != nil {
		resultErr = err
		return AIOutput{}, resultErr
	}

	responsePath := filepath.Join(tempDir, "response.json")
	prompt := buildCodexExecPrompt(inputBytes)

	if debugBundle != nil {
		if err := debugBundle.WriteRequest(inputBytes, prompt, schemaBytes); err != nil {
			resultErr = err
			return AIOutput{}, resultErr
		}
	}

	cmd := exec.CommandContext(
		ctx,
		"codex",
		"exec",
		"--skip-git-repo-check",
		"--sandbox", "read-only",
		"--model", model,
		"--output-schema", schemaPath,
		"-o", responsePath,
		"-",
	)
	cmd.Stdin = strings.NewReader(prompt)

	var stderr strings.Builder
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		stderrText = stderr.String()
		resultErr = summarizeCodexExecError(err, stderrText)
		return AIOutput{}, resultErr
	}

	stderrText = stderr.String()
	responseBytes, err = os.ReadFile(responsePath)
	if err != nil {
		resultErr = err
		return AIOutput{}, resultErr
	}
	if strings.TrimSpace(string(responseBytes)) == "" {
		resultErr = fmt.Errorf("codex exec returned an empty response")
		return AIOutput{}, resultErr
	}

	var output AIOutput
	if err := json.Unmarshal(responseBytes, &output); err != nil {
		resultErr = err
		return AIOutput{}, resultErr
	}

	return output, nil
}

func aiOutputSchema() map[string]any {
	return map[string]any{
		"type":                 "object",
		"additionalProperties": false,
		"required":             []string{"decisions"},
		"properties": map[string]any{
			"decisions": map[string]any{
				"type": "array",
				"items": map[string]any{
					"type":                 "object",
					"additionalProperties": false,
					"required": []string{
						"subtitle_path",
						"outcome",
						"matched_movie_path",
						"confidence",
						"reason",
					},
					"properties": map[string]any{
						"subtitle_path": map[string]any{"type": "string"},
						"outcome": map[string]any{
							"type": "string",
							"enum": []string{
								string(AIDecisionMatch),
								string(AIDecisionSkip),
								string(AIDecisionNeedsHuman),
							},
						},
						"matched_movie_path": map[string]any{
							"type": "string",
						},
						"confidence": map[string]any{
							"type":    "number",
							"minimum": 0,
							"maximum": 1,
						},
						"reason": map[string]any{"type": "string"},
					},
				},
			},
		},
	}
}

func buildCodexExecPrompt(input []byte) string {
	return strings.TrimSpace(`
You are assisting a local subtitle renamer.
Return JSON only.
Do not invent filenames.
If uncertain, choose "skip" or "needs_human".
Only match a subtitle to an existing movie path from the provided list.
Prefer precision over recall.
Use the provided JSON schema.
Use an empty string for matched_movie_path when outcome is "skip" or "needs_human".

Input JSON:
`) + "\n" + string(input) + "\n"
}

func summarizeCodexExecError(err error, stderr string) error {
	switch {
	case errors.Is(err, exec.ErrNotFound):
		return fmt.Errorf("codex CLI was not found in PATH")
	case errors.Is(err, context.DeadlineExceeded):
		return fmt.Errorf("codex exec timed out")
	}

	if message := extractStructuredCodexErrorMessage(stderr); message != "" {
		return fmt.Errorf("codex exec request failed: %s", message)
	}

	if line := extractLastUsefulCodexLine(stderr); line != "" {
		return fmt.Errorf("codex exec failed: %s", line)
	}

	return fmt.Errorf("codex exec failed: %w", err)
}

func extractStructuredCodexErrorMessage(stderr string) string {
	re := regexp.MustCompile(`"message"\s*:\s*"((?:\\.|[^"])*)"`)
	match := re.FindStringSubmatch(stderr)
	if len(match) != 2 {
		return ""
	}

	message, err := strconv.Unquote(`"` + match[1] + `"`)
	if err != nil {
		return match[1]
	}
	return message
}

func extractLastUsefulCodexLine(stderr string) string {
	lines := strings.Split(stderr, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		line := strings.TrimSpace(lines[i])
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "202") {
			continue
		}
		if strings.HasPrefix(line, "mcp:") {
			continue
		}
		if strings.HasPrefix(line, "warning:") {
			continue
		}
		if strings.HasPrefix(line, "OpenAI Codex") {
			continue
		}
		if strings.HasPrefix(line, "workdir:") || strings.HasPrefix(line, "model:") || strings.HasPrefix(line, "provider:") {
			continue
		}
		if strings.HasPrefix(line, "approval:") || strings.HasPrefix(line, "sandbox:") || strings.HasPrefix(line, "reasoning effort:") {
			continue
		}
		if strings.HasPrefix(line, "reasoning summaries:") || strings.HasPrefix(line, "session id:") {
			continue
		}
		if line == "--------" || line == "user" {
			continue
		}
		return line
	}

	return ""
}
