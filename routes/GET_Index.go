package routes

import (
	"bytes"
	"fmt"
	"html/template"
	"net/http"

	"bakonpancakz/stickerboard/env"
)

func GET_Index(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	// Parse and Render Template
	// 	Rendered Document is stored in memory to help protect Database
	// 	from Slowloris attacks
	env.DatabaseMtx.RLock()
	tmpl, err := template.ParseFiles("resources/index.html")
	if err != nil {
		fmt.Println("[http] Template Parse Error", err)
		w.WriteHeader(http.StatusInternalServerError)
		env.DatabaseMtx.RUnlock()
		return
	}
	var data bytes.Buffer
	if err := tmpl.Execute(&data, env.Database.Stickers); err != nil {
		fmt.Println("[http] Template Execute Error", err)
		w.WriteHeader(http.StatusInternalServerError)
		env.DatabaseMtx.RUnlock()
		return
	}
	env.DatabaseMtx.RUnlock()

	// Send Document
	w.Header().Add("Content-Type", "text/html; charset=utf-8")
	w.Write(data.Bytes())
}
