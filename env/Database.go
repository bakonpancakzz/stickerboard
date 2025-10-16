package env

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"os"
	"path"
	"sync"
	"time"
)

type DatabaseRoot struct {
	Stickers []DatabaseSticker `json:"stickers"`
}

type DatabaseSticker struct {
	Created     time.Time `json:"created"`      // Sticker Created
	UserAddress string    `json:"user_address"` // User IP Address (For Manual Bans)
	UserName    string    `json:"user_name"`    // User Name
	UserURL     string    `json:"user_url"`     // User URL (Optional)
	Message     string    `json:"message"`      // Sticker Message
	Visible     bool      `json:"visible"`      // Sticker Visible?
	OffsetX     int       `json:"offset_x"`     // Placement X
	OffsetY     int       `json:"offset_y"`     // Placement Y
	ImageScale  float64   `json:"image_scale"`  // Image Scale
	ImageHeight int       `json:"image_height"` // Image Height
	ImageWidth  int       `json:"image_width"`  // Image Width
	ImageType   ImageType `json:"image_type"`   // Original Image File Type
	ImageHash   string    `json:"image_hash"`   // Original Image File Hash
}

var (
	Database    DatabaseRoot
	DatabaseMtx sync.RWMutex
)

func SetupDatabase(stop context.Context, await *sync.WaitGroup) {
	t := time.Now()
	p := path.Join(DATA_DIRECTORY, "database.json")

	// Decode File
	b, err := os.ReadFile(p)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			log.Fatalln("[db] Read Database Error:", err)
		}
		Database.Stickers = make([]DatabaseSticker, 0)
	}
	if err == nil {
		if err := json.Unmarshal(b, &Database); err != nil {
			log.Fatalln("[db] Parse Database Error:", err)
		}
	}

	// Shutdown Logic
	await.Add(1)
	go func() {
		defer await.Done()
		<-stop.Done()

		// Write Database To Disk
		b, err := json.MarshalIndent(&Database, "", "    ")
		if err != nil {
			log.Fatalln("[db] Cannot Marshal Database:", err)
		}
		if err := os.WriteFile(p, b, FILE_MODE); err != nil {
			log.Fatalln("[db] Cannot Write Database:", err)
		}
		log.Println("[db] Database Saved")
	}()

	log.Println("[db] Ready in", time.Since(t))
}
