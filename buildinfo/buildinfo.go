
//go:generate go run ./script/buildinfo-extractor.go .
//
// Generated: 2024-03-28T11:14:08-07:00 
//
package buildinfo

var VERSION_INFO = "264283f"
func BuildInfo() string {
	return VERSION_INFO
}
