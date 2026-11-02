package tracing

import (
	"strings"
	"testing"
)

func TestParseTraceparent_Valid(t *testing.T) {
	header := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	sc := ParseTraceparent(header)
	if !sc.Valid() {
		t.Fatal("expected valid span context")
	}
	if sc.TraceID != "4bf92f3577b34da6a3ce929d0e0e4736" {
		t.Errorf("traceID = %q", sc.TraceID)
	}
	if sc.SpanID != "00f067aa0ba902b7" {
		t.Errorf("spanID = %q", sc.SpanID)
	}
	if sc.Flags != "01" {
		t.Errorf("flags = %q", sc.Flags)
	}
}

func TestParseTraceparent_Invalid(t *testing.T) {
	cases := []string{
		"",
		"invalid",
		"00-short-span-01",
		"00-00000000000000000000000000000000-00f067aa0ba902b7-01",
		"00-4bf92f3577b34da6a3ce929d0e0e4736-0000000000000000-01",
	}
	for _, c := range cases {
		sc := ParseTraceparent(c)
		if sc.Valid() {
			t.Errorf("expected invalid for %q, got %+v", c, sc)
		}
	}
}

func TestGenerateSpanID_Length(t *testing.T) {
	id := GenerateSpanID()
	if len(id) != 16 {
		t.Errorf("span ID length = %d, want 16", len(id))
	}
}

func TestGenerateSpanID_Unique(t *testing.T) {
	a := GenerateSpanID()
	b := GenerateSpanID()
	if a == b {
		t.Error("two generated span IDs should differ")
	}
}

func TestFormatTraceparent(t *testing.T) {
	sc := SpanContext{
		TraceID: "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:  "00f067aa0ba902b7",
		Flags:   "01",
	}
	got := FormatTraceparent(sc)
	want := "00-4bf92f3577b34da6a3ce929d0e0e4736-00f067aa0ba902b7-01"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestFormatTraceparent_Invalid(t *testing.T) {
	sc := SpanContext{}
	got := FormatTraceparent(sc)
	if got != "" {
		t.Errorf("expected empty string for invalid context, got %q", got)
	}
}

func TestFormatTraceparent_DefaultFlags(t *testing.T) {
	sc := SpanContext{
		TraceID: "4bf92f3577b34da6a3ce929d0e0e4736",
		SpanID:  "00f067aa0ba902b7",
	}
	got := FormatTraceparent(sc)
	if !strings.HasSuffix(got, "-01") {
		t.Errorf("expected default flags 01, got %q", got)
	}
}