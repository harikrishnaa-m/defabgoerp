package storage

import (
	"bytes"
	"errors"
	"image"
	"mime/multipart"
	"path/filepath"
	"strings"

	"github.com/disintegration/imaging"
	"github.com/google/uuid"
)

const (
	MaxImageSize = 5 << 20 // 5 MB
	MaxWidth     = 1200
)

var allowedTypes = map[string]bool{
	"image/jpeg": true,
	"image/png":  true,
}

func ProcessImage(file *multipart.FileHeader) ([]byte, string, error) {

	// ✅ size check
	if file.Size > MaxImageSize {
		return nil, "", errors.New("file too large (max 5MB)")
	}

	// ✅ mime check
	ct := file.Header.Get("Content-Type")
	if !allowedTypes[ct] {
		return nil, "", errors.New("invalid image type")
	}

	f, err := file.Open()
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	img, _, err := image.Decode(f)
	if err != nil {
		return nil, "", errors.New("invalid image data")
	}

	// ✅ resize
	img = imaging.Resize(img, MaxWidth, 0, imaging.Lanczos)

	buf := new(bytes.Buffer)

	ext := extFromMime(ct)

	switch ext {
	case ".jpg":
		err = imaging.Encode(buf, img, imaging.JPEG)
	case ".png":
		err = imaging.Encode(buf, img, imaging.PNG)
	}

	if err != nil {
		return nil, "", err
	}

	// ✅ unique filename
	filename := uuid.New().String() + ext

	return buf.Bytes(), filename, nil
}

func extFromMime(ct string) string {
	switch ct {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	default:
		return filepath.Ext(strings.ToLower(ct))
	}
}
