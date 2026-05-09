package models_test

import (
	"testing"

	"github.com/mihirsn/planck/internal/models"
)

func TestDefaultFieldMap(t *testing.T) {
	t.Parallel()

	fm := models.DefaultFieldMap()

	if fm.Timestamp != "timestamp" {
		t.Errorf("expected Timestamp=timestamp, got %q", fm.Timestamp)
	}
	if fm.Method != "method" {
		t.Errorf("expected Method=method, got %q", fm.Method)
	}
	if fm.Path != "path" {
		t.Errorf("expected Path=path, got %q", fm.Path)
	}
	if fm.Status != "status" {
		t.Errorf("expected Status=status, got %q", fm.Status)
	}
	if fm.LatencyMs != "latency_ms" {
		t.Errorf("expected LatencyMs=latency_ms, got %q", fm.LatencyMs)
	}
}

func TestPresetFieldMap_Default(t *testing.T) {
	t.Parallel()

	for _, name := range []string{"", "default"} {
		fm, err := models.PresetFieldMap(name)
		if err != nil {
			t.Errorf("PresetFieldMap(%q) unexpected error: %v", name, err)
		}
		if fm.Path != "path" {
			t.Errorf("PresetFieldMap(%q): expected path=path, got %q", name, fm.Path)
		}
	}
}

func TestPresetFieldMap_FastAPI(t *testing.T) {
	t.Parallel()

	fm, err := models.PresetFieldMap(models.PresetFastAPI)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Status != "status_code" {
		t.Errorf("fastapi: expected Status=status_code, got %q", fm.Status)
	}
	if fm.LatencyMs != "duration" {
		t.Errorf("fastapi: expected LatencyMs=duration, got %q", fm.LatencyMs)
	}
}

func TestPresetFieldMap_Express(t *testing.T) {
	t.Parallel()

	fm, err := models.PresetFieldMap(models.PresetExpress)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Path != "url" {
		t.Errorf("express: expected Path=url, got %q", fm.Path)
	}
	if fm.Status != "statusCode" {
		t.Errorf("express: expected Status=statusCode, got %q", fm.Status)
	}
	if fm.LatencyMs != "responseTime" {
		t.Errorf("express: expected LatencyMs=responseTime, got %q", fm.LatencyMs)
	}
}

func TestPresetFieldMap_Gin(t *testing.T) {
	t.Parallel()

	fm, err := models.PresetFieldMap(models.PresetGin)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Timestamp != "time" {
		t.Errorf("gin: expected Timestamp=time, got %q", fm.Timestamp)
	}
	if fm.LatencyMs != "latency" {
		t.Errorf("gin: expected LatencyMs=latency, got %q", fm.LatencyMs)
	}
}

func TestPresetFieldMap_Echo(t *testing.T) {
	t.Parallel()

	fm, err := models.PresetFieldMap(models.PresetEcho)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Path != "uri" {
		t.Errorf("echo: expected Path=uri, got %q", fm.Path)
	}
	if fm.Timestamp != "time" {
		t.Errorf("echo: expected Timestamp=time, got %q", fm.Timestamp)
	}
}

func TestPresetFieldMap_Spring(t *testing.T) {
	t.Parallel()

	fm, err := models.PresetFieldMap(models.PresetSpring)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if fm.Path != "uri" {
		t.Errorf("spring: expected Path=uri, got %q", fm.Path)
	}
	if fm.LatencyMs != "duration" {
		t.Errorf("spring: expected LatencyMs=duration, got %q", fm.LatencyMs)
	}
}

func TestPresetFieldMap_Unknown(t *testing.T) {
	t.Parallel()

	_, err := models.PresetFieldMap("django")
	if err == nil {
		t.Error("expected error for unknown preset, got nil")
	}
}

func TestAvailablePresets(t *testing.T) {
	t.Parallel()

	presets := models.AvailablePresets()
	if len(presets) == 0 {
		t.Error("expected at least one preset")
	}

	// All returned presets should be loadable.
	for _, name := range presets {
		if _, err := models.PresetFieldMap(name); err != nil {
			t.Errorf("AvailablePresets returned %q but PresetFieldMap failed: %v", name, err)
		}
	}
}
