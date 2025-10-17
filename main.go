package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"bakonpancakz/stickerboard/env"
	"bakonpancakz/stickerboard/routes"

	_ "github.com/joho/godotenv/autoload"
)

func main() {
	// Startup Services
	var stopCtx, stop = context.WithCancel(context.Background())
	var stopWg sync.WaitGroup
	env.SetupDatabase(stopCtx, &stopWg)
	env.SetupModel(stopCtx, &stopWg)
	go SetupHTTP(stopCtx, &stopWg)
	go env.StickerboardRender()

	// Await Shutdown Signal
	cancel := make(chan os.Signal, 1)
	signal.Notify(cancel, syscall.SIGINT, syscall.SIGTERM)
	<-cancel
	stop()

	// Begin Shutdown Process
	timeout, finish := context.WithTimeout(context.Background(), time.Minute)
	defer finish()
	go func() {
		<-timeout.Done()
		if timeout.Err() == context.DeadlineExceeded {
			log.Fatalln("[main] Cleanup timeout! Exiting now.")
		}
	}()
	stopWg.Wait()
	log.Println("[main] All done, bye bye!")
	os.Exit(0)
}

func SetupHTTP(stop context.Context, await *sync.WaitGroup) {

	r := http.NewServeMux()
	r.HandleFunc("/", routes.GET_Index)
	r.HandleFunc("/stickers", routes.POST_Stickers)
	r.HandleFunc("/assets/{filename}", routes.GET_Assets_Filename)
	svr := http.Server{
		Handler:           r,
		Addr:              env.HTTP_ADDRESS,
		TLSConfig:         env.HTTP_TLS,
		MaxHeaderBytes:    4096,
		IdleTimeout:       5 * time.Second,
		ReadHeaderTimeout: 5 * time.Second,
		WriteTimeout:      30 * time.Second,
		ReadTimeout:       30 * time.Second,
	}

	// Shutdown Logic
	await.Add(1)
	go func() {
		defer await.Done()
		<-stop.Done()
		svr.Shutdown(context.Background())
		log.Println("[http] Cleaned up HTTP")
	}()

	// Server Startup
	var err error
	if env.TLS_ENABLED {
		log.Printf("[http] Bound HTTPS - %s\n", svr.Addr)
		err = svr.ListenAndServeTLS("", "")
	} else {
		log.Printf("[http] Bound HTTP - %s\n", svr.Addr)
		err = svr.ListenAndServe()
	}
	if err != http.ErrServerClosed {
		log.Fatalln("[http] Listen Error:", err)
	}
}
