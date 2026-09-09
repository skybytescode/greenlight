package vcs

import (
	"fmt"
	"runtime/debug"
)

func Version() string {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return ""
	}

	var (
		revision string
		time     string
		modified bool
	)

	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			revision = s.Value
		case "vcs.time":
			time = s.Value
		case "vcs.modified":
			if s.Value == "true" {
				modified = true
			}
		}
	}

	if revision == "" {
		return ""
	}

	if modified {
		return fmt.Sprintf("%s-%s-dirty", time, revision)
	}

	return fmt.Sprintf("%s-%s", time, revision)
}
