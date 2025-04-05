package lib

import "regexp"

var (
	VersionStrRegex = regexp.MustCompile(`^20[0-9]{2}\.[0-9]{1,2}\.[0-9]{1,2}\.[0-9]+$`)
)

// ValidateResoniteVersionString は文字列がResoniteのバージョン表記であるかをチェックする. (例: 2025.4.3.1346)
func ValidateResoniteVersionString(version string) bool {
	return VersionStrRegex.MatchString(version)
}
