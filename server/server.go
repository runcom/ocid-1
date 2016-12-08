package server

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"sync"
	"syscall"

	"github.com/Sirupsen/logrus"
	storage "github.com/containers/storage/cri"
	sstorage "github.com/containers/storage/storage"
	"github.com/docker/docker/pkg/registrar"
	"github.com/docker/docker/pkg/truncindex"
	"github.com/kubernetes-incubator/cri-o/oci"
	"github.com/kubernetes-incubator/cri-o/server/apparmor"
	"github.com/kubernetes-incubator/cri-o/server/seccomp"
	"github.com/kubernetes-incubator/cri-o/utils"
	"github.com/opencontainers/runc/libcontainer/label"
	rspec "github.com/opencontainers/runtime-spec/specs-go"
	"github.com/rajatchopra/ocicni"
	pb "k8s.io/kubernetes/pkg/kubelet/api/v1alpha1/runtime"
)

const (
	runtimeAPIVersion = "v1alpha1"
)

// Server implements the RuntimeService and ImageService
type Server struct {
	config       Config
	runtime      *oci.Runtime
	store        sstorage.Store
	images       storage.ImageServer
	storage      storage.RuntimeServer
	stateLock    sync.Mutex
	state        *serverState
	netPlugin    ocicni.CNIPlugin
	podNameIndex *registrar.Registrar
	podIDIndex   *truncindex.TruncIndex
	ctrNameIndex *registrar.Registrar
	ctrIDIndex   *truncindex.TruncIndex

	seccompEnabled bool
	seccompProfile seccomp.Seccomp

	appArmorEnabled bool
	appArmorProfile string
}

func (s *Server) loadContainer(id string) error {
	config, err := s.store.GetContainerDirectoryFile(id, "config.json")
	if err != nil {
		return err
	}
	var m rspec.Spec
	if err = json.Unmarshal(config, &m); err != nil {
		return err
	}
	labels := make(map[string]string)
	if err = json.Unmarshal([]byte(m.Annotations["ocid/labels"]), &labels); err != nil {
		return err
	}
	name := m.Annotations["ocid/name"]
	name, err = s.reserveContainerName(id, name)
	if err != nil {
		return err
	}
	var metadata pb.ContainerMetadata
	if err = json.Unmarshal([]byte(m.Annotations["ocid/metadata"]), &metadata); err != nil {
		return err
	}
	sb := s.getSandbox(m.Annotations["ocid/sandbox_id"])
	if sb == nil {
		logrus.Warnf("could not get sandbox with id %s, skipping", m.Annotations["ocid/sandbox_id"])
		return nil
	}

	var tty bool
	if v := m.Annotations["ocid/tty"]; v == "true" {
		tty = true
	}
	containerPath, err := s.store.GetContainerRunDirectory(id)
	if err != nil {
		return err
	}

	ctr, err := oci.NewContainer(id, name, containerPath, m.Annotations["ocid/log_path"], labels, &metadata, sb.id, tty)
	if err != nil {
		return err
	}
	s.addContainer(ctr)
	if err = s.runtime.UpdateStatus(ctr); err != nil {
		logrus.Warnf("error updating status for container %s: %v", ctr.ID(), err)
	}
	if err = s.ctrIDIndex.Add(id); err != nil {
		return err
	}
	return nil
}

func (s *Server) loadSandbox(id string) error {
	config, err := s.store.GetContainerDirectoryFile(id, "config.json")
	if err != nil {
		return err
	}
	var m rspec.Spec
	if err = json.Unmarshal(config, &m); err != nil {
		return err
	}
	labels := make(map[string]string)
	if err = json.Unmarshal([]byte(m.Annotations["ocid/labels"]), &labels); err != nil {
		return err
	}
	name := m.Annotations["ocid/name"]
	name, err = s.reservePodName(id, name)
	if err != nil {
		return err
	}
	var metadata pb.PodSandboxMetadata
	if err = json.Unmarshal([]byte(m.Annotations["ocid/metadata"]), &metadata); err != nil {
		return err
	}

	processLabel, mountLabel, err := label.InitLabels(label.DupSecOpt(m.Process.SelinuxLabel))
	if err != nil {
		return err
	}

	annotations := make(map[string]string)
	if err = json.Unmarshal([]byte(m.Annotations["ocid/annotations"]), &annotations); err != nil {
		return err
	}

	sb := &sandbox{
		id:           id,
		name:         name,
		logDir:       m.Annotations["ocid/log_path"],
		labels:       labels,
		containers:   oci.NewMemoryStore(),
		processLabel: processLabel,
		mountLabel:   mountLabel,
		annotations:  annotations,
		metadata:     &metadata,
	}
	s.addSandbox(sb)

	sandboxPath, err := s.store.GetContainerRunDirectory(id)
	if err != nil {
		return err
	}

	if err = label.ReserveLabel(processLabel); err != nil {
		return err
	}

	cname, err := s.reserveContainerName(m.Annotations["ocid/container_id"], m.Annotations["ocid/container_name"])
	if err != nil {
		return err
	}
	scontainer, err := oci.NewContainer(m.Annotations["ocid/container_id"], cname, sandboxPath, sandboxPath, labels, nil, id, false)
	if err != nil {
		return err
	}
	sb.infraContainer = scontainer
	if err = s.runtime.UpdateStatus(scontainer); err != nil {
		logrus.Warnf("error updating status for pod sandbox infra container %s: %v", scontainer.ID(), err)
	}
	if err = s.ctrIDIndex.Add(scontainer.ID()); err != nil {
		return err
	}
	if err = s.podIDIndex.Add(id); err != nil {
		return err
	}
	return nil
}

func (s *Server) restore() {
	containers, err := s.store.Containers()
	if err != nil && !os.IsNotExist(err) {
		logrus.Warnf("could not read containers and sandboxes: %v", err)
	}
	pods := map[string]*storage.RuntimeContainerMetadata{}
	podContainers := map[string]*storage.RuntimeContainerMetadata{}
	for _, container := range containers {
		metadata, err2 := s.storage.GetContainerMetadata(container.ID)
		if err2 != nil {
			logrus.Warnf("error parsing metadata for %s: %v, ignoring", container.ID, err2)
			continue
		}
		if metadata.Pod {
			pods[container.ID] = &metadata
		} else {
			podContainers[container.ID] = &metadata
		}
	}
	for containerID, metadata := range pods {
		if err = s.loadSandbox(containerID); err != nil {
			logrus.Warnf("could not restore sandbox %s container %s: %v", metadata.PodID, containerID, err)
		}
	}
	for containerID := range podContainers {
		if err := s.loadContainer(containerID); err != nil {
			logrus.Warnf("could not restore container %s: %v", containerID, err)
		}
	}
}

func (s *Server) reservePodName(id, name string) (string, error) {
	if err := s.podNameIndex.Reserve(name, id); err != nil {
		if err == registrar.ErrNameReserved {
			id, err := s.podNameIndex.Get(name)
			if err != nil {
				logrus.Warnf("name %s already reserved for %s", name, id)
				return "", err
			}
			return "", fmt.Errorf("conflict, name %s already reserved", name)
		}
		return "", fmt.Errorf("error reserving name %s", name)
	}
	return name, nil
}

func (s *Server) releasePodName(name string) {
	s.podNameIndex.Release(name)
}

func (s *Server) reserveContainerName(id, name string) (string, error) {
	if err := s.ctrNameIndex.Reserve(name, id); err != nil {
		if err == registrar.ErrNameReserved {
			id, err := s.ctrNameIndex.Get(name)
			if err != nil {
				logrus.Warnf("get reserved name %s failed", name)
				return "", err
			}
			return "", fmt.Errorf("conflict, name %s already reserved for %s", name, id)
		}
		return "", fmt.Errorf("error reserving name %s", name)
	}
	return name, nil
}

func (s *Server) releaseContainerName(name string) {
	s.ctrNameIndex.Release(name)
}

const (
	// SeccompModeFilter refers to the syscall argument SECCOMP_MODE_FILTER.
	SeccompModeFilter = uintptr(2)
)

func seccompEnabled() bool {
	var enabled bool
	// Check if Seccomp is supported, via CONFIG_SECCOMP.
	if _, _, err := syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_GET_SECCOMP, 0, 0); err != syscall.EINVAL {
		// Make sure the kernel has CONFIG_SECCOMP_FILTER.
		if _, _, err := syscall.RawSyscall(syscall.SYS_PRCTL, syscall.PR_SET_SECCOMP, SeccompModeFilter, 0); err != syscall.EINVAL {
			enabled = true
		}
	}
	return enabled
}

// Shutdown attempts to shut down the server's storage cleanly
func (s *Server) Shutdown() error {
	_, err := s.store.Shutdown(false)
	return err
}

// New creates a new Server with options provided
func New(config *Config) (*Server, error) {
	// TODO: This will go away later when we have wrapper process or systemd acting as
	// subreaper.
	if err := utils.SetSubreaper(1); err != nil {
		return nil, fmt.Errorf("failed to set server as subreaper: %v", err)
	}

	utils.StartReaper()

	store, err := sstorage.GetStore(sstorage.StoreOptions{
		RunRoot:            config.RunRoot,
		GraphRoot:          config.Root,
		GraphDriverName:    config.Storage,
		GraphDriverOptions: config.StorageOption,
	})
	if err != nil {
		return nil, err
	}

	imageService, err := storage.GetImageService(store, config.DefaultTransport)
	if err != nil {
		return nil, err
	}

	storageRuntimeService := storage.GetRuntimeService(imageService)
	if err != nil {
		return nil, err
	}

	r, err := oci.New(config.Runtime, config.Conmon, config.ConmonEnv)
	if err != nil {
		return nil, err
	}
	sandboxes := make(map[string]*sandbox)
	containers := oci.NewMemoryStore()
	netPlugin, err := ocicni.InitCNI("")
	if err != nil {
		return nil, err
	}
	s := &Server{
		runtime:   r,
		store:     store,
		images:    imageService,
		storage:   storageRuntimeService,
		netPlugin: netPlugin,
		config:    *config,
		state: &serverState{
			sandboxes:  sandboxes,
			containers: containers,
		},
		seccompEnabled:  seccompEnabled(),
		appArmorEnabled: apparmor.IsEnabled(),
	}
	seccompProfile, err := ioutil.ReadFile(config.SeccompProfile)
	if err != nil {
		return nil, fmt.Errorf("opening seccomp profile (%s) failed: %v", config.SeccompProfile, err)
	}
	var seccompConfig seccomp.Seccomp
	if err := json.Unmarshal(seccompProfile, &seccompConfig); err != nil {
		return nil, fmt.Errorf("decoding seccomp profile failed: %v", err)
	}
	s.seccompProfile = seccompConfig

	if s.appArmorEnabled {
		apparmor.InstallDefaultAppArmorProfile()
	}
	s.appArmorProfile = config.ApparmorProfile

	s.podIDIndex = truncindex.NewTruncIndex([]string{})
	s.podNameIndex = registrar.NewRegistrar()
	s.ctrIDIndex = truncindex.NewTruncIndex([]string{})
	s.ctrNameIndex = registrar.NewRegistrar()

	s.restore()

	logrus.Debugf("sandboxes: %v", s.state.sandboxes)
	logrus.Debugf("containers: %v", s.state.containers)
	return s, nil
}

type serverState struct {
	sandboxes  map[string]*sandbox
	containers oci.Store
}

func (s *Server) addSandbox(sb *sandbox) {
	s.stateLock.Lock()
	s.state.sandboxes[sb.id] = sb
	s.stateLock.Unlock()
}

func (s *Server) getSandbox(id string) *sandbox {
	s.stateLock.Lock()
	sb := s.state.sandboxes[id]
	s.stateLock.Unlock()
	return sb
}

func (s *Server) hasSandbox(id string) bool {
	s.stateLock.Lock()
	_, ok := s.state.sandboxes[id]
	s.stateLock.Unlock()
	return ok
}

func (s *Server) removeSandbox(id string) {
	s.stateLock.Lock()
	delete(s.state.sandboxes, id)
	s.stateLock.Unlock()
}

func (s *Server) addContainer(c *oci.Container) {
	s.stateLock.Lock()
	sandbox := s.state.sandboxes[c.Sandbox()]
	// TODO(runcom): handle !ok above!!! otherwise it panics!
	sandbox.addContainer(c)
	s.state.containers.Add(c.ID(), c)
	s.stateLock.Unlock()
}

func (s *Server) getContainer(id string) *oci.Container {
	s.stateLock.Lock()
	c := s.state.containers.Get(id)
	s.stateLock.Unlock()
	return c
}

func (s *Server) removeContainer(c *oci.Container) {
	s.stateLock.Lock()
	sandbox := s.state.sandboxes[c.Sandbox()]
	sandbox.removeContainer(c)
	s.state.containers.Delete(c.ID())
	s.stateLock.Unlock()
}
