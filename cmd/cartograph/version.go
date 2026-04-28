package main

import (
	"fmt"
	"runtime/debug"
)

var version = ""

func printVersion() {
	if version != "" {
		fmt.Printf("cartograph %s\n", version)
		return
	}

	info, ok := debug.ReadBuildInfo()
	if !ok {
		fmt.Println("no build info")
		return
	}

	var vcsRevision string
	var vcsTime string
	var vcsModified bool

	for _, setting := range info.Settings {
		switch setting.Key {
		case "vcs.revision":
			vcsRevision = setting.Value
		case "vcs.time":
			vcsTime = setting.Value
		case "vcs.modified":
			vcsModified = setting.Value == "true"
		}
	}

	switch {
	case info.Main.Version != "(devel)" && info.Main.Version != "":
		fmt.Printf("cartograph %s\n", info.Main.Version)
	case vcsRevision != "":
		mod := ""
		if vcsModified {
			mod = "-dirty"
		}

		rev := vcsRevision
		if len(rev) > 8 {
			rev = rev[:8]
		}

		fmt.Printf("cartograph %s%s (%s)\n", rev, mod, vcsTime)
	default:
		fmt.Printf("cartograph (unknown version)\n")
	}
}
