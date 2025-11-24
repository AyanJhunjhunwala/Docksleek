package main

import (
	"context"
	"log"
	"github.com/docker/docker/api/types"
    "github.com/docker/docker/client"

)

func test() {

	ctx := context.Background()
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		log.Fatalf("Error creating Docker client: %v", err)

	}

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{All: true})
    if err != nil {
        log.Fatalf("Error listing containers: %v", err)
    }


	for _, container := range containers {
		log.Printf("%s  %s  %v  %s", container.ID[:12], container.Image, container.Names, container.Status)
    }
}
