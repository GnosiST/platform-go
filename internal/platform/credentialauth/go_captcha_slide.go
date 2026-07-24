package credentialauth

import (
	"bytes"
	"embed"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"strconv"
	"strings"

	"github.com/wenlng/go-captcha/v2/base/option"
	"github.com/wenlng/go-captcha/v2/slide"
	"golang.org/x/image/draw"
)

const (
	defaultGoCaptchaSlideImageWidth   = 720
	defaultGoCaptchaSlideImageHeight  = 160
	defaultGoCaptchaSlideGraphSizeMin = 52
	defaultGoCaptchaSlideGraphSizeMax = 60
	goCaptchaSlideTileSourceSize      = 96
)

//go:embed assets/slide/*.jpg
var goCaptchaSlideBackgroundFS embed.FS

var goCaptchaSlideBackgroundNames = []string{
	"assets/slide/greenway.jpg",
	"assets/slide/lakeside.jpg",
}

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
	backgrounds, err := goCaptchaSlideBackgrounds(options.ImageWidth, options.ImageHeight)
	if err != nil {
		return nil, err
	}
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

func goCaptchaSlideBackgrounds(width int, height int) ([]image.Image, error) {
	backgrounds := make([]image.Image, 0, len(goCaptchaSlideBackgroundNames))
	for _, name := range goCaptchaSlideBackgroundNames {
		data, err := goCaptchaSlideBackgroundFS.ReadFile(name)
		if err != nil {
			return nil, fmt.Errorf("load go-captcha slide background %q: %w", name, err)
		}
		decoded, err := jpeg.Decode(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("decode go-captcha slide background %q: %w", name, err)
		}
		backgrounds = append(backgrounds, resizeGoCaptchaSlideBackground(decoded, width, height))
	}
	return backgrounds, nil
}

func resizeGoCaptchaSlideBackground(source image.Image, width int, height int) image.Image {
	sourceBounds := source.Bounds()
	crop := sourceBounds
	if sourceBounds.Dx()*height > sourceBounds.Dy()*width {
		cropWidth := sourceBounds.Dy() * width / height
		crop.Min.X += (sourceBounds.Dx() - cropWidth) / 2
		crop.Max.X = crop.Min.X + cropWidth
	} else {
		cropHeight := sourceBounds.Dx() * height / width
		crop.Min.Y += (sourceBounds.Dy() - cropHeight) / 2
		crop.Max.Y = crop.Min.Y + cropHeight
	}
	destination := image.NewNRGBA(image.Rect(0, 0, width, height))
	draw.CatmullRom.Scale(destination, destination.Bounds(), source, crop, draw.Src, nil)
	return destination
}

func goCaptchaSlideGraphImages() []*slide.GraphImage {
	return []*slide.GraphImage{{
		OverlayImage: goCaptchaSlideTileImage(color.NRGBA{R: 255, G: 255, B: 255, A: 36}, color.NRGBA{R: 255, G: 255, B: 255, A: 232}),
		ShadowImage:  goCaptchaSlideTargetImage(),
		MaskImage:    goCaptchaSlideMaskImage(),
	}}
}

func goCaptchaSlideTargetImage() image.Image {
	img := image.NewNRGBA(image.Rect(0, 0, goCaptchaSlideTileSourceSize, goCaptchaSlideTileSourceSize))
	for y := 0; y < goCaptchaSlideTileSourceSize; y++ {
		for x := 0; x < goCaptchaSlideTileSourceSize; x++ {
			if !goCaptchaSlideShapeContains(x, y, goCaptchaSlideTileSourceSize) {
				continue
			}
			if goCaptchaSlideShapeEdgeWidth(x, y, goCaptchaSlideTileSourceSize, 3) {
				img.SetNRGBA(x, y, color.NRGBA{R: 255, G: 255, B: 255, A: 232})
				continue
			}
			img.SetNRGBA(x, y, color.NRGBA{R: 9, G: 20, B: 30, A: 156})
		}
	}
	return img
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
	return goCaptchaSlideShapeEdgeWidth(x, y, size, 1)
}

func goCaptchaSlideShapeEdgeWidth(x int, y int, size int, width int) bool {
	for offsetY := -width; offsetY <= width; offsetY++ {
		for offsetX := -width; offsetX <= width; offsetX++ {
			if !goCaptchaSlideShapeContains(x+offsetX, y+offsetY, size) {
				return true
			}
		}
	}
	return false
}

func insideCircle(x int, y int, cx int, cy int, radius int) bool {
	dx := x - cx
	dy := y - cy
	return dx*dx+dy*dy <= radius*radius
}
