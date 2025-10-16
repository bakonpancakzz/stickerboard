package env

import (
	"runtime"
	"sync"
	"sync/atomic"
)

type ImageType string

const (
	IMAGE_OTHER ImageType = "UNKNOWN"
	IMAGE_WEBP  ImageType = "WEBP"
	IMAGE_JPEG  ImageType = "JPG"
	IMAGE_PNG   ImageType = "PNG"
	IMAGE_GIF   ImageType = "GIF"
)

// Classify the contents of a file based on it's starting bytes
// https://en.wikipedia.org/wiki/Magic_number_(programming)#Magic_numbers_in_files)
func ImageSniffType(d []byte) ImageType {
	switch {
	case len(d) > 3 && // JPEG
		d[0] == 0xFF && d[1] == 0xD8 && d[2] == 0xFF:
		return IMAGE_JPEG

	case len(d) > 8 && // PNG
		d[0] == 0x89 && d[1] == 0x50 && d[2] == 0x4E && d[3] == 0x47 &&
		d[4] == 0x0D && d[5] == 0x0A && d[6] == 0x1A && d[7] == 0x0A:
		return IMAGE_PNG

	case len(d) > 4 && // GIF
		d[0] == 0x47 && d[1] == 0x49 && d[2] == 0x46 && d[3] == 0x38:
		return IMAGE_GIF

	case len(d) > 12 && // WEBP
		d[0] == 0x52 && d[1] == 0x49 && d[2] == 0x46 && d[3] == 0x46 &&
		d[8] == 0x57 && d[9] == 0x45 && d[10] == 0x42 && d[11] == 0x50:
		return IMAGE_WEBP

	default:
		return IMAGE_OTHER
	}
}

// Easily spread a workload across all available threads, returning the error if any
func Multithread(jobCount int, handler func(i int) error) error {
	threads := runtime.NumCPU()
	channel := make(chan int, jobCount)
	var wait sync.WaitGroup
	var fail atomic.Value

	// Startup Goroutines
	for workerId := 0; workerId < threads; workerId++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for i := range channel {
				// Exit early if an error has ocurred
				if fail.Load() != nil {
					return
				}
				// Run Handler and store it's error (if any)
				if err := handler(i); err != nil {
					fail.Store(err)
					return
				}
			}
		}()
	}

	// Queue and Complete Work Across all Threads
	for i := 0; i < jobCount; i++ {
		channel <- i
	}
	close(channel) // close or wait forever
	wait.Wait()

	// Return Error (if any)
	if v := fail.Load(); v != nil {
		return v.(error)
	}
	return nil
}
