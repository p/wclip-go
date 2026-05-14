package main

import (
  "fmt"
  "runtime"
  "runtime/debug"
)

// Build-time injectable via:
//   go build -ldflags '-X main.version=... -X main.commit=... -X main.date=...'
// When unset, fields are populated best-effort from
// runtime/debug.ReadBuildInfo() so even a plain `go build ./src`
// produces a binary that knows its commit and dirty status.
var (
  version = "dev"
  commit  = ""
  date    = ""
)

// buildInfo returns the resolved version, commit, date, and dirty
// flag, filling fields not set by ldflags from the Go toolchain's
// embedded VCS info.
func buildInfo() (v, c, d string, dirty bool) {
  v, c, d = version, commit, date
  if bi, ok := debug.ReadBuildInfo(); ok {
    // `go install pkg@vX.Y.Z` sets bi.Main.Version to that tag.
    if v == "dev" && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
      v = bi.Main.Version
    }
    for _, s := range bi.Settings {
      switch s.Key {
      case "vcs.revision":
        if c == "" {
          c = shortSHA(s.Value)
        }
      case "vcs.time":
        if d == "" {
          d = s.Value
        }
      case "vcs.modified":
        if s.Value == "true" {
          dirty = true
        }
      }
    }
  }
  return v, c, d, dirty
}

// shortSHA returns up to the first 7 characters of a git SHA.
func shortSHA(s string) string {
  if len(s) > 7 {
    return s[:7]
  }
  return s
}

// shortVersion returns just the version (e.g. for startup logs).
func shortVersion() string {
  v, _, _, _ := buildInfo()
  return v
}

// versionString returns the multi-line --version output.
func versionString() string {
  v, c, d, dirty := buildInfo()
  return formatVersion(v, c, d, dirty, runtime.Version(), runtime.GOOS, runtime.GOARCH)
}

// formatVersion is the pure-formatter half of versionString, split out
// so it can be unit-tested without depending on package vars.
func formatVersion(v, c, d string, dirty bool, goVer, goos, goarch string) string {
  if c == "" {
    c = "unknown"
  }
  if d == "" {
    d = "unknown"
  }
  dirtyStr := ""
  if dirty {
    dirtyStr = " (dirty)"
  }
  return fmt.Sprintf(
    "wclip %s\n  commit:  %s%s\n  built:   %s\n  go:      %s\n  os/arch: %s/%s",
    v, c, dirtyStr, d, goVer, goos, goarch,
  )
}
// scratch
