package version

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

// Info holds build-time version information.
type Info struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

// BuildInfo returns the current build version information.
func BuildInfo() Info {
	return Info{
		Version: Version,
		Commit:  Commit,
		Date:    Date,
	}
}
