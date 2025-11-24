package main

import (
	"context"
	"log"

	"github.com/docker/docker/api/types/container" // NEW: Needed for the options (container.ListOptions)
	"github.com/docker/docker/client"
)

func test() {
	ctx := context.Background()

	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Error creating Docker client: %v", err)
	}
	defer cli.Close()

	containers, err := cli.ContainerList(ctx, container.ListOptions{All: true})
	if err != nil {
		log.Fatalf("Error listing containers: %v", err)
	}

	for _, ctr := range containers {
		id := ctr.ID
		if len(id) > 12 {
			id = id[:12]
		}
		log.Printf("%s  %s  %v  %s", id, ctr.Image, ctr.Names, ctr.Status)
	}
}

func main() {
	test()
}