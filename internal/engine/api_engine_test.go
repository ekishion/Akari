package engine

import (
	"testing"
)

func TestCleanJsonPath(t *testing.T) {
	if cleanJsonPath("$.data.records[*]") != "data.records[*]" {
		t.Fatalf("expected data.records[*], got %s", cleanJsonPath("$.data.records[*]"))
	}
	if cleanJsonPath("$title") != "title" {
		t.Fatalf("expected title, got %s", cleanJsonPath("$title"))
	}
}

func TestFormatRequestBody(t *testing.T) {
	bodyStr := formatRequestBody(`{"keyword":"@keyword"}`, "@keyword", "葬送")
	if bodyStr != `{"keyword":"葬送"}` {
		t.Fatalf("expected replaced keyword, got %s", bodyStr)
	}

	bodyMap := map[string]any{"search": "@keyword"}
	bodyMapStr := formatRequestBody(bodyMap, "@keyword", "芙莉莲")
	if bodyMapStr != `{"search":"芙莉莲"}` {
		t.Fatalf("expected replaced keyword in map, got %s", bodyMapStr)
	}
}
