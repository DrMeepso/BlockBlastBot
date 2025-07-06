package main

import (
	"image"
	"image/png"
	"os"
)

func int64ToFullBinaryASCII(n int64) string {
	result := make([]byte, 64)

	for i := 63; i >= 0; i-- {
		if (n>>i)&1 == 1 {
			result[63-i] = '1'
		} else {
			result[63-i] = '0'
		}
	}

	return string(result)
}

func uint64ToFullBinaryASCII(n uint64) string {
	result := make([]byte, 64)

	for i := 63; i >= 0; i-- {
		if (n>>i)&1 == 1 {
			result[63-i] = '1'
		} else {
			result[63-i] = '0'
		}
	}

	return string(result)
}

func saveRectToFile(img image.Image, rect image.Rectangle, filename string) error {
	cutImg := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}).SubImage(rect)

	outFile, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer outFile.Close()

	err = png.Encode(outFile, cutImg)
	if err != nil {
		return err
	}

	return nil
}
