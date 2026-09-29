package main

import (
	"image"
	"image/color"
	"image/png"
	"os"
)

func main() {
	const size, scale = 256, 4
	large := image.NewNRGBA(image.Rect(0, 0, size*scale, size*scale))
	background := color.NRGBA{R: 31, G: 69, B: 54, A: 255}
	foreground := color.NRGBA{R: 245, G: 250, B: 247, A: 255}
	radius := 52 * scale
	for y := 0; y < size*scale; y++ {
		for x := 0; x < size*scale; x++ {
			cx, cy := x, y
			if cx < radius { cx = radius }
			if cx >= size*scale-radius { cx = size*scale-radius-1 }
			if cy < radius { cy = radius }
			if cy >= size*scale-radius { cy = size*scale-radius-1 }
			dx, dy := x-cx, y-cy
			if dx*dx+dy*dy <= radius*radius { large.SetNRGBA(x, y, background) }
		}
	}
	pattern := []string{"1000001", "1100011", "1010101", "1010101", "1001001", "1001001", "1000001"}
	cell := 25 * scale
	x0, y0 := (size*scale-7*cell)/2, (size*scale-7*cell)/2
	for row, line := range pattern {
		for column, bit := range line {
			if bit != '1' { continue }
			for y := 0; y < cell; y++ {
				for x := 0; x < cell; x++ { large.SetNRGBA(x0+column*cell+x, y0+row*cell+y, foreground) }
			}
		}
	}
	small := image.NewNRGBA(image.Rect(0, 0, size, size))
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			var r, g, b, a uint32
			for sy := 0; sy < scale; sy++ {
				for sx := 0; sx < scale; sx++ {
					cr, cg, cb, ca := large.At(x*scale+sx, y*scale+sy).RGBA()
					r, g, b, a = r+cr, g+cg, b+cb, a+ca
				}
			}
			divisor := uint32(scale * scale)
			small.SetNRGBA(x, y, color.NRGBA{R: uint8(r/divisor >> 8), G: uint8(g/divisor >> 8), B: uint8(b/divisor >> 8), A: uint8(a/divisor >> 8)})
		}
	}
	file, err := os.Create("Icon.png")
	if err != nil { panic(err) }
	if err := png.Encode(file, small); err != nil { panic(err) }
	if err := file.Close(); err != nil { panic(err) }
}
