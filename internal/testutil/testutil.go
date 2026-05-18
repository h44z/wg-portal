package testutil

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"testing"
)

func CaptureWarnLogs(t *testing.T) (restore func() []map[string]any) {
	t.Helper()
	original := slog.Default()
	var buf bytes.Buffer
	handler := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	slog.SetDefault(slog.New(handler))

	return func() []map[string]any {
		slog.SetDefault(original)
		var records []map[string]any
		decoder := json.NewDecoder(&buf)
		for decoder.More() {
			var rec map[string]any
			if err := decoder.Decode(&rec); err == nil {
				records = append(records, rec)
			}
		}
		return records
	}
}

func CountWarnEntries(records []map[string]any) int {
	count := 0
	for _, r := range records {
		if lvl, ok := r["level"].(string); ok && lvl == "WARN" {
			count++
		}
	}
	return count
}

func FindWarnWithField(records []map[string]any, fieldName string) (map[string]any, bool) {
	for _, r := range records {
		if lvl, ok := r["level"].(string); ok && lvl == "WARN" {
			if f, ok := r["field"].(string); ok && f == fieldName {
				return r, true
			}
		}
	}
	return nil, false
}
