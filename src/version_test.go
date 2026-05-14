package main

import (
  "strings"
  "testing"
)

func TestFormatVersion_AllFieldsPresent(t *testing.T) {
  out := formatVersion("0.3.0", "abc1234", "2026-05-14T10:00:00Z", false,
    "go1.22.0", "linux", "amd64")
  for _, want := range []string{
    "wclip 0.3.0",
    "abc1234",
    "2026-05-14T10:00:00Z",
    "go1.22.0",
    "linux/amd64",
  } {
    if !strings.Contains(out, want) {
      t.Errorf("formatVersion output missing %q:\n%s", want, out)
    }
  }
  if strings.Contains(out, "(dirty)") {
    t.Errorf("unexpected (dirty) marker:\n%s", out)
  }
}

func TestFormatVersion_DirtyMarker(t *testing.T) {
  out := formatVersion("0.3.0-5-gabc1234", "abc1234", "2026-05-14T10:00:00Z", true,
    "go1.22.0", "linux", "amd64")
  if !strings.Contains(out, "(dirty)") {
    t.Errorf("expected (dirty) marker, got:\n%s", out)
  }
}

func TestFormatVersion_UnknownFallbacks(t *testing.T) {
  // Empty commit/date should render as "unknown" rather than blank.
  out := formatVersion("dev", "", "", false, "go1.22.0", "linux", "amd64")
  if !strings.Contains(out, "commit:  unknown") {
    t.Errorf("expected 'commit:  unknown', got:\n%s", out)
  }
  if !strings.Contains(out, "built:   unknown") {
    t.Errorf("expected 'built:   unknown', got:\n%s", out)
  }
}

func TestShortSHA(t *testing.T) {
  cases := []struct {
    in, want string
  }{
    {"", ""},
    {"abc", "abc"},
    {"abcdefg", "abcdefg"},        // exactly 7
    {"abcdefgh", "abcdefg"},       // truncate
    {"abc1234567890def", "abc1234"},
  }
  for _, c := range cases {
    if got := shortSHA(c.in); got != c.want {
      t.Errorf("shortSHA(%q) = %q, want %q", c.in, got, c.want)
    }
  }
}

func TestBuildInfo_DoesNotPanic(t *testing.T) {
  // We can't easily mock debug.ReadBuildInfo, but we can assert the
  // function returns something usable in a real test binary (where
  // VCS info is typically present).
  v, _, _, _ := buildInfo()
  if v == "" {
    t.Errorf("buildInfo returned empty version")
  }
}

func TestVersionString_ContainsVersion(t *testing.T) {
  out := versionString()
  if !strings.Contains(out, "wclip ") {
    t.Errorf("versionString missing 'wclip ' prefix:\n%s", out)
  }
}
