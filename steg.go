package libsteg

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"os"
	"strconv"
	"strings"

	"github.com/op/go-logging"
)

const (
	// stopStegConst defines where the implanted message ends
	stopStegConst string = "##STOP_STEG##"
)

// StegImage type holds all vars needed for image manipulation
type StegImage struct {
	imgLoaded  image.Image
	imgType    string
	secretBits []int // Splice of ints for bits of secret
	newImg     *image.RGBA
}

// By default set the logger to only log CRITICAL level messages
var log = setupLogging(logging.CRITICAL)

// SetLoggingLevel sets the libsteg logging level
func SetLoggingLevel(level logging.Level) {
	logging.SetLevel(level, "libsteg")
}

// setupLogging initialises the libsteg logger
func setupLogging(level logging.Level) *logging.Logger {
	logger := logging.MustGetLogger("libsteg")
	format := logging.MustStringFormatter(
		`%{color}%{time:15:04:05.000} %{shortfunc} ▶ %{level:.4s} %{id:03x}%{color:reset} %{message}`,
	)

	backend1 := logging.NewLogBackend(os.Stdout, "", 0)
	backend1Formatter := logging.NewBackendFormatter(backend1, format)
	backend1Leveled := logging.AddModuleLevel(backend1Formatter)
	backend1Leveled.SetLevel(level, "")
	logging.SetBackend(backend1Leveled)

	return logger
}

// Base64Embed performs a full base64 based embed
func Base64Embed(imageB64In string, secret string) (imageB64Out string, err error) {
	// Load clean image
	var cleanImg StegImage
	err = cleanImg.LoadImageFromB64(imageB64In)
	if err != nil {
		log.Error(err)
		return "", err
	}

	// Embed secret
	err = cleanImg.DoStegEmbed(secret)
	if err != nil {
		log.Error(err)
		return "", err
	}

	// Write manipulated image to Base64
	imageB64Out, err = cleanImg.WriteNewImageToB64()
	if err != nil {
		log.Error(err)
		return "", err
	}
	return imageB64Out, nil
}

// Base64Extract performs a full base64 based extract
func Base64Extract(imageB64In string) (secret string, err error) {
	// Load tampered image
	var tamperedImg StegImage
	if err = tamperedImg.LoadImageFromB64(imageB64In); err != nil {
		log.Error(err)
		return "", err
	}

	secret, err = tamperedImg.DoStegExtract()
	if err != nil {
		log.Error(err)
		return "", err
	}

	return secret, nil
}

// LoadImageFromFile loads the given image into the StegImage structure
func (s *StegImage) LoadImageFromFile(imgPath string) error {
	reader, err := os.Open(imgPath)
	if err != nil {
		log.Error(err)
		return err
	}
	defer reader.Close()

	// Read into an image
	s.imgLoaded, s.imgType, err = image.Decode(reader)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Notice("Image Type Loaded:", s.imgType)
	return nil
}

// LoadImageFromB64 loads the given base64 encoded image into the
// StegImage structure
func (s *StegImage) LoadImageFromB64(b64Img string) (err error) {
	// Load image file from base 64 string
	reader := base64.NewDecoder(base64.StdEncoding, strings.NewReader(b64Img))

	// Read into an image
	s.imgLoaded, s.imgType, err = image.Decode(reader)
	if err != nil {
		log.Error(err)
		return err
	}
	log.Notice("Image Type Loaded:", s.imgType)
	return nil
}

// WriteNewImageToFile outputs the image held in StegImage.newImg to
// the imgPath given
func (s *StegImage) WriteNewImageToFile(imgPath string) (err error) {
	myfile, _ := os.Create(imgPath)
	defer myfile.Close()

	enc := png.Encoder{CompressionLevel: png.BestCompression}
	err = enc.Encode(myfile, s.newImg)
	if err != nil {
		log.Error(err)
	}

	return err
}

// WriteNewImageToB64 base64 encodes the image held in StegImage.newImg and
// returns it as a string
func (s *StegImage) WriteNewImageToB64() (b64Img string, err error) {
	buf := new(bytes.Buffer)
	err = png.Encode(buf, s.newImg)
	if err != nil {
		return "", err
	}
	b64Img = base64.StdEncoding.EncodeToString(buf.Bytes())
	return b64Img, err
}

// DoStegExtract retrieves the embedded secret from the loaded image
func (s *StegImage) DoStegExtract() (secretOut string, err error) {
	secretOut, err = s.getSecretString()
	if err != nil {
		log.Error(err)
		return "", err
	}
	log.Info("Secret Found:", secretOut)

	return secretOut, err
}

// DoStegEmbed embeds the given secret into the loaded image
func (s *StegImage) DoStegEmbed(secretIn string) (err error) {
	err = s.createMutableImage()
	if err != nil {
		log.Error(err)
		return err
	}

	err = s.loadSecret(secretIn)
	if err != nil {
		log.Error(err)
		return err
	}

	err = s.embedSecret()
	if err != nil {
		log.Error(err)
		return err
	}

	return err
}

func (s *StegImage) createMutableImage() error {
	// Ensure we have an image Loaded
	if s.imgLoaded == nil {
		return errors.New("no image loaded")
	}
	s.newImg = image.NewRGBA(s.imgLoaded.Bounds())
	draw.Draw(s.newImg, s.newImg.Bounds(), s.imgLoaded,
		s.imgLoaded.Bounds().Min, draw.Over)
	return nil
}

func (s *StegImage) loadSecret(secret string) (err error) {
	log.Notice("Loaded secret", secret)
	secretWordBin := stringToBinary(secret + stopStegConst)
	log.Debug("Secret " + secret + " in binary: " + secretWordBin)
	secretWordBinList := strings.Split(secretWordBin, "")
	s.secretBits, err = stringsToInts(secretWordBinList)
	return err
}

func (s *StegImage) embedSecret() (err error) {
	secretIndex := 0
	bounds := s.newImg.Bounds()

	// Check if we can store the secret message
	if len(s.secretBits) > (bounds.Max.X * bounds.Max.Y * 3) {
		return errors.New("not enough pixels to hide secret")
	}

	for x := bounds.Min.X; x < bounds.Max.X; x++ {
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			if secretIndex < len(s.secretBits) {
				r, g, b, a := s.newImg.At(x, y).RGBA()

				newR := manipulateLSB(int(r), s.secretBits[secretIndex])
				secretIndex++

				newG := uint8(g)
				if secretIndex < len(s.secretBits) {
					newG = manipulateLSB(int(g), s.secretBits[secretIndex])
					secretIndex++
				}

				newB := uint8(b)
				if secretIndex < len(s.secretBits) {
					newB = manipulateLSB(int(b), s.secretBits[secretIndex])
					secretIndex++
				}
				// Set new Color
				s.newImg.SetRGBA(x, y,
					color.RGBA{newR, newG, newB, uint8(a / 0x101)})
			} else {
				break
			}
		}
		if secretIndex >= len(s.secretBits) {
			break
		}
	}
	return nil
}

func getBitsFromRGB(values ...uint32) (bitsOut []int) {
	for _, v := range values {
		if v%2 == 0 {
			bitsOut = append(bitsOut, 0)
		} else {
			bitsOut = append(bitsOut, 1)
		}
	}
	return bitsOut
}

func (s *StegImage) getSecretString() (secret string, err error) {
	bounds := s.imgLoaded.Bounds()
	bitsOut := make([]int, 0, bounds.Max.X*bounds.Max.Y)
	for x := bounds.Min.X; x <= bounds.Max.X; x++ {
		for y := bounds.Min.Y; y <= bounds.Max.Y; y++ {
			r, g, b, _ := s.imgLoaded.At(x, y).RGBA()
			bitsOut = append(bitsOut, getBitsFromRGB(r, g, b)...)
		}
	}

	tmp := bitsToString(bitsOut)
	if strings.Contains(tmp, stopStegConst) {
		secret = strings.Split(tmp, stopStegConst)[0]
		log.Info("Secret string:", secret)
		return secret, nil
	}

	return secret, errors.New("error finding embedded secret string")
}

func bitsToString(bits []int) (out string) {
	for i := 0; i < len(bits); i += 8 {
		batch := bits[i : i+8]
		out = out + getCharFromBits(batch)
	}
	return out
}

func getCharFromBits(bits []int) (char string) {
	tmp := 0
	for b := 0; b < 8; b++ {
		n := bits[b]
		if n == 1 {
			// Set bit
			tmp = tmp | (1 << (7 - uint(b)))
		}
	}
	char = string(tmp)
	return char
}

func manipulateLSB(i int, bit int) (newI uint8) {
	newI = uint8(i)
	log.Debug("Int Before:", newI, stringToBinary(string(newI)))
	log.Debug("Value to Set LSB to:", bit)
	if bit == 0 {
		// Unset LSB
		newI = newI &^ (1 << 0)
	} else {
		// Set LSB
		newI = newI | (1 << 0)
	}
	log.Debug("Int After:", newI, stringToBinary(string(newI)))
	return newI
}

func stringToBinary(s string) (res string) {
	for _, c := range s {
		res = fmt.Sprintf("%s%.8b", res, c)
	}
	return res
}

func stringsToInts(s []string) (ints []int, err error) {
	for _, i := range s {
		var j int
		j, err = strconv.Atoi(i)
		if err != nil {
			log.Error(err)
			break
		}
		ints = append(ints, j)
	}
	return ints, err
}
