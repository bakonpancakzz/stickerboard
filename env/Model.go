package env

import (
	"context"
	"image"
	"log"
	"sync"
	"time"

	tf "github.com/galeone/tensorflow/tensorflow/go"
	"golang.org/x/image/draw"
)

const (
	MODEL_TRESHOLD = 0.7 // Threshold before an image is considered inappropriate
	MODEL_SIZE     = 224 // Model Size
)

var nsfwModel *tf.SavedModel

func SetupModel(stop context.Context, await *sync.WaitGroup) {
	t := time.Now()

	// Read Model from Disk
	model, err := tf.LoadSavedModel("resources/model", []string{"serve"}, nil)
	if err != nil {
		log.Fatalf("[model] Unable to Load Model (%s)\n", err)
	}
	nsfwModel = model

	// Test Model using Dummy Tensor
	dummy, _ := tf.NewTensor([1][MODEL_SIZE][MODEL_SIZE][3]float32{})
	if _, err := ModelClassifyTensor(dummy); err != nil {
		log.Fatalf("[model] Failed to Initialize Model (%s)\n", err)
	}

	// Shutdown Logic
	await.Add(1)
	go func() {
		defer await.Done()
		<-stop.Done()
		nsfwModel.Session.Close()
		log.Println("[model] Model Closed")
	}()

	log.Printf("[model] Model Ready (%s)\n", time.Since(t))
}

// Cast Predictions on a Tensor using the NSFW Model
func ModelClassifyTensor(tensor *tf.Tensor) ([]float32, error) {
	results, err := nsfwModel.Session.Run(
		map[tf.Output]*tf.Tensor{
			nsfwModel.Graph.Operation("serving_default_input").Output(0): tensor,
		},
		[]tf.Output{
			nsfwModel.Graph.Operation("StatefulPartitionedCall").Output(0),
		},
		[]*tf.Operation{},
	)
	if err != nil {
		return []float32{}, err
	}
	// cursed...
	return results[0].Value().([][]float32)[0], err
}

// Classify an Image returning true if it's considered safe
func ModelClassifyImage(someImage image.Image) (bool, error) {

	// Resize Image to Usable Size
	resized := image.NewRGBA(image.Rect(0, 0, MODEL_SIZE, MODEL_SIZE))
	draw.NearestNeighbor.Scale(resized, resized.Rect, someImage, someImage.Bounds(), draw.Over, nil)

	// Convert Pixel Data into Normalized Floats
	var tensorCap = MODEL_SIZE * MODEL_SIZE * 3
	var tensorData = make([]float32, 0, tensorCap)
	var tensorShape = []int64{1, MODEL_SIZE, MODEL_SIZE, 3}
	for x := 0; x < MODEL_SIZE; x++ {
		for y := 0; y < MODEL_SIZE; y++ {
			r, g, b, _ := resized.At(x, y).RGBA()
			tensorData = append(tensorData, float32(r)/65535, float32(g)/65535, float32(b)/65535)
		}
	}

	// Create Tensor, reshape it, then classify
	tensor, err := tf.NewTensor(tensorData)
	if err != nil {
		return false, err
	}
	if err := tensor.Reshape(tensorShape); err != nil {
		return false, err
	}
	results, err := ModelClassifyTensor(tensor)
	if err != nil {
		return false, err
	}

	// Calculate How Inappropriate this Image is
	// Drawing[0], Hentai[1], Neutral[2], Porn[3], Sexy[4]
	return (results[1] + results[3] + (results[4] * 0.9)) < MODEL_TRESHOLD, nil
}
