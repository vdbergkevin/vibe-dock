// Command theme-icon derives the light Dock icon from VibeDock's dark source
// icon while preserving the coloured, pixel-art mark and transparent corners.
package main

import (
	"image"
	"image/color"
	"image/png"
	"log"
	"os"
)

const (
	inputPath  = "build/appicon.png"
	outputPath = "build/appicon-light.png"
)

func main() {
	input, err := os.Open(inputPath)
	if err != nil {
		log.Fatal(err)
	}
	defer input.Close()

	source, err := png.Decode(input)
	if err != nil {
		log.Fatal(err)
	}
	result := image.NewNRGBA(source.Bounds())
	for y := source.Bounds().Min.Y; y < source.Bounds().Max.Y; y++ {
		for x := source.Bounds().Min.X; x < source.Bounds().Max.X; x++ {
			pixel := color.NRGBAModel.Convert(source.At(x, y)).(color.NRGBA)
			result.SetNRGBA(x, y, lightPixel(pixel))
		}
	}

	output, err := os.Create(outputPath)
	if err != nil {
		log.Fatal(err)
	}
	if err := png.Encode(output, result); err != nil {
		_ = output.Close()
		log.Fatal(err)
	}
	if err := output.Close(); err != nil {
		log.Fatal(err)
	}
}

func lightPixel(pixel color.NRGBA) color.NRGBA {
	if pixel.A == 0 {
		return pixel
	}
	minimum := min(pixel.R, pixel.G, pixel.B)
	maximum := max(pixel.R, pixel.G, pixel.B)
	// The source artwork uses neutral charcoal only for the icon tile and its
	// shading. Warm pixels belong to the VibeDock mark and border, so leave
	// those untouched.
	if maximum-minimum <= 28 && maximum < 125 {
		return color.NRGBA{R: 255, G: 255, B: 255, A: pixel.A}
	}
	return pixel
}
