package server

import (
	"fmt"
	"sync"

	"github.com/kubernetes-incubator/ocid/oci"
	"github.com/kubernetes-incubator/ocid/utils"
	"github.com/rajatchopra/ocicni"
)

const (
	runtimeAPIVersion = "v1alpha1"
)

// Server implements the RuntimeService and ImageService
type Server struct {
	runtime    *oci.Runtime
	sandboxDir string
	stateLock  sync.Mutex
	state      *serverState
	netPlugin  ocicni.CNIPlugin
}

// New creates a new Server with options provided
func New(runtimePath, sandboxDir, containerDir string) (*Server, error) {
	// TODO: This will go away later when we have wrapper process or systemd acting as
	// subreaper.
	if err := utils.SetSubreaper(1); err != nil {
		return nil, fmt.Errorf("failed to set server as subreaper: %v", err)
	}

	utils.StartReaper()

	r, err := oci.New(runtimePath, containerDir)
	if err != nil {
		return nil, err
	}
	sandboxes := make(map[string]*sandbox)
	containers := make(map[string]*oci.Container)
	netPlugin, err := ocicni.InitCNI("")
	if err != nil {
		return nil, err
	}
	return &Server{
		runtime:    r,
		netPlugin:  netPlugin,
		sandboxDir: sandboxDir,
		state: &serverState{
			sandboxes:  sandboxes,
			containers: containers,
		},
	}, nil
}

type serverState struct {
	sandboxes  map[string]*sandbox
	containers map[string]*oci.Container
}

type sandbox struct {
	name           string
	logDir         string
	labels         map[string]string
	containersLock sync.Mutex
	containers     map[string]*oci.Container
}

func (s *sandbox) addContainer(c *oci.Container) {
	s.containersLock.Lock()
	s.containers[c.Name()] = c
	s.containersLock.Unlock()
}

func (s *sandbox) getContainer(name string) *oci.Container {
	s.containersLock.Lock()
	c := s.containers[name]
	s.containersLock.Unlock()
	return c
}

func (s *sandbox) removeContainer(c *oci.Container) {
	s.containersLock.Lock()
	delete(s.containers, c.Name())
	s.containersLock.Unlock()
}

func (s *Server) addSandbox(sb *sandbox) {
	s.stateLock.Lock()
	s.state.sandboxes[sb.name] = sb
	s.stateLock.Unlock()
}

func (s *Server) getSandbox(name string) *sandbox {
	s.stateLock.Lock()
	sb := s.state.sandboxes[name]
	s.stateLock.Unlock()
	return sb
}

func (s *Server) hasSandbox(name string) bool {
	s.stateLock.Lock()
	_, ok := s.state.sandboxes[name]
	s.stateLock.Unlock()
	return ok
}

func (s *Server) addContainer(c *oci.Container) {
	s.stateLock.Lock()
	sandbox := s.state.sandboxes[c.Sandbox()]
	sandbox.addContainer(c)
	s.state.containers[c.Name()] = c
	s.stateLock.Unlock()
}

func (s *Server) getContainer(name string) *oci.Container {
	s.stateLock.Lock()
	c := s.state.containers[name]
	s.stateLock.Unlock()
	return c
}

func (s *Server) removeContainer(c *oci.Container) {
	s.stateLock.Lock()
	sandbox := s.state.sandboxes[c.Sandbox()]
	sandbox.removeContainer(c)
	delete(s.state.containers, c.Name())
	s.stateLock.Unlock()
}
