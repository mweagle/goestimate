
//go:generate go run ./script/buildinfo-extractor.go .
//
// Generated: 2024-03-27T23:22:07-07:00 
//
package buildinfo

var VERSION_INFO = "d494748"
func BuildInfo() string {
	return VERSION_INFO
}
