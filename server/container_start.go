package server

import (
	"fmt"
	"path/filepath"

	"github.com/Sirupsen/logrus"
	"github.com/opencontainers/runtime-tools/generate"
	"golang.org/x/net/context"
	pb "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

// StartContainer starts the container.
func (s *Server) StartContainer(ctx context.Context, req *pb.StartContainerRequest) (*pb.StartContainerResponse, error) {
	logrus.Debugf("StartContainerRequest %+v", req)
	c, err := s.getContainerFromRequest(req)
	if err != nil {
		return nil, err
	}

	workdir, err := s.storage.GetWorkDir(c.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to find work directory for container %s(%s): %v", c.Name(), c.ID(), err)
	}
	rundir, err := s.storage.GetRunDir(c.Name())
	if err != nil {
		return nil, fmt.Errorf("failed to find runtime directory for container %s(%s): %v", c.Name(), c.ID(), err)
	}
	mountPoint, err := s.storage.StartContainer(c.ID())
	if err != nil {
		return nil, fmt.Errorf("failed to mount container %s(%s): %v", c.Name(), c.ID(), err)
	}
	specgen, err := generate.NewFromFile(filepath.Join(workdir, "config.json"))
	if err != nil {
		return nil, fmt.Errorf("failed to read template configuration for container: %v", err)
	}
	specgen.SetRootPath(mountPoint)
	saveOptions := generate.ExportOptions{}
	if err = specgen.SaveToFile(filepath.Join(workdir, "config.json"), saveOptions); err != nil {
		return nil, fmt.Errorf("failed to rewrite template configuration for container: %v", err)
	}
	if err = specgen.SaveToFile(filepath.Join(rundir, "config.json"), saveOptions); err != nil {
		return nil, fmt.Errorf("failed to write runtime configuration for container: %v", err)
	}

	if err = s.runtime.StartContainer(c); err != nil {
		return nil, fmt.Errorf("failed to start container %s: %v", c.ID(), err)
	}

	resp := &pb.StartContainerResponse{}
	logrus.Debugf("StartContainerResponse %+v", resp)
	return resp, nil
}
