package routes

import (
	"bakonpancakz/stickerboard/env"
	"bytes"
	"crypto/sha1"
	"encoding/json"
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"sync"
	"time"

	"golang.org/x/image/webp"
)

// Dead Simple Ratelimiting, refreshes whenever it feels like...
var uploadDebounce sync.Map

func init() {
	go func() {
		for {
			uploadDebounce.Clear()
			time.Sleep(time.Minute)
		}
	}()
}

// Retrieve the real IP address based on the environment variables
func getRealAddress(r *http.Request) string {
	var ip string
	if env.HTTP_PROXY_HEADER != "" {
		ip = r.Header.Get(env.HTTP_PROXY_HEADER)
		if ip != "" {
			return ip
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		host = r.RemoteAddr
	}
	ip = host
	return ip
}

func POST_Stickers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Rate Limiting
	uploadOK := false
	uploadIP := getRealAddress(r)
	if _, exists := uploadDebounce.Load(uploadIP); exists {
		http.Error(w, "Posted Too Recently", http.StatusTooManyRequests)
		return
	}
	defer func() {
		if !uploadOK {
			uploadDebounce.Delete(uploadIP)
		}
	}()
	uploadDebounce.Store(uploadIP, true)

	// Sanity Checks
	r.Body = http.MaxBytesReader(w, r.Body, env.MAX_FORM_BYTES)
	if r.ContentLength > env.MAX_FORM_BYTES {
		http.Error(w, "Payload Too Large", http.StatusRequestEntityTooLarge)
		return
	}
	if err := r.ParseMultipartForm(env.MAX_FORM_BYTES); err != nil {
		http.Error(w, "Invalid Form Body", http.StatusBadRequest)
		return
	}

	// Parse Incoming JSON
	var formJSON struct {
		OffsetX    int    `json:"offset_x"`
		OffsetY    int    `json:"offset_y"`
		ImageScale int    `json:"image_scale"`
		UserName   string `json:"user_name"`
		UserURL    string `json:"user_url"`
		Message    string `json:"message"`
	}
	if err := json.Unmarshal([]byte(r.FormValue("data")), &formJSON); err != nil {
		http.Error(w, "Malformed Form Data", http.StatusBadRequest)
		return
	}
	if formJSON.ImageScale < 1 || formJSON.ImageScale > 100 || len(formJSON.Message) > 1024 {
		http.Error(w, "Invalid Form Body", http.StatusBadRequest)
		return
	}

	// Copy Incoming Image to Memory
	// 	We're might partially or fully read it multiple times, so yes it has to
	// 	be stored entirely in memory (@_@)
	var formImage []byte
	if file, _, err := r.FormFile("sticker"); err != nil {
		http.Error(w, "Malformed Form Image", http.StatusBadRequest)
		return
	} else if data, err := io.ReadAll(file); err != nil {
		http.Error(w, "Invalid Form Body", http.StatusBadRequest)
		return
	} else {
		file.Close()
		formImage = data
	}

	// Validate Incoming Image
	var imageType = env.ImageSniffType(formImage)
	var imageInfo image.Config
	var imageErr error

	switch imageType {
	case env.IMAGE_WEBP:
		imageInfo, imageErr = webp.DecodeConfig(bytes.NewReader(formImage))
	case env.IMAGE_JPEG:
		imageInfo, imageErr = jpeg.DecodeConfig(bytes.NewReader(formImage))
	case env.IMAGE_PNG:
		imageInfo, imageErr = png.DecodeConfig(bytes.NewReader(formImage))
	case env.IMAGE_GIF:
		imageInfo, imageErr = gif.DecodeConfig(bytes.NewReader(formImage))
	default:
		http.Error(w, "Unsupported Image Format", http.StatusBadRequest)
		return
	}
	if imageErr != nil {
		http.Error(w, "Invalid Image Data", http.StatusBadRequest)
		return
	}
	if imageInfo.Height > 2048 || imageInfo.Width > 2048 {
		http.Error(w, "Image dimension cannot be larger than 2048 pixels", http.StatusBadRequest)
		return
	}
	if imageInfo.Height < 32 || imageInfo.Width < 32 {
		http.Error(w, "Image dimension cannot be smaller than 32 pixels", http.StatusBadRequest)
		return
	}

	// Validate Image Placement
	scaledFloat := float32(formJSON.ImageScale) / 100
	scaledWidth := int((float32(imageInfo.Width) * scaledFloat))
	scaledHeight := int((float32(imageInfo.Height) * scaledFloat))
	if scaledHeight > (env.CANVAS_STICKER_MAX_HEIGHT + 4) {
		http.Error(w, "Image is Too Large", http.StatusBadRequest)
		return
	}
	if formJSON.OffsetX < -scaledWidth || formJSON.OffsetX > env.CANVAS_WIDTH ||
		formJSON.OffsetY < -scaledHeight || formJSON.OffsetY > env.CANVAS_HEIGHT {
		http.Error(w, "Image cannot be placed off-screen", http.StatusBadRequest)
		return
	}

	// Decode Image Frame(s) for Classification
	var imageFrames = make([]image.Image, 0, 1)
	var decodedStill image.Image
	var decodedFrame *gif.GIF

	switch imageType {
	case env.IMAGE_WEBP:
		decodedStill, imageErr = webp.Decode(bytes.NewReader(formImage))
	case env.IMAGE_JPEG:
		decodedStill, imageErr = jpeg.Decode(bytes.NewReader(formImage))
	case env.IMAGE_PNG:
		decodedStill, imageErr = png.Decode(bytes.NewReader(formImage))
	case env.IMAGE_GIF:
		decodedFrame, imageErr = gif.DecodeAll(bytes.NewReader(formImage))
	default:
		log.Printf("[http] Missing Decoder for Image Type: %s\n", imageType)
		http.Error(w, "Unsupported Image Format", http.StatusBadRequest)
		return
	}
	if imageErr != nil {
		http.Error(w, "Invalid Image Data", http.StatusBadRequest)
		return
	} else {
		// Copy Decoded Frames
		if imageType == env.IMAGE_GIF {
			for i := range decodedFrame.Image {
				frame := decodedFrame.Image[i]
				imageFrames = append(imageFrames, frame.SubImage(frame.Rect))
			}
		} else {
			imageFrames = append(imageFrames, decodedStill)
			decodedStill = nil
		}
	}

	// Classify Decoded Frames
	for i := range imageFrames {
		pass, err := env.ModelClassifyImage(imageFrames[i])
		if err != nil {
			log.Println("[http] Cannot Classify Image:", err)
			http.Error(w, "Model Error", http.StatusInternalServerError)
			return
		}
		if !pass {
			log.Printf("[http] Inappopriate Image Uploaded by %s\n", uploadIP)
			http.Error(w, "Inappropriate Image", http.StatusBadRequest)
			return
		}
	}

	// Write Contents to Disk
	imageHash := fmt.Sprintf("%X", sha1.Sum(formImage))
	imagePath := path.Join(env.DATA_DIRECTORY, imageHash)
	if err := os.WriteFile(imagePath, formImage, env.FILE_MODE); err != nil {
		log.Println("[http] Cannot Write Image:", imagePath, err)
		return
	}
	// Write Contents to Database
	env.DatabaseMtx.Lock()
	env.Database.Stickers = append(env.Database.Stickers, env.DatabaseSticker{
		Created:     time.Now(),
		UserAddress: uploadIP,
		UserName:    formJSON.UserName,
		UserURL:     formJSON.UserURL,
		Message:     formJSON.Message,
		Visible:     true,
		OffsetX:     formJSON.OffsetX,
		OffsetY:     formJSON.OffsetY,
		ImageScale:  float64(formJSON.ImageScale) / 100,
		ImageHeight: imageInfo.Height,
		ImageWidth:  imageInfo.Width,
		ImageType:   imageType,
		ImageHash:   imageHash,
	})
	env.DatabaseMtx.Unlock()
	uploadOK = true

	// Update Stickerboard
	env.StickerboardRender()
	w.WriteHeader(http.StatusCreated)
}
