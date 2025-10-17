package routes

import (
	"io"
	"log"
	"mime"
	"net/http"
	"os"
	"path"
	"time"

	"bakonpancakz/stickerboard/env"
)

var pathPublic = path.Join("resources", "public")

// Serve Static File from Resource Directory
func serveStaticFilename(w http.ResponseWriter, filepath string) {

	// Read File from Disk
	f, err := os.Open(filepath)
	if err != nil {
		if os.IsNotExist(err) {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		log.Println("[http] Read Asset Error:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer f.Close()

	// Determine Content-Type and Stream Contents
	w.Header().Add("Content-Type", mime.TypeByExtension(path.Ext(filepath)))
	io.Copy(w, f)
}

func GET_Assets_Filename(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	f := path.Clean(r.PathValue("filename"))

	if f == env.STICKERBOARD_FILENAME {
		// Wait Until Stickerboard is Ready, this should only occur on startup!
		if !env.StickerboardReady.Load() {
			for {
				if env.StickerboardReady.Load() {
					break
				}
				time.Sleep(time.Second)
			}
		}
		// Serve Stickerboard from Memory
		env.StickerboardMtx.RLock()
		w.Header().Add("Content-Type", "image/webp")
		w.Write(env.Stickerboard)
		env.StickerboardMtx.RUnlock()
		return
	}

	// Serve Asset from Disk
	serveStaticFilename(w, path.Join(pathPublic, f))
}
