package credentialauth

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"strconv"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"
)

const (
	textCaptchaImageWidth  = 180
	textCaptchaImageHeight = 64
)

func defaultTextCaptchaMaterial() (ChallengeMaterial, error) {
	proof, err := randomChallengeToken(6)
	if err != nil {
		return ChallengeMaterial{}, err
	}
	imageData, err := renderTextCaptchaImage(proof)
	if err != nil {
		return ChallengeMaterial{}, err
	}
	return ChallengeMaterial{
		Prompt: "Enter the displayed verification text.",
		Parameters: map[string]string{
			"image":       imageData,
			"imageWidth":  strconv.Itoa(textCaptchaImageWidth),
			"imageHeight": strconv.Itoa(textCaptchaImageHeight),
			"proofFormat": "text",
		},
		Proof: proof,
	}, nil
}

func renderTextCaptchaImage(text string) (string, error) {
	img := image.NewNRGBA(image.Rect(0, 0, textCaptchaImageWidth, textCaptchaImageHeight))
	draw.Draw(img, img.Bounds(), image.NewUniform(color.NRGBA{R: 248, G: 249, B: 250, A: 255}), image.Point{}, draw.Src)
	for y := 0; y < textCaptchaImageHeight; y++ {
		for x := 0; x < textCaptchaImageWidth; x++ {
			if (x+y)%17 == 0 || (x*3+y*2)%43 == 0 {
				img.SetNRGBA(x, y, color.NRGBA{R: 176, G: 196, B: 208, A: 180})
			}
		}
	}
	for x := -textCaptchaImageHeight; x < textCaptchaImageWidth; x += 28 {
		for y := 0; y < textCaptchaImageHeight; y++ {
			px := x + y
			if px >= 0 && px < textCaptchaImageWidth {
				img.SetNRGBA(px, y, color.NRGBA{R: 205, G: 214, B: 220, A: 220})
			}
		}
	}
	drawer := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(color.NRGBA{R: 33, G: 49, B: 57, A: 255}),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(48, 38),
	}
	drawer.DrawString(text)
	var buffer bytes.Buffer
	if err := png.Encode(&buffer, img); err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(buffer.Bytes()), nil
}
