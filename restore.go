package main

import (
	"archive/tar"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"strings"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/network"
	"github.com/spf13/cobra"
)

var (
	optStart = false

	restoreCmd = &cobra.Command{
		Use:   "restore <backup file>",
		Short: "restores a backup of a container",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 1 {
				return fmt.Errorf("restore requires a .json or .tar backup")
			}

			if strings.HasSuffix(args[0], ".json") {
				return restore(args[0])
			} else if strings.HasSuffix(args[0], ".tar") {
				return restoreTar(args[0])
			}

			return fmt.Errorf("unknown file type, please provide a .tar or .json file")
		},
	}
)

func restoreTar(filename string) error {
	tarfile, err := os.Open(filename)
	if err != nil {
		return err
	}
	defer tarfile.Close()

	tr := tar.NewReader(tarfile)
	var b []byte
	for {
		th, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		switch th.Name {
		case "container.json":
			var err error
			b, err = ioutil.ReadAll(tr)
			if err != nil {
				return err
			}
		}
	}

	var backup Backup
	err = json.Unmarshal(b, &backup)
	if err != nil {
		return err
	}

	id, err := createContainer(backup)
	if err != nil {
		return err
	}

	if optStart {
		return startContainer(id)
	}
	return nil
}

func restore(filename string) error {
	var backup Backup
	b, err := ioutil.ReadFile(filename)
	if err != nil {
		return err
	}
	err = json.Unmarshal(b, &backup)
	if err != nil {
		return err
	}

	id, err := createContainer(backup)
	if err != nil {
		return err
	}

	if optStart {
		return startContainer(id)
	}
	return nil
}

func createContainer(backup Backup) (string, error) {
	nameparts := strings.Split(backup.Name, "/")
	name := nameparts[len(nameparts)-1]
	fmt.Println("Restoring Container:", name)

	// Create container configuration
	config := backup.Config

	// Create host configuration
	hostConfig := backup.HostConfig

	// Get network configuration
	var networkConfig *network.NetworkingConfig
	if backup.NetworkSettings != nil {
		ep := make(map[string]*network.EndpointSettings)
		for k, v := range backup.NetworkSettings.Networks {
			ep[k] = v
		}
		networkConfig = &network.NetworkingConfig{
			EndpointsConfig: ep,
		}
	}

	// Create the container
	resp, err := cli.ContainerCreate(
		ctx,
		config,
		hostConfig,
		networkConfig,
		nil,
		name,
	)
	if err != nil {
		return "", fmt.Errorf("failed to create container: %v", err)
	}

	fmt.Println("Created Container with ID:", resp.ID)

	for _, m := range backup.HostConfig.Mounts {
		fmt.Printf("Old Mount (type %s) %s -> %s\n", m.Type, m.Source, m.Target)
	}

	conf, err := cli.ContainerInspect(ctx, resp.ID)
	if err != nil {
		return "", err
	}
	for _, m := range conf.Mounts {
		fmt.Printf("New Mount (type %s) %s -> %s\n", m.Type, m.Source, m.Destination)
	}

	return resp.ID, nil
}

func startContainer(id string) error {
	fmt.Println("Starting container:", id[:12])

	err := cli.ContainerStart(ctx, id, container.StartOptions{})
	if err != nil {
		return err
	}

	/*
		statusCh, errCh := cli.ContainerWait(ctx, id, container.WaitConditionNotRunning)
		select {
		case err := <-errCh:
			if err != nil {
				return err
			}
		case <-statusCh:
		}

		out, err := cli.ContainerLogs(ctx, id, container.LogsOptions{ShowStdout: true})
		if err != nil {
			return err
		}
		io.Copy(os.Stdout, out)
	*/

	return nil
}

func init() {
	restoreCmd.Flags().BoolVarP(&optStart, "start", "s", false, "start restored container")
	RootCmd.AddCommand(restoreCmd)
}
