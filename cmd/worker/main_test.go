package main

import (
	"bytes"
	"image"
	"image/jpeg"
	"testing"
)

func TestResizeJPEG_LandscapeScalesToLongSide(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 1000, 500)) // широкая картинка 1000x500

	data, err := resizeJPEG(src, 300)
	if err != nil {
		t.Fatalf("resizeJPEG вернула ошибку: %v", err)
	}

	got, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("результат не открывается как JPEG: %v", err)
	}
	w, h := got.Bounds().Dx(), got.Bounds().Dy()
	if w != 300 || h != 150 {
		t.Errorf("ждали 300x150, получили %dx%d", w, h)
	}
}

func TestResizeJPEG_SmallImageNotEnlarged(t *testing.T) {
	src := image.NewRGBA(image.Rect(0, 0, 150, 100)) // маленькая картинка 150x100

	data, err := resizeJPEG(src, 300)
	if err != nil {
		t.Fatalf("resizeJPEG вернула ошибку: %v", err)
	}

	got, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatalf("результат не открывается как JPEG: %v", err)
	}

	w, h := got.Bounds().Dx(), got.Bounds().Dy()
	if w != 150 || h != 100 {
		t.Errorf("ждали 150x100, получили увеличеную до %dx%d", w, h)
	}
}