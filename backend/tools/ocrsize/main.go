// ocrsize reads one page or more at several sizes and renderings, in one
// endpoint job, so an operator can see what the shape of an image does to what
// an engine reads off it. Every variant goes under one prefix, so the endpoint
// loads its models once and reads them all together.
//
//	ocrsize -sizes 1536,2048 -renders color,mono -out /tmp/sizes page.jpg
//
// A size of 0 names the size the image already has, and a size of 2x names twice
// that size, so one job can ask every image for a size of its own.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"golang.org/x/image/draw"

	"github.com/themylogin/ownocr/backend/internal/config"
	"github.com/themylogin/ownocr/backend/internal/objectstore"
	"github.com/themylogin/ownocr/backend/internal/ocr"
	"github.com/themylogin/ownocr/backend/internal/runpod"
)

const jpegQuality = 85

// root keeps a probe out of the way of the prefixes the workers use.
const root = "ocrsize/"

// The renderings a probe can ask for. `color` is the image as it stands, `gray`
// drops the colour, and `mono` drops everything but the ink: it thresholds the
// page and writes one bit per pixel.
const (
	renderColor = "color"
	renderGray  = "gray"
	renderMono  = "mono"
)

func main() {
	sizes := flag.String("sizes", "1024,1536,2048",
		"longest sides to read at: pixels, 0 for the size it has, or a factor such as 2x")
	renders := flag.String("renders", renderColor, "renderings to read: color, gray, mono")
	modes := flag.String("modes", ocr.ModeVL, "engines to run, comma separated")
	out := flag.String("out", "", "directory the results are written to")
	keep := flag.Bool("keep", false, "leave the exchange objects in the bucket")
	dry := flag.Bool("dry", false, "write the variants and start no job")
	temperature := flag.Float64("temperature", -1, "VL sampling temperature, -1 to leave it unset")
	topP := flag.Float64("top-p", -1, "VL sampling top_p, -1 to leave it unset")
	flag.Parse()

	if flag.NArg() == 0 || *out == "" {
		fmt.Fprintln(os.Stderr,
			"usage: ocrsize -out <dir> [-sizes ...] [-renders ...] <image> ...")
		os.Exit(2)
	}

	err := run(flag.Args(), parseSizes(*sizes), parseRenders(*renders),
		strings.Split(*modes, ","), *out, *keep, *dry, optional(*temperature), optional(*topP))
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// size names a longest side to read at. A factor names one against the size the
// image already has, so `2x` asks every image for twice its own longest side.
type size struct {
	pixels int
	factor float64
}

// longestSideOf answers the longest side this size asks of an image.
func (s size) longestSideOf(bounds image.Rectangle) int {
	if s.factor == 0 {
		return s.pixels
	}

	longest := bounds.Dx()
	if bounds.Dy() > longest {
		longest = bounds.Dy()
	}
	return int(float64(longest)*s.factor + 0.5)
}

func parseSizes(raw string) []size {
	var sizes []size
	for _, field := range strings.Split(raw, ",") {
		field = strings.TrimSpace(field)

		if factor, ok := strings.CutSuffix(field, "x"); ok {
			value, err := strconv.ParseFloat(factor, 64)
			if err != nil || value <= 0 {
				fmt.Fprintf(os.Stderr, "not a factor: %q\n", field)
				os.Exit(2)
			}
			sizes = append(sizes, size{factor: value})
			continue
		}

		pixels, err := strconv.Atoi(field)
		if err != nil || pixels < 0 {
			fmt.Fprintf(os.Stderr, "not a size: %q\n", field)
			os.Exit(2)
		}
		sizes = append(sizes, size{pixels: pixels})
	}
	return sizes
}

func parseRenders(raw string) []string {
	var renders []string
	for _, field := range strings.Split(raw, ",") {
		render := strings.TrimSpace(field)
		switch render {
		case renderColor, renderGray, renderMono:
			renders = append(renders, render)
		default:
			fmt.Fprintf(os.Stderr, "not a rendering: %q\n", render)
			os.Exit(2)
		}
	}
	return renders
}

// optional answers the value a caller set, or nothing for the -1 that stands
// for "leave it to the endpoint".
func optional(value float64) *float64 {
	if value < 0 {
		return nil
	}
	return &value
}

func run(images []string, sizes []size, renders []string, modes []string,
	out string, keep, dry bool, temperature, topP *float64) error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}

	prefix := root + time.Now().UTC().Format("20060102-150405") + "/"
	inputPrefix, outputPrefix := prefix+"in/", prefix+"out/"

	objects := objectstore.New(objectstore.Settings(cfg.S3))
	ctx := context.Background()

	if err := objects.DeletePrefix(ctx, prefix); err != nil {
		return err
	}
	if !keep {
		defer func() {
			if err := objects.DeletePrefix(ctx, prefix); err != nil {
				fmt.Fprintln(os.Stderr, "cleanup failed:", err)
			}
		}()
	}

	names, err := upload(ctx, objects, images, sizes, renders, inputPrefix, out, dry)
	if err != nil {
		return err
	}

	if dry {
		fmt.Printf("wrote %d variants, started no job\n", len(names))
		return nil
	}

	input := ocr.JobInput{
		Bucket:          cfg.S3.Bucket,
		EndpointURL:     cfg.S3.Endpoint,
		Region:          cfg.S3.Region,
		AccessKeyID:     cfg.S3.AccessKeyID,
		SecretAccessKey: cfg.S3.SecretAccessKey,
		InputPrefix:     inputPrefix,
		OutputPrefix:    outputPrefix,
		Modes:           modes,
		Temperature:     temperature,
		TopP:            topP,
	}

	client := runpod.NewClient("OCR size probe", cfg.OCR.APIURL, cfg.OCR.APIToken)
	log := slog.Default()

	fmt.Printf("reading %d objects with %v, temperature=%s top_p=%s\n",
		len(names), modes, show(temperature), show(topP))
	started := time.Now()

	jobID, err := client.Run(ctx, input)
	if err != nil {
		return err
	}

	raw, err := client.Wait(ctx, jobID, log, func(string) {})
	if err != nil {
		return err
	}

	var answer ocr.Output
	if err := json.Unmarshal(raw, &answer); err != nil {
		return err
	}

	fmt.Printf("read %d, failed %d, took %s\n", len(answer.Pages), len(answer.Failed),
		time.Since(started).Round(time.Second))
	for _, failure := range answer.Failed {
		fmt.Printf("  FAILED %s: %s\n", failure.InputKey, failure.Error)
	}

	return download(ctx, objects, answer.Pages, out)
}

// upload writes every image at every size and rendering, and puts it under the
// input prefix. The name carries both, so a result names what it came from.
func upload(ctx context.Context, objects *objectstore.Store, images []string, sizes []size,
	renders []string, prefix, out string, dry bool) ([]string, error) {
	var names []string

	for _, path := range images {
		source, err := decode(path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}

		stem := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
		bounds := source.Bounds()
		fmt.Printf("%s: %dx%d\n", stem, bounds.Dx(), bounds.Dy())

		for _, want := range sizes {
			longestSide := want.longestSideOf(bounds)
			scaled := scale(source, longestSide)

			for _, render := range renders {
				name := fmt.Sprintf("%s-%04d-%s.%s", stem, longestSide, render, extOf(render))
				local := filepath.Join(out, name)

				if err := write(scaled, render, local); err != nil {
					return nil, err
				}
				if !dry {
					if err := objects.PutFile(ctx, prefix+name, local); err != nil {
						return nil, err
					}
				}

				box := scaled.Bounds()
				fmt.Printf("  %-5d %-5s -> %dx%d\n", longestSide, render, box.Dx(), box.Dy())
				names = append(names, name)
			}
		}
	}

	return names, nil
}

// A thresholded page holds two tones, which JPEG would blur back into many.
func extOf(render string) string {
	if render == renderMono {
		return "png"
	}
	return "jpg"
}

func write(img image.Image, render, path string) error {
	switch render {
	case renderGray:
		return writeJPEG(toGray(img), path)
	case renderMono:
		return writePNG(toMono(img), path)
	default:
		return writeJPEG(img, path)
	}
}

func toGray(img image.Image) image.Image {
	bounds := img.Bounds()
	gray := image.NewGray(bounds)
	draw.Draw(gray, bounds, img, bounds.Min, draw.Src)
	return gray
}

// toMono thresholds the page into ink and paper, one bit a pixel. The threshold
// is the one that parts the two tones of the page best, which a scan of white
// paper and black type separates well.
func toMono(img image.Image) image.Image {
	gray := toGray(img).(*image.Gray)
	cut := otsu(gray)

	bounds := gray.Bounds()
	mono := image.NewPaletted(bounds, color.Palette{color.Gray{Y: 0}, color.Gray{Y: 255}})
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			if gray.GrayAt(x, y).Y > cut {
				mono.SetColorIndex(x, y, 1)
			}
		}
	}
	return mono
}

// otsu answers the grey level that splits the page into two tones with the
// least spread inside each.
func otsu(gray *image.Gray) uint8 {
	var histogram [256]int
	for _, value := range gray.Pix {
		histogram[value]++
	}

	total := len(gray.Pix)
	sum := 0.0
	for level, count := range histogram {
		sum += float64(level * count)
	}

	best, bestVariance := uint8(127), -1.0
	below, belowSum := 0, 0.0
	for level := 0; level < 256; level++ {
		below += histogram[level]
		if below == 0 {
			continue
		}
		above := total - below
		if above == 0 {
			break
		}

		belowSum += float64(level * histogram[level])
		belowMean := belowSum / float64(below)
		aboveMean := (sum - belowSum) / float64(above)

		spread := float64(below) * float64(above) * (belowMean - aboveMean) * (belowMean - aboveMean)
		if spread > bestVariance {
			best, bestVariance = uint8(level), spread
		}
	}

	return best
}

// scale answers img with its longest side at longestSide. It enlarges as well
// as shrinks: a probe asks what a size does, including a size above the one the
// page was scanned at. A longestSide of 0 leaves the image as it is.
func scale(img image.Image, longestSide int) image.Image {
	if longestSide == 0 {
		return img
	}

	bounds := img.Bounds()
	width, height := bounds.Dx(), bounds.Dy()

	targetWidth, targetHeight := longestSide, longestSide
	if width >= height {
		targetHeight = (height*longestSide + width/2) / width
	} else {
		targetWidth = (width*longestSide + height/2) / height
	}

	if targetWidth == width && targetHeight == height {
		return img
	}

	dst := image.NewRGBA(image.Rect(0, 0, targetWidth, targetHeight))
	draw.CatmullRom.Scale(dst, dst.Bounds(), img, bounds, draw.Src, nil)
	return dst
}

func decode(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	return img, err
}

func writeJPEG(img image.Image, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := jpeg.Encode(file, img, &jpeg.Options{Quality: jpegQuality}); err != nil {
		return err
	}
	return file.Close()
}

func writePNG(img image.Image, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	if err := png.Encode(file, img); err != nil {
		return err
	}
	return file.Close()
}

// download writes the text of every result beside the image it was read from.
func download(ctx context.Context, objects *objectstore.Store, pages []ocr.OutputPage,
	out string) error {
	for _, page := range pages {
		textKey, ok := page.TextKey()
		if !ok {
			fmt.Printf("  %s: no text\n", page.InputKey)
			continue
		}

		stem := strings.TrimSuffix(filepath.Base(page.InputKey), filepath.Ext(page.InputKey))
		local := filepath.Join(out, fmt.Sprintf("%s.%s.md", stem, page.Mode()))
		if err := objects.GetFile(ctx, textKey, local); err != nil {
			return err
		}
		fmt.Printf("  %s\n", local)
	}
	return nil
}

// show names a sampling value for the log, or the endpoint's own default.
func show(value *float64) string {
	if value == nil {
		return "default"
	}
	return strconv.FormatFloat(*value, 'g', -1, 64)
}
