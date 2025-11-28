package main
import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/client"
)

func AnalyzeImageLayers(imageName string) error {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return err
	}
	defer cli.Close()

	history, err := cli.ImageHistory(ctx, imageName)
	if err != nil {
		return err
	}

	fmt.Printf("Layers for image %s:\n\n", imageName)

	// history is from newest layer to oldest
	var cumulativeSize int64
	for i, h := range history {
		layerSize := h.Size
		cumulativeSize += layerSize

		created := time.Unix(h.Created, 0)

		fmt.Printf("Layer #%d\n", len(history)-i-1)
		fmt.Printf("  ID:        %s\n", h.ID)
		fmt.Printf("  Created:   %s\n", created.Format(time.RFC3339))
		fmt.Printf("  Size:      %.2f MB\n", float64(layerSize)/(1024*1024))
		fmt.Printf("  Cumulative: %.2f MB\n", float64(cumulativeSize)/(1024*1024))
		fmt.Printf("  CreatedBy: %s\n", h.CreatedBy)
		if h.Comment != "" {
			fmt.Printf("  Comment:   %s\n", h.Comment)
		}
		fmt.Println()
	}

	return nil
}


func main() {

	// Can update this with all images available in the folder
	if err := AnalyzeImageLayers("docksleek:latest"); err != nil {
		log.Fatal(err)
	}
}