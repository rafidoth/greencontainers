package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/go-connections/nat"
)

type ImageMetadata struct {
	Name      string
	Tag       string
	Digest    string
	ID        string
	SizeBytes int64
	SizeMB    float64
	NumLayers int
}

type Controller interface {
	Pull(ctx context.Context, image string) error
	Build(ctx context.Context, dockerfile, contextDir, tag string, noCache bool, buildArgs map[string]*string) error
	Start(ctx context.Context, name, image string, cmd []string, hostPort, containerPort string) error
	Stop(ctx context.Context, name string) error
	RemoveContainer(ctx context.Context, name string) error
	RemoveImage(ctx context.Context, image string) error
	InspectImage(ctx context.Context, image string) (ImageMetadata, error)
	Version(ctx context.Context) (string, error)
}

type dockerController struct {
	cli *client.Client
}

func NewController() (Controller, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	return &dockerController{cli: cli}, nil
}

func (c *dockerController) Pull(ctx context.Context, image string) error {
	reader, err := c.cli.ImagePull(ctx, image, types.ImagePullOptions{})
	if err != nil {
		return err
	}
	defer reader.Close()
	// consume the output to wait for pull to finish
	_, err = io.Copy(io.Discard, reader)
	return err
}

func (c *dockerController) Build(ctx context.Context, dockerfile, contextDir, tag string, noCache bool, buildArgs map[string]*string) error {
	tar, err := archive.TarWithOptions(contextDir, &archive.TarOptions{})
	if err != nil {
		return err
	}

	opts := types.ImageBuildOptions{
		Dockerfile: dockerfile,
		Tags:       []string{tag},
		Remove:     true,
		NoCache:    noCache,
		BuildArgs:  buildArgs,
	}

	res, err := c.cli.ImageBuild(ctx, tar, opts)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	// Docker's build API streams JSON messages. Errors are embedded
	// in the stream, not returned as HTTP errors. We must parse each
	// message to detect build failures.
	decoder := json.NewDecoder(res.Body)
	for {
		var msg struct {
			Stream string `json:"stream"`
			Error  string `json:"error"`
		}
		if err := decoder.Decode(&msg); err != nil {
			if err == io.EOF {
				break
			}
			return fmt.Errorf("error reading build output: %w", err)
		}
		if msg.Error != "" {
			return fmt.Errorf("docker build error: %s", msg.Error)
		}
	}
	return nil
}

func (c *dockerController) Start(ctx context.Context, name, image string, cmd []string, hostPort, containerPort string) error {
	cfg := &container.Config{
		Image: image,
		Cmd:   cmd,
	}
	hostConfig := &container.HostConfig{}
	networkConfig := &network.NetworkingConfig{}

	// Set up port mapping if both ports are specified
	if hostPort != "" && containerPort != "" {
		port, err := nat.NewPort("tcp", containerPort)
		if err != nil {
			return fmt.Errorf("invalid container port %s: %w", containerPort, err)
		}

		cfg.ExposedPorts = nat.PortSet{
			port: struct{}{},
		}

		hostConfig.PortBindings = nat.PortMap{
			port: []nat.PortBinding{
				{
					HostIP:   "0.0.0.0",
					HostPort: hostPort,
				},
			},
		}
	}

	resp, err := c.cli.ContainerCreate(ctx, cfg, hostConfig, networkConfig, nil, name)
	if err != nil {
		return err
	}

	return c.cli.ContainerStart(ctx, resp.ID, types.ContainerStartOptions{})
}

func (c *dockerController) Stop(ctx context.Context, name string) error {
	// Stop with a short timeout to prevent hanging forever
	timeout := 10
	return c.cli.ContainerStop(ctx, name, container.StopOptions{Timeout: &timeout})
}

func (c *dockerController) RemoveContainer(ctx context.Context, name string) error {
	return c.cli.ContainerRemove(ctx, name, types.ContainerRemoveOptions{Force: true})
}

func (c *dockerController) RemoveImage(ctx context.Context, image string) error {
	_, err := c.cli.ImageRemove(ctx, image, types.ImageRemoveOptions{Force: true})
	return err
}

func (c *dockerController) InspectImage(ctx context.Context, image string) (ImageMetadata, error) {
	img, _, err := c.cli.ImageInspectWithRaw(ctx, image)
	if err != nil {
		return ImageMetadata{}, err
	}

	digest := ""
	if len(img.RepoDigests) > 0 {
		parts := strings.Split(img.RepoDigests[0], "@")
		if len(parts) == 2 {
			digest = parts[1]
		}
	}

	name := image
	tag := "latest"
	if strings.Contains(image, ":") {
		parts := strings.SplitN(image, ":", 2)
		name = parts[0]
		tag = parts[1]
	}

	var layers int
	if img.RootFS.Type == "layers" {
		layers = len(img.RootFS.Layers)
	}

	return ImageMetadata{
		Name:      name,
		Tag:       tag,
		Digest:    digest,
		ID:        img.ID,
		SizeBytes: img.Size,
		SizeMB:    float64(img.Size) / (1024 * 1024),
		NumLayers: layers,
	}, nil
}

func (c *dockerController) Version(ctx context.Context) (string, error) {
	ver, err := c.cli.ServerVersion(ctx)
	if err != nil {
		return "", err
	}
	return ver.Version, nil
}
