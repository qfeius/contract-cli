package build

import "fmt"

const Name = "contract-cli"

var (
	Version = "dev"
	Commit  = "unknown"
	Date    = "unknown"
)

type Info struct {
	Name    string
	Version string
	Commit  string
	Date    string
}

func Current() Info {
	return Info{
		Name:    Name,
		Version: defaultString(Version, "dev"),
		Commit:  defaultString(Commit, "unknown"),
		Date:    defaultString(Date, "unknown"),
	}
}

func (i Info) String() string {
	return fmt.Sprintf("%s version %s (commit %s, built %s)", i.Name, defaultString(i.Version, "dev"), defaultString(i.Commit, "unknown"), defaultString(i.Date, "unknown"))
}

func defaultString(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
