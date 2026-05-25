package storage

import (
	"bytes"
	"image"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
)

const (
	MaxWidth = 1200
)

func ProcessImage(file *multipart.FileHeader) ([]byte, string, error) {
	f, err := file.Open()
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	// Read raw bytes
	raw := new(bytes.Buffer)
	if _, err = raw.ReadFrom(f); err != nil {
		return nil, "", err
	}

	// Derive extension from original filename
	ext := strings.ToLower(filepath.Ext(file.Filename))
	if ext == "" {
		ext = ".jpg"
	}

	// Attempt decode + resize for supported formats; fall back to raw bytes if not
	img, _, decErr := image.Decode(bytes.NewReader(raw.Bytes()))
	if decErr == nil {
		img = imaging.Resize(img, MaxWidth, 0, imaging.Lanczos)
		buf := new(bytes.Buffer)
		var encErr error
		switch ext {
		case ".png":
			encErr = imaging.Encode(buf, img, imaging.PNG)
		case ".gif":
			encErr = imaging.Encode(buf, img, imaging.GIF)
		case ".tiff", ".tif":
			encErr = imaging.Encode(buf, img, imaging.TIFF)
		case ".bmp":
			encErr = imaging.Encode(buf, img, imaging.BMP)
		default: // .jpg, .jpeg, .webp, and anything else → encode as JPEG
			ext = ".jpg"
			encErr = imaging.Encode(buf, img, imaging.JPEG)
		}
		if encErr == nil {
			return buf.Bytes(), uuid.New().String() + ext, nil
		}
	}

	// Format not decodable by imaging (e.g. HEIC, AVIF) — store raw
	return raw.Bytes(), uuid.New().String() + ext, nil
}
