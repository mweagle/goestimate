package app

import (
	"context"
	"log/slog"
	"os"

	"oss.terrastruct.com/d2/d2exporter"
	"oss.terrastruct.com/d2/d2layouts/d2elklayout"
	"oss.terrastruct.com/d2/d2lib"
	"oss.terrastruct.com/d2/d2renderers/d2svg"
	"oss.terrastruct.com/d2/d2themes/d2themescatalog"
	"oss.terrastruct.com/d2/lib/textmeasure"
)

func createD2Image(inputFile string,
	outputFile string,
	lightTheme int64,
	darkTheme int64,
	log *slog.Logger) error {
	log.Info("Creating D2 image from source", "path", inputFile)
	srcFile, srcFileErr := os.ReadFile(inputFile)
	if srcFileErr != nil {
		return srcFileErr
	}
	_, config, configErr := d2lib.Compile(context.Background(), string(srcFile), nil, nil)
	if configErr != nil {
		log.Warn("Error during compile", "error", configErr)
	}
	applyErr := config.ApplyTheme(d2themescatalog.ColorblindClear.ID)
	if applyErr != nil {
		return applyErr
	}
	ruler, rulerErr := textmeasure.NewRuler()
	if rulerErr != nil {
		return rulerErr
	}
	dimErr := config.SetDimensions(nil, ruler, nil)
	if dimErr != nil {
		return dimErr
	}
	layoutErr := d2elklayout.Layout(context.Background(), config, nil)
	if layoutErr != nil {
		return layoutErr
	}
	diagram, diagramErr := d2exporter.Export(context.Background(), config, nil)
	if diagramErr != nil {
		return diagramErr
	}
	sketch := false
	padding := int64(50)
	render, renderErr := d2svg.Render(diagram, &d2svg.RenderOpts{
		ThemeID:     &lightTheme,
		Sketch:      &sketch,
		DarkThemeID: &darkTheme,
		Pad:         &padding,
	})
	if renderErr != nil {
		return renderErr
	}
	log.Info("Writing D2 image", "path", outputFile)
	return os.WriteFile(outputFile, render, 0600)
}
