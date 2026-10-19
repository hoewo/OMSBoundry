package buildinfo

import (
	"encoding/json"
	"fmt"
	"io"
	"runtime"
)

var (
	Version = "dev"
	Commit  = "unknown"
	BuiltAt = "unknown"
)

type Info struct {
	Service   string `json:"service"`
	Version   string `json:"version"`
	Commit    string `json:"commit"`
	BuiltAt   string `json:"built_at"`
	GoVersion string `json:"go_version"`
	Target    string `json:"target"`
}

func Current() Info {
	return Info{Service: "evidence-service", Version: Version, Commit: Commit, BuiltAt: BuiltAt, GoVersion: runtime.Version(), Target: runtime.GOOS + "/" + runtime.GOARCH}
}
func Write(writer io.Writer) error {
	if writer == nil {
		return fmt.Errorf("version output writer is required")
	}
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	return encoder.Encode(Current())
}
func Requested(arguments []string) bool {
	return len(arguments) == 1 && (arguments[0] == "version" || arguments[0] == "--version")
}
