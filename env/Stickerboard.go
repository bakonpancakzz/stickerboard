package env

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"golang.org/x/image/draw"
	"golang.org/x/image/webp"
)

const (
	CANVAS_WIDTH              = 854 // Canvas Width
	CANVAS_HEIGHT             = 480 // Canvas Height
	CANVAS_STICKER_MAX_HEIGHT = 240 // Canvas Max Sticker Height in Pixels
	CANVAS_FRAMES             = 100
	CANVAS_FPS                = 20
	CANVAS_DELAY              = 100 / CANVAS_FPS
)

var (
	StickerboardReady atomic.Bool
	StickerboardPath  = path.Join(DATA_DIRECTORY, STICKERBOARD_FILENAME)
	StickerboardBack  *image.RGBA
	StickerboardMtx   sync.RWMutex
	Stickerboard      []byte
)

func init() {
	// Initialize Background to Black to Prevent Coalescing
	base := image.NewRGBA(image.Rect(0, 0, CANVAS_WIDTH, CANVAS_HEIGHT))
	for x := 0; x < CANVAS_WIDTH; x++ {
		for y := 0; y < CANVAS_HEIGHT; y++ {
			base.Set(x, y, color.Black)
		}
	}
	StickerboardBack = base

	// Load Custom Background (if any)
	p := path.Join(DATA_DIRECTORY, "background.png")
	if f, err := os.Open(p); err == nil {
		if o, err := png.Decode(f); err == nil {
			b := base.Bounds()
			draw.CatmullRom.Scale(base, b.Bounds(), o, o.Bounds(), draw.Over, nil)
		}
		f.Close()
	}
}

func stickerboardCopy() {
	b, err := os.ReadFile(StickerboardPath)
	if err != nil {
		log.Println("[stickerboard] Read Image Error:", err)
		return
	}
	StickerboardMtx.Lock()
	Stickerboard = b
	StickerboardReady.Store(true)
	StickerboardMtx.Unlock()
}

func StickerboardRender() (int, error) {
	t := time.Now()

	// Mass Decode and Resizing of all Stickers
	type DecodedSticker struct {
		Position image.Rectangle
		Frames   []*image.RGBA
		Delays   []int
	}
	stickers := make([]DecodedSticker, len(Database.Stickers))

	DatabaseMtx.RLock()
	if err := Multithread(len(stickers), func(i int) error {

		// Read Sticker from Disk
		info := &Database.Stickers[i]
		path := path.Join(DATA_DIRECTORY, info.ImageHash)
		reader, err := os.Open(path)
		if err != nil {
			return err
		}
		defer reader.Close()
		var images = make([]image.Image, 1)
		var delays = make([]int, 1)

		// Decode Sticker Frames
		var decodeGIF *gif.GIF
		var decodeImage image.Image
		var decodeError error
		switch info.ImageType {
		case IMAGE_GIF:
			decodeGIF, decodeError = gif.DecodeAll(reader)
		case IMAGE_WEBP:
			decodeImage, decodeError = webp.Decode(reader)
		case IMAGE_JPEG:
			decodeImage, decodeError = jpeg.Decode(reader)
		case IMAGE_PNG:
			decodeImage, decodeError = png.Decode(reader)
		default:
			return fmt.Errorf("decoder available")
		}
		if decodeError != nil {
			return err
		}
		if info.ImageType == IMAGE_GIF {

			// This section here properly layers GIF frames

			delays = decodeGIF.Delay
			images = make([]image.Image, len(decodeGIF.Image))

			var imageBase = image.NewRGBA(image.Rect(0, 0, decodeGIF.Config.Width, decodeGIF.Config.Height))
			var imagePrev *image.RGBA
			var disposal = byte(gif.DisposalNone)

			for i, frame := range decodeGIF.Image {
				if disposal == gif.DisposalPrevious {
					imagePrev = image.NewRGBA(imageBase.Bounds())
					copy(imageBase.Pix, imagePrev.Pix)
				}
				if i > 0 {
					switch disposal {
					case gif.DisposalBackground:
						draw.Draw(imageBase, decodeGIF.Image[i-1].Bounds(), image.Transparent, image.Point{}, draw.Src)
					case gif.DisposalPrevious:
						if imagePrev != nil {
							draw.Draw(imageBase, imageBase.Bounds(), imagePrev, image.Point{}, draw.Src)
						}
					}
				}
				// Save Composited Frame
				draw.Draw(imageBase, frame.Bounds(), frame, frame.Bounds().Min, draw.Over)
				imageCopy := image.NewRGBA(imageBase.Bounds())
				copy(imageCopy.Pix, imageBase.Pix)

				images[i] = imageCopy
				disposal = decodeGIF.Disposal[i]
			}

		} else {
			// Copy Static Frame
			images[0] = decodeImage
		}

		// Resize Decoded Frames
		var (
			stickerFrames   = make([]*image.RGBA, len(images))
			stickerPosition image.Rectangle
			stickerBounds   = images[0].Bounds()
			stickerWidth    = int(float64(stickerBounds.Dx()) * info.ImageScale)
			stickerHeight   = int(float64(stickerBounds.Dy()) * info.ImageScale)
		)
		for j := range images {
			// Nice and Smooth Scaling
			source := images[j]
			scaled := image.NewRGBA(image.Rect(0, 0, stickerWidth, stickerHeight))
			draw.CatmullRom.Scale(scaled, scaled.Bounds(), source, source.Bounds(), draw.Over, nil)

			// Invert Y position because browsers placment origin is bottom-left but server is top-left
			y := CANVAS_HEIGHT - info.OffsetY - stickerHeight
			stickerPosition = image.Rect(info.OffsetX, y, info.OffsetX+stickerWidth, y+stickerHeight)
			stickerFrames[j] = scaled
		}

		// Store Resized Frame
		stickers[i] = DecodedSticker{
			Frames:   stickerFrames,
			Delays:   delays,
			Position: stickerPosition,
		}
		return nil

	}); err != nil {
		DatabaseMtx.RUnlock()
		return 0, err
	}
	DatabaseMtx.RUnlock()

	// Startup Encoder
	var outputLogs bytes.Buffer
	cmd := exec.Command(
		"ffmpeg", "-y",
		"-threads", fmt.Sprint(runtime.NumCPU()),
		"-f", "rawvideo",
		"-pix_fmt", "rgba",
		"-s", fmt.Sprintf("%dx%d", CANVAS_WIDTH, CANVAS_HEIGHT),
		"-framerate", fmt.Sprint(CANVAS_FPS),
		"-i", "pipe:0",
		"-vcodec", "libwebp",
		"-compression_level", "4",
		"-q:v", "75",
		"-loop", "0",
		StickerboardPath,
	)
	cmdStdin, err := cmd.StdinPipe()
	if err != nil {
		return 0, err
	}
	cmd.Stdout = &outputLogs
	cmd.Stderr = &outputLogs
	if err := cmd.Start(); err != nil {
		return 0, err
	}

	// Generate Frames
	for i := 0; i < CANVAS_FRAMES; i++ {

		// Generate Frame
		canvas := image.NewRGBA(StickerboardBack.Rect)
		copy(canvas.Pix, StickerboardBack.Pix)
		for j := range stickers {
			decode := &stickers[j]
			index := 0
			if len(decode.Frames) > 1 {
				offset := 0
				for {
					idx := index % len(decode.Frames)
					offset += decode.Delays[idx]
					if offset > (i * CANVAS_DELAY) {
						index = idx
						break
					}
					index++
				}
			}
			draw.Draw(canvas, decode.Position, decode.Frames[index], image.Point{}, draw.Over)
		}

		// Submit Encoder
		for y := 0; y < canvas.Rect.Dy(); y++ {
			start := y * canvas.Stride
			end := start + canvas.Rect.Dx()*4
			cmdStdin.Write(canvas.Pix[start:end])
		}
	}

	// Await Encoding
	cmdStdin.Close()
	if err := cmd.Wait(); err != nil {
		exitCode := -1
		if cmd.ProcessState != nil {
			exitCode = cmd.ProcessState.ExitCode()
		}
		log.Println("[sticker] FFMPEG Exited With Code %d\n%s\n", exitCode, outputLogs.String())
		return 0, err
	}

	log.Printf("[sticker] Rendered %d Images in %s", len(stickers), time.Since(t))
	stickerboardCopy()
	return len(stickers), nil
}
