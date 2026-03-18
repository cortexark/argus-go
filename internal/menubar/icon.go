// Package menubar provides the macOS menu bar app for Argus.
package menubar

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
)

// IconActive returns a 22x22 PNG icon for the "monitoring active" state (solid eye).
func IconActive() []byte {
	return renderIcon(true)
}

// IconPaused returns a 22x22 PNG icon for the "paused" state (dimmed eye).
func IconPaused() []byte {
	return renderIcon(false)
}

// renderIcon draws a minimal eye-shaped icon as a 22x22 RGBA PNG.
// macOS renders menu bar icons as template images (black with alpha).
func renderIcon(active bool) []byte {
	const size = 22
	img := image.NewRGBA(image.Rect(0, 0, size, size))

	// Fill transparent
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			img.Set(x, y, color.Transparent)
		}
	}

	a := uint8(255)
	if !active {
		a = 100
	}
	ink := color.RGBA{0, 0, 0, a}

	// Draw an eye shape: outer oval + inner circle (pupil)
	cx, cy := 11.0, 11.0

	setIfInEye := func(x, y int) {
		dx := float64(x) - cx
		dy := (float64(y) - cy) * 2.0 // squash vertically → oval
		dist := dx*dx + dy*dy
		if dist <= 36 { // outer oval radius ~6 (squashed)
			innerDist := dx*dx + (float64(y)-cy)*(float64(y)-cy)
			if innerDist <= 4 { // pupil radius 2
				img.Set(x, y, ink)
			} else if dist <= 36 && dist >= 25 {
				// rim of eye
				img.Set(x, y, ink)
			}
		}
	}

	// Eye outline: top and bottom arcs
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := float64(x) - cx
			dy := float64(y) - cy

			// Outer eye oval (horizontal)
			outerA, outerB := 9.0, 4.0
			outerVal := (dx*dx)/(outerA*outerA) + (dy*dy)/(outerB*outerB)

			// Inner eye oval (slightly smaller)
			innerA, innerB := 7.0, 3.0
			innerVal := (dx*dx)/(innerA*innerA) + (dy*dy)/(innerB*innerB)

			// Ring between outer and inner = eye white outline
			if outerVal <= 1.0 && innerVal >= 1.0 {
				img.Set(x, y, ink)
			}

			// Pupil circle
			pupilR := 2.5
			if dx*dx+dy*dy <= pupilR*pupilR {
				img.Set(x, y, ink)
			}
		}
	}

	_ = setIfInEye // unused after switch to oval approach

	var buf bytes.Buffer
	_ = png.Encode(&buf, img)
	return buf.Bytes()
}
