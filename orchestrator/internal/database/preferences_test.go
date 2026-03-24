package database

import (
	"testing"
)

func TestParsePreferenceValue_Nil(t *testing.T) {
	got, err := ParsePreferenceValue(nil)
	if err != nil || got != nil {
		t.Errorf("got %v, err %v", got, err)
	}
}

func TestParsePreferenceValue_Empty(t *testing.T) {
	got, err := ParsePreferenceValue(&testPrefEmpty)
	if err != nil || got != nil {
		t.Errorf("got %v, err %v", got, err)
	}
}

func TestParsePreferenceValue_Whitespace(t *testing.T) {
	got, err := ParsePreferenceValue(&testPrefWhitespace)
	if err != nil || got != nil {
		t.Errorf("got %v, err %v", got, err)
	}
}

func TestParsePreferenceValue_String(t *testing.T) {
	got, err := ParsePreferenceValue(&testPrefJSONString)
	if err != nil || got != "hello" {
		t.Errorf("got %v, err %v", got, err)
	}
}

func TestParsePreferenceValue_Number(t *testing.T) {
	got, err := ParsePreferenceValue(&testPrefNumber)
	if err != nil || got != float64(42) {
		t.Errorf("got %v, err %v", got, err)
	}
}

func TestParsePreferenceValue_Object(t *testing.T) {
	got, err := ParsePreferenceValue(&testPrefJSONObject)
	if err != nil {
		t.Fatal(err)
	}
	if m, ok := got.(map[string]interface{}); !ok || m["a"] != float64(1) {
		t.Errorf("got %v", got)
	}
}

func TestParsePreferenceValue_InvalidJSON(t *testing.T) {
	_, err := ParsePreferenceValue(&testPrefInvalidJSON)
	if err == nil {
		t.Error("expected error")
	}
}
