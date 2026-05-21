// genicons generates PWA icons and favicon.ico from assets/icons/plop1.png.
// Run from project root: go run ./cmd/genicons
package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"log"
	"os"
)

func main() {
	src, err := loadPNG("assets/icons/plop1.png")
	if err != nil {
		log.Fatalf("load source image: %v", err)
	}

	if err := os.MkdirAll("web/icons", 0755); err != nil {
		log.Fatalf("create web/icons: %v", err)
	}

	for _, size := range []struct {
		path string
		px   int
	}{
		{"web/icons/icon-192.png", 192},
		{"web/icons/icon-512.png", 512},
	} {
		if err := savePNG(resizeNN(src, size.px), size.path); err != nil {
			log.Fatalf("write %s: %v", size.path, err)
		}
		fmt.Printf("wrote %s\n", size.path)
	}

	icoImages := []image.Image{
		resizeNN(src, 16),
		resizeNN(src, 32),
		resizeNN(src, 48),
	}
	if err := writeICO("web/favicon.ico", icoImages); err != nil {
		log.Fatalf("write favicon.ico: %v", err)
	}
	fmt.Println("wrote web/favicon.ico")

	if err := writeICO("desktop/tray/icon.ico", icoImages); err != nil {
		log.Fatalf("write desktop/tray/icon.ico: %v", err)
	}
	fmt.Println("wrote desktop/tray/icon.ico")
	fmt.Println("done.")
}

// resizeNN resizes src to size×size using nearest-neighbor sampling.
func resizeNN(src image.Image, size int) image.Image {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	draw.Draw(dst, dst.Bounds(), image.NewUniform(color.Transparent), image.Point{}, draw.Src)

	sb := src.Bounds()
	sw, sh := float64(sb.Dx()), float64(sb.Dy())
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			sx := int(float64(x)/float64(size)*sw) + sb.Min.X
			sy := int(float64(y)/float64(size)*sh) + sb.Min.Y
			dst.Set(x, y, src.At(sx, sy))
		}
	}
	return dst
}

// writeICO writes a valid ICO file containing embedded PNG images.
// Modern browsers (Vista+) and Windows support PNG-in-ICO.
func writeICO(path string, images []image.Image) error {
	type icoEntry struct {
		data []byte
		w, h int
	}

	entries := make([]icoEntry, len(images))
	for i, img := range images {
		b := img.Bounds()
		var buf bytes.Buffer
		if err := png.Encode(&buf, img); err != nil {
			return fmt.Errorf("encode image %d: %w", i, err)
		}
		entries[i] = icoEntry{data: buf.Bytes(), w: b.Dx(), h: b.Dy()}
	}

	var out bytes.Buffer

	// ICONDIR header
	writeU16(&out, 0)                   // reserved
	writeU16(&out, 1)                   // type: 1 = ICO
	writeU16(&out, uint16(len(entries))) // image count

	// Offset to first image data: 6 (header) + N*16 (directory entries)
	offset := uint32(6 + len(entries)*16)
	for _, e := range entries {
		w := uint8(e.w)
		if e.w >= 256 {
			w = 0
		}
		h := uint8(e.h)
		if e.h >= 256 {
			h = 0
		}
		out.WriteByte(w)                  // width (0 = 256)
		out.WriteByte(h)                  // height (0 = 256)
		out.WriteByte(0)                  // color count (0 = no palette)
		out.WriteByte(0)                  // reserved
		writeU16(&out, 1)                 // color planes
		writeU16(&out, 32)                // bits per pixel
		writeU32(&out, uint32(len(e.data))) // size of image data
		writeU32(&out, offset)            // offset of image data
		offset += uint32(len(e.data))
	}
	for _, e := range entries {
		out.Write(e.data)
	}

	return os.WriteFile(path, out.Bytes(), 0644)
}

func writeU16(b *bytes.Buffer, v uint16) {
	_ = binary.Write(b, binary.LittleEndian, v)
}

func writeU32(b *bytes.Buffer, v uint32) {
	_ = binary.Write(b, binary.LittleEndian, v)
}

func loadPNG(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return png.Decode(f)
}

func savePNG(img image.Image, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}
