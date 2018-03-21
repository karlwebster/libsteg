package libsteg

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"testing"

	logging "github.com/op/go-logging"
)

const (
	cleanImageFile = "./resources/clean.png"
	tinyImageFile  = "./resources/tiny.png"
	secretStringIn = "Karl"
)

var loggingLevel = flag.Int("loglevel", int(logging.CRITICAL),
	"log level to use [0=CRITICAL, 1=ERROR, 2=WARNING, 3=NOTICE, 4=INFO, 5=DEBUG]")

// TestB2BFile does a back to back file based test
func TestB2BFile(t *testing.T) {
	t.Parallel()
	SetLoggingLevel(logging.Level(*loggingLevel))

	// Load clean image
	var cleanImg StegImage
	err := cleanImg.LoadImageFromFile(cleanImageFile)
	if err != nil {
		t.Error(err)
	}

	// Embed secret
	err = cleanImg.DoStegEmbed(secretStringIn)
	if err != nil {
		t.Error(err)
	}

	// Create tmp file
	tmpTampered, err := ioutil.TempFile("", "")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(tmpTampered.Name())

	// Write manipulated image to file
	err = cleanImg.WriteNewImageToFile(tmpTampered.Name())
	if err != nil {
		t.Error(err)
	}

	var tamperedImg StegImage
	// Read it back in
	err = tamperedImg.LoadImageFromFile(tmpTampered.Name())
	if err != nil {
		t.Error(err)
	}

	var secretOut string
	secretOut, err = tamperedImg.DoStegExtract()
	if err != nil {
		t.Error(err)
	}

	if secretOut != secretStringIn {
		t.Error("Secrets Do Not Match!")
	} else if err != nil {
		t.Error(err)
	}
}

// TestB2BFile does a back to back Base64 based test
func TestB2BBase64(t *testing.T) {
	t.Parallel()
	SetLoggingLevel(logging.Level(*loggingLevel))

	imageB64Out, err := Base64Embed(CleanB64Image, secretStringIn)
	if err != nil {
		t.Error(err)
	}

	var secretOut string
	secretOut, err = Base64Extract(imageB64Out)
	if err != nil {
		t.Error(err)
	}

	if secretOut != secretStringIn {
		t.Error("Secrets Do Not Match!")
	} else if err != nil {
		t.Error(err)
	}
}

// TestCleanFileExtract tests that DoStegExtract returns err on being unable to extract a secret
func TestCleanFileExtract(t *testing.T) {
	t.Parallel()
	SetLoggingLevel(logging.Level(*loggingLevel))

	var cleanImg StegImage
	// Read in clean image
	err := cleanImg.LoadImageFromFile(cleanImageFile)
	if err != nil {
		t.Error(err)
	}

	// Attempt to get secret from clean image - expect to fail
	_, err = cleanImg.DoStegExtract()

	// Check for expected error
	if err.Error() != "error finding embedded secret string" {
		t.Error("Correct Error not thrown!")
	}
}

// TestSecretTooLarge verifies that an error is thrown when the image
// is too small to embed the secret string inside
func TestSecretTooLarge(t *testing.T) {
	t.Parallel()
	SetLoggingLevel(logging.Level(*loggingLevel))

	// Load clean image
	var cleanImg StegImage
	err := cleanImg.LoadImageFromFile(tinyImageFile)
	if err != nil {
		t.Error(err)
	}

	b := cleanImg.imgLoaded.Bounds()
	max := b.Max.X * b.Max.Y * 3
	longString := strings.Repeat("a", max+1)

	err = cleanImg.DoStegEmbed(longString)
	// Check for expected error
	expectedError := "not enough pixels to hide secret"
	if err.Error() != expectedError {
		t.Error("Correct Error not thrown, expected: '" + expectedError + "' got: '" + err.Error() + "'")
	}
}

// TestColourValuesDifference validates that the tampered image
// colour values are in the acceptable range for LSB steganography (0,1,-1)
func TestColourValuesDifference(t *testing.T) {
	t.Parallel()
	SetLoggingLevel(logging.Level(*loggingLevel))

	// Load clean image
	var cleanImg StegImage
	err := cleanImg.LoadImageFromFile(cleanImageFile)
	if err != nil {
		t.Error(err)
	}

	// Embed secret
	err = cleanImg.DoStegEmbed(secretStringIn)
	if err != nil {
		t.Error(err)
	}

	// Create tmp file
	tmpTampered, err := ioutil.TempFile("", "")
	if err != nil {
		t.Error(err)
	}
	defer os.Remove(tmpTampered.Name())

	// Write manipulated image to file
	err = cleanImg.WriteNewImageToFile(tmpTampered.Name())
	if err != nil {
		t.Error(err)
	}

	var tamperedImg StegImage
	// Read it back in
	err = tamperedImg.LoadImageFromFile(tmpTampered.Name())
	if err != nil {
		t.Error(err)
	}

	// Check that the first n rgb values are only +1/-1 difference at most
	bounds := cleanImg.imgLoaded.Bounds()
	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			r1, g1, b1, _ := cleanImg.imgLoaded.At(x, y).RGBA()
			r2, g2, b2, _ := tamperedImg.imgLoaded.At(x, y).RGBA()
			if err := checkColourDifference(uint8(r1), uint8(r2)); err != nil {
				t.Log(err)
				t.FailNow()
			}
			if err := checkColourDifference(uint8(g1), uint8(g2)); err != nil {
				t.Log(err)
				t.FailNow()
			}
			if err := checkColourDifference(uint8(b1), uint8(b2)); err != nil {
				t.Log(err)
				t.FailNow()
			}
		}
	}
}

func TestEmbedOnNoLoadedImage(t *testing.T) {
	t.Parallel()
	SetLoggingLevel(logging.Level(*loggingLevel))

	var cleanImg StegImage
	err := cleanImg.DoStegEmbed(secretStringIn)
	// Check for expected error
	expectedError := "no image loaded"
	if err.Error() != expectedError {
		t.Error("Correct Error not thrown, expected: '" + expectedError + "' got: '" + err.Error() + "'")
	}
}

// Used to store result from benchmark tests at package level so compiler doesnt
// optimise function calls out
var benchmarkRet string

func BenchmarkB64Embed(b *testing.B) {
	var imageB64Out string
	for n := 0; n < b.N; n++ {
		imageB64Out, _ = Base64Embed(CleanB64Image, secretStringIn)
	}
	benchmarkRet = imageB64Out
}

func checkColourDifference(x uint8, y uint8) (err error) {
	z := int8(x - y)

	if z == 0 || z == -1 || z == 1 {
		return nil
	}

	return fmt.Errorf("Colour difference not in acceptable range (-1,0,1) : %d", z)
}
