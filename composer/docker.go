package composer

import (
	"context"
	"fmt"
	"os"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/pkg/errors"
	"github.com/tomatool/tomato/config"
)

type DockerClient struct {
	Client *client.Client

	listOfContainerIDs []string
}

func (c *DockerClient) CreateContainer(ctx context.Context, config *config.Resource) error {
	_, err := c.Client.ImagePull(ctx, "docker.io/library/"+config.Image, types.ImagePullOptions{})
	if err != nil {
		return errors.Wrapf(err, "Failed to pull an image: %s", config.Image)
	}
	resp, err := c.Client.ContainerCreate(ctx, &container.Config{
		Image: config.Image,
	}, nil, nil, "tomato_"+config.Name)
	if err != nil {
		return errors.Wrapf(err, "Failed to create a container: %s", config.Image)
	}

	c.listOfContainerIDs = append(c.listOfContainerIDs, resp.ID)

	err = c.Client.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
	if err != nil {
		return errors.Wrapf(err, "Failed to start a container: %s", config.Image)
	}
	return err
}
func (c *DockerClient) DeleteAll(ctx context.Context) error {
	for _, containerID := range c.listOfContainerIDs {
		err := c.Client.ContainerRemove(ctx, containerID, types.ContainerRemoveOptions{
			Force: true,
		})
		if err != nil {
			fmt.Fprintf(os.Stdout, "Failed to delete container %s", containerID)
		}
	}
	return nil
}
