package houdini

import (
	"errors"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"code.cloudfoundry.org/garden"
	"github.com/charlievieth/fs"
	"github.com/concourse/atc"
)

var (
	ErrContainerNotFound = errors.New("container not found")
)

type Backend struct {
	containersDir string

	containers  map[string]*container
	containersL sync.RWMutex

	containerNum uint32
}

func NewBackend(containersDir string) *Backend {
	return &Backend{
		containersDir: containersDir,

		containers: make(map[string]*container),

		containerNum: uint32(time.Now().UnixNano()),
	}
}

func (backend *Backend) Start() error {
	return fs.MkdirAll(backend.containersDir, 0755)
}

func (backend *Backend) Stop() {
	containers, _ := backend.Containers(nil)

	for _, container := range containers {
		backend.Destroy(container.Handle())
	}
}

func (backend *Backend) GraceTime(c garden.Container) time.Duration {
	return c.(*container).currentGraceTime()
}

func (backend *Backend) Ping() error {
	return nil
}

func (backend *Backend) Capacity() (garden.Capacity, error) {
	return garden.Capacity{
		MaxContainers: 1,
	}, nil
}

func (backend *Backend) Create(spec garden.ContainerSpec) (garden.Container, error) {
	backend.containersL.Lock()
	defer backend.containersL.Unlock()

	capacity, err := backend.Capacity()
	if err != nil {
		return nil, err
	}

	activeContainers := 0
	for _, container := range backend.containers {
		if _, ok := container.currentProperties()["concourse:exit-status"]; !ok {
			activeContainers++
		}
	}

	if activeContainers >= int(capacity.MaxContainers) {
		return nil, atc.WorkerNotCreatedError{errors.New("worker already has the maximum number of active containers")}
	}

	id := backend.generateContainerID()

	if spec.Handle == "" {
		spec.Handle = id
	}

	dir := filepath.Join(backend.containersDir, id)

	err = fs.MkdirAll(dir, 0755)
	if err != nil {
		return nil, err
	}

	container := newContainer(spec, dir)
	backend.containers[spec.Handle] = container

	return container, nil
}

func (backend *Backend) Destroy(handle string) error {
	backend.containersL.Lock()
	defer backend.containersL.Unlock()

	container, found := backend.containers[handle]
	if !found {
		return nil
	}

	err := container.Stop(false)
	if err != nil {
		return err
	}

	err = fs.RemoveAll(container.workDir)
	if err != nil {
		return err
	}

	delete(backend.containers, handle)

	return nil
}

func (backend *Backend) Containers(filter garden.Properties) ([]garden.Container, error) {
	backend.containersL.RLock()
	defer backend.containersL.RUnlock()

	return backend.filterContainers(filter)
}

func (backend *Backend) filterContainers(filter garden.Properties) ([]garden.Container, error) {
	matchingContainers := []garden.Container{}

	for _, container := range backend.containers {
		if containerHasProperties(container, filter) {
			matchingContainers = append(matchingContainers, container)
		}
	}

	return matchingContainers, nil
}

func (backend *Backend) BulkInfo(handles []string) (map[string]garden.ContainerInfoEntry, error) {
	return map[string]garden.ContainerInfoEntry{}, nil
}

func (backend *Backend) BulkMetrics(handles []string) (map[string]garden.ContainerMetricsEntry, error) {
	return map[string]garden.ContainerMetricsEntry{}, nil
}

func (backend *Backend) Lookup(handle string) (garden.Container, error) {
	backend.containersL.RLock()
	defer backend.containersL.RUnlock()

	container, found := backend.containers[handle]
	if !found {
		return nil, ErrContainerNotFound
	}

	return container, nil
}

func (backend *Backend) generateContainerID() string {
	containerNum := atomic.AddUint32(&backend.containerNum, 1)

	containerID := []byte{}

	var i uint64
	for i = 0; i < 11; i++ {
		containerID = strconv.AppendUint(
			containerID,
			(uint64(containerNum)>>(55-(i+1)*5))&31,
			32,
		)
	}

	return string(containerID)
}

func containerHasProperties(container *container, properties garden.Properties) bool {
	containerProps := container.currentProperties()

	for key, val := range properties {
		cval, ok := containerProps[key]
		if !ok {
			return false
		}

		if cval != val {
			return false
		}
	}

	return true
}
