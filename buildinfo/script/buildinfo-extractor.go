//go:build ignore

package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// //////////////////////////////////////////////////////////////////////////////
//
// _ __  __ _(_)_ _
// | '  \/ _` | | ' \
// |_|_|_\__,_|_|_||_|
//
// //////////////////////////////////////////////////////////////////////////////
func main() {
	nowTime := time.Now().Format(time.RFC3339)
	if len(os.Args) < 2 {
		log.Fatalf("Provide output directory as only command line argument")
	}
	outputDir := os.Args[1]
	absOutputPath, absOutputPathErr := filepath.Abs(outputDir)
	if absOutputPathErr != nil {
		log.Fatalf("Failed to get absolute output path. Error: %s", absOutputPathErr)
	}
	log.Printf("Output directory for buildinfo.go: %s", absOutputPath)
	cmd := exec.Command("git", "rev-parse", "--short", "HEAD")
	out, err := cmd.CombinedOutput()
	if err != nil {
		log.Fatalf("cmd.Run() failed with %s\n", err)
	}

	outputFile := filepath.Join(outputDir, "buildinfo.go")
	outFile, outFileErr := os.Create(outputFile)
	if outFileErr != nil {
		log.Fatalf("Failed to create output file: %s. Error: %s", outputFile, outFileErr)
	}
	outFileContents := fmt.Sprintf(`
//go:generate go run ./script/buildinfo-extractor.go .
//
// Generated: %s 
//
package buildinfo

var VERSION_INFO = "%s"
func BuildInfo() string {
	return VERSION_INFO
}
`,
		nowTime,
		strings.TrimSpace(string(out)))

	outFile.Write([]byte(outFileContents))
	log.Printf("Created output file: %s", outputFile)
}
