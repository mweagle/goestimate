package main

import (
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/mweagle/goestimate/app"
	"github.com/mweagle/goestimate/buildinfo"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
)

// //////////////////////////////////////////////////////////////////////////////
// commandLineArgs
type commandLineArgs struct {
	logLevelValue   int
	inputFile       string
	outputDirectory string
	lightTheme      int64
	darkTheme       int64
}

func (cla *commandLineArgs) parseCommandLine(_ *slog.Logger) error {
	logLevelString := ""

	flag.StringVar(&logLevelString, "level", "INFO", "Logging verbosity level. Must be one of: {DEBUG, INFO, WARN, ERROR}.")
	flag.StringVar(&cla.inputFile, "input", "", "Full filepath to definition to be evaluated.")
	flag.StringVar(&cla.outputDirectory, "output", "", "Path to output directory for created files. Defaults to inputFile parent directory.")
	flag.Int64Var(&cla.lightTheme, "lightTheme", d2themescatalog.NeutralGrey.ID, "Light theme ID to use for generated SVG. Defaults to NeutralGrey.")
	flag.Int64Var(&cla.darkTheme, "darkTheme", d2themescatalog.DarkMauve.ID, "Light theme ID to use for generated SVG. Defaults to DarkMauve.")
	flag.Parse()

	// Parse the verbosity level
	switch strings.ToLower(logLevelString) {
	case "debug":
		cla.logLevelValue = int(slog.LevelDebug)
	case "info":
		cla.logLevelValue = int(slog.LevelInfo)
	case "warn":
		cla.logLevelValue = int(slog.LevelWarn)
	case "error":
		cla.logLevelValue = int(slog.LevelError)
	default:
		return fmt.Errorf("invalid log level specified: %s", logLevelString)
	}
	if len(cla.inputFile) <= 0 {
		return errors.New("empty inputFile path provided")
	}
	absPath, absPathErr := filepath.Abs(cla.inputFile)
	if absPathErr != nil {
		return absPathErr
	}
	cla.inputFile = absPath
	if len(cla.outputDirectory) <= 0 {
		cla.outputDirectory = path.Dir(cla.inputFile)
	}
	return nil
}

// //////////////////////////////////////////////////////////////////////////////
//
// _ __  __ _(_)_ _
// | '  \/ _` | | ' \
// |_|_|_\__,_|_|_||_|
//
// //////////////////////////////////////////////////////////////////////////////
func main() {
	lvl := &slog.LevelVar{}
	lvl.Set(slog.LevelInfo)
	logger := slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{
		Level: lvl,
	}))
	cla := commandLineArgs{}
	parseError := cla.parseCommandLine(logger)
	if parseError != nil {
		logger.Error("Failed to parse command line arguments", "error", parseError)
		os.Exit(-1)
	}
	lvl.Set(slog.Level(cla.logLevelValue))
	logger.Info("Welcome to goestimate!",
		"version", buildinfo.BuildInfo(),
		"go", runtime.Version())

	params := &app.ApplicationFlowGraphParams{
		InputFile:       cla.inputFile,
		OutputDirectory: cla.outputDirectory,
		CreateDot:       true,
		LightThemeID:    cla.lightTheme,
		DarkThemeID:     cla.darkTheme,
	}
	_, err := app.NewApplicationFlowGraph(params, logger)
	if err != nil {
		logger.Error("Failed to create graph", "error", err)
	}
	logger.Info("goestimate generated")
}
