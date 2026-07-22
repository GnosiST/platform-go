package credentialauth

import (
	"fmt"
	"image"
	"image/color"
	"strconv"
	"strings"

	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/slide"
)

const (
	defaultGoCaptchaSlideImageWidth   = 300
	defaultGoCaptchaSlideImageHeight  = 220
	defaultGoCaptchaSlideGraphSizeMin = 60
	defaultGoCaptchaSlideGraphSizeMax = 70
	goCaptchaSlideTileSourceSize      = 96
)

type GoCaptchaSlideOptions struct {
	ImageWidth   int
	ImageHeight  int
	GraphSizeMin int
	GraphSizeMax int
}

func NewGoCaptchaSlideMaterialGenerator(options GoCaptchaSlideOptions) (func(ChallengeKind) (ChallengeMaterial, error), error) {
	options, err := normalizeGoCaptchaSlideOptions(options)
	if err != nil {
		return nil, err
	}
	backgrounds := goCaptchaSlideBackgrounds(options.ImageWidth, options.ImageHeight)
	graphs := goCaptchaSlideGraphImages()
	return func(kind ChallengeKind) (ChallengeMaterial, error) {
		kind = ChallengeKind(strings.TrimSpace(string(kind)))
		if kind != ChallengeKindSlider {
			return ChallengeMaterial{}, fmt.Errorf("%w: go-captcha slide only supports slider challenges", ErrInvalidInput)
		}
		builder := slide.NewBuilder(
			slide.WithImageSize(option.Size{Width: options.ImageWidth, Height: options.ImageHeight}),
			slide.WithRangeGraphSize(option.RangeVal{Min: options.GraphSizeMin, Max: options.GraphSizeMax}),
			slide.WithRangeGraphAnglePos([]option.RangeVal{{Min: 0, Max: 0}}),
			slide.WithRangeDeadZoneDirections([]slide.DeadZoneDirectionType{slide.DeadZoneDirectionTypeLeft}),
		)
		builder.SetResources(
			slide.WithBackgrounds(backgrounds),
			slide.WithGraphImages(graphs),
		)
		data, err := builder.Make().Generate()
		if err != nil {
			return ChallengeMaterial{}, err
		}
		block := data.GetData()
		if block == nil {
			return ChallengeMaterial{}, fmt.Errorf("%w: go-captcha slide block is empty", ErrRepositoryInvariant)
		}
		masterImage, err := data.GetMasterImage().ToBase64()
		if err != nil {
			return ChallengeMaterial{}, err
		}
		tileImage, err := data.GetTileImage().ToBase64()
		if err != nil {
			return ChallengeMaterial{}, err
		}
		return ChallengeMaterial{
			Prompt: "Complete the slide verification.",
			Parameters: map[string]string{
				"provider":       "go-captcha",
				"mode":           "slide",
				"masterImage":    masterImage,
				"tileImage":      tileImage,
				"imageWidth":     strconv.Itoa(options.ImageWidth),
				"imageHeight":    strconv.Itoa(options.ImageHeight),
				"tileWidth":      strconv.Itoa(block.Width),
				"tileHeight":     strconv.Itoa(block.Height),
				"tileX":          strconv.Itoa(block.DX),
				"tileY":          strconv.Itoa(block.Y),
				"initialX":       strconv.Itoa(block.DX),
				"xTolerance":     strconv.Itoa(sliderChallengeProofXTolerance),
				"coordinateUnit": "px",
				"proofFormat":    "x=<targetX>&y=<targetY>",
			},
			Proof: FormatSlideChallengeProof(block.X, block.Y),
		}, nil
	}, nil
}

func FormatSlideChallengeProof(x int, y int) string {
	return fmt.Sprintf("x=%d&y=%d", x, y)
}

func normalizeGoCaptchaSlideOptions(options GoCaptchaSlideOptions) (GoCaptchaSlideOptions, error) {
	if options.ImageWidth == 0 {
		options.ImageWidth = defaultGoCaptchaSlideImageWidth
	}
	if options.ImageHeight == 0 {
		options.ImageHeight = defaultGoCaptchaSlideImageHeight
	}
	if options.GraphSizeMin == 0 {
		options.GraphSizeMin = defaultGoCaptchaSlideGraphSizeMin
	}
	if options.GraphSizeMax == 0 {
		options.GraphSizeMax = defaultGoCaptchaSlideGraphSizeMax
	}
	if options.ImageWidth < 160 || options.ImageHeight < 120 {
		return GoCaptchaSlideOptions{}, fmt.Errorf("%w: go-captcha slide image size is too small", ErrInvalidInput)
	}
	if options.GraphSizeMin < 24 || options.GraphSizeMax < options.GraphSizeMin {
		return GoCaptchaSlideOptions{}, fmt.Errorf("%w: go-captcha slide graph size is invalid", ErrInvalidInput)
	}
	if options.GraphSizeMax >= options.ImageHeight-20 || options.GraphSizeMax >= options.ImageWidth/2 {
		return GoCaptchaSlideOptions{}, fmt.Errorf("%w: go-captcha slide graph size does not fit image size", ErrInvalidInput)
	}
	return options, nil
}

func goCaptchaSlideBackgrounds(width int, height int) []image.Image {
	return []image.Image{
		goCaptchaSlideBackground(width, height, color.NRGBA{R: 245, G: 248, B: 250, A: 255}, color.NRGBA{R: 187, G: 218, B: 214, A: 255}),
		goCaptchaSlideBackground(width, height, color.NRGBA{R: 246, G: 244, B: 239, A: 255}, color.NRGBA{R: 204, G: 190, B: 170, A: 255}),
	}
}

func goCaptchaSlideBackground(width int, height int, first color.NRGBA, second color.NRGBA) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			ratio := float64(x+y) / float64(width+height)
			base := blendNRGBA(first, second, ratio)
			if (x/14+y/18)%2 == 0 {
				base = blendNRGBA(base, color.NRGBA{R: 255, G: 255, B: 255, A: 255}, 0.18)
			}
			img.SetNRGBA(x, y, base)
		}
	}
	for x := -height; x < width; x += 36 {
		for y := 0; y < height; y++ {
			px := x + y/2
			if px >= 0 && px < width {
				img.SetNRGBA(px, y, color.NRGBA{R: 255, G: 255, B: 255, A: 70})
			}
		}
	}
	return img
}

func goCaptchaSlideGraphImages() []*slide.GraphImage {
	return []*slide.GraphImage{{
		OverlayImage: goCaptchaSlideTileImage(color.NRGBA{R: 255, G: 255, B: 255, A: 160}, color.NRGBA{R: 47, G: 82, B: 97, A: 170}),
		ShadowImage:  goCaptchaSlideTileImage(color.NRGBA{R: 18, G: 32, B: 39, A: 90}, color.NRGBA{R: 18, G: 32, B: 39, A: 120}),
		MaskImage:    goCaptchaSlideMaskImage(),
	}}
}

func goCaptchaSlideMaskImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, goCaptchaSlideTileSourceSize, goCaptchaSlideTileSourceSize))
	for y := 0; y < goCaptchaSlideTileSourceSize; y++ {
		for x := 0; x < goCaptchaSlideTileSourceSize; x++ {
			if goCaptchaSlideShapeContains(x, y, goCaptchaSlideTileSourceSize) {
				img.SetNRGBA(x, y, color.NRGBA{A: 255})
			}
		}
	}
	return img
}

func goCaptchaSlideTileImage(fill color.NRGBA, stroke color.NRGBA) image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, goCaptchaSlideTileSourceSize, goCaptchaSlideTileSourceSize))
	for y := 0; y < goCaptchaSlideTileSourceSize; y++ {
		for x := 0; x < goCaptchaSlideTileSourceSize; x++ {
			if !goCaptchaSlideShapeContains(x, y, goCaptchaSlideTileSourceSize) {
				continue
			}
			if goCaptchaSlideShapeEdge(x, y, goCaptchaSlideTileSourceSize) {
				img.SetNRGBA(x, y, stroke)
				continue
			}
			img.SetNRGBA(x, y, fill)
		}
	}
	return img
}

func goCaptchaSlideShapeContains(x int, y int, size int) bool {
	margin := size / 6
	radius := size / 7
	base := x >= margin && x < size-margin && y >= margin && y < size-margin
	topKnob := insideCircle(x, y, size/2, margin, radius)
	rightKnob := insideCircle(x, y, size-margin, size/2, radius)
	leftNotch := insideCircle(x, y, margin, size/2, radius)
	return (base || topKnob || rightKnob) && !leftNotch
}

func goCaptchaSlideShapeEdge(x int, y int, size int) bool {
	for _, point := range [][2]int{{x - 1, y}, {x + 1, y}, {x, y - 1}, {x, y + 1}} {
		if !goCaptchaSlideShapeContains(point[0], point[1], size) {
			return true
		}
	}
	return false
}

func insideCircle(x int, y int, cx int, cy int, radius int) bool {
	dx := x - cx
	dy := y - cy
	return dx*dx+dy*dy <= radius*radius
}

func blendNRGBA(first color.NRGBA, second color.NRGBA, ratio float64) color.NRGBA {
	if ratio < 0 {
		ratio = 0
	}
	if ratio > 1 {
		ratio = 1
	}
	return color.NRGBA{
		R: uint8(float64(first.R)*(1-ratio) + float64(second.R)*ratio),
		G: uint8(float64(first.G)*(1-ratio) + float64(second.G)*ratio),
		B: uint8(float64(first.B)*(1-ratio) + float64(second.B)*ratio),
		A: uint8(float64(first.A)*(1-ratio) + float64(second.A)*ratio),
	}
}
