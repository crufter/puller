package main

import (
	"encoding/json"
	"errors"
	log "github.com/cihub/seelog"
	docker "github.com/fsouza/go-dockerclient"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os/exec"
	"strings"
	"sync"
	"time"
)

const (
	dir = "/etc/pauler"
)

func main() {
	first := true
	for {
		if !first {
			time.Sleep(1 * time.Second)
		}
		first = false
		if err := load(); err != nil {
			log.Warnf("Failed to load service definitions: %v", err)
			continue
		}
		if err := launch(); err != nil {
			log.Warnf("Failed to launch services: %v", err)
			continue
		}
	}
}

type Service struct {
	Name string
	Bash string
	// Repo + tag enables pulling new versions of an image ie. eu.gcr.io/projectid/servicename:b3735a6bac3f17592d8344ac708ba1df4fcbd358
	Repo string // ie. eu.gcr.io/projectid/servicename
	Tag  string // ie. b3735a6bac3f17592d8344ac708ba1df4fcbd358, latest etc
}

func (a Service) Differs(b Service) bool {
	switch {
	case a.Bash != b.Bash:
		return true
	case a.Repo != b.Repo:
		return true
	case b.Tag != b.Tag:
		return true
	}
	return false
}

func (s Service) Valid() error {
	switch {
	case !strings.Contains(s.Bash, s.Repo):
		return errors.New("Service bash command does not contain repo")
	}
	return nil
}

// GenerateBash returns the final
func (s Service) GenerateBash() []string {
	bashParts := strings.Split(s.Bash, " ")
	return append([]string{bashParts[0]}, append([]string{"--name", s.Name}, bashParts[1:]...)...)
}

var (
	Services       map[string]Service
	serviceChanged map[string]bool
)

func load() error {
	if Services == nil {
		Services = map[string]Service{}
	}
	finfos, err := ioutil.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, finfo := range finfos {
		fname := finfo.Name()
		fcontents, err := ioutil.ReadFile(dir + "/" + fname)
		if err != nil {
			log.Warnf("Service config can't be read: %v", err)
			continue
		}
		service := Service{}
		switch {
		case strings.HasSuffix(fname, "json"):
			err = json.Unmarshal(fcontents, &service)
			if err != nil {
				log.Warnf("Can't read %v: %v", fname, err)
				continue
			}
		case strings.HasSuffix(fname, "yaml") || strings.HasSuffix(fname, "yml"):
			err = yaml.Unmarshal(fcontents, &service)
			if err != nil {
				log.Warnf("Can't read %v: %v", fname, err)
				continue
			}
		default:
			log.Warnf("Service config file has unknown suffix: %v", fname)
			continue
		}
		if err := service.Valid(); err != nil {
			log.Warnf("Service %v failed validation: %v", service.Name, err)
			continue
		}
		s, ok := Services[service.Name]
		if !ok {
			Services[service.Name] = service
			continue
		}
		if s.Differs(service) {
			serviceChanged[service.Name] = true
		}
	}
	return nil
}

var (
	mtx    sync.Mutex
	client *docker.Client
)

func getDockerClient() (*docker.Client, error) {
	if client != nil {
		return client, nil
	}
	mtx.Lock()
	defer mtx.Unlock()
	var err error
	client, err = docker.NewClientFromEnv()
	if err != nil {
		return nil, err
	}
	return client, nil
}

func launch() error {
	dclient, err := getDockerClient()
	if err != nil {
		return err
	}
	containers, err := dclient.ListContainers(docker.ListContainersOptions{
		All: true,
	})
	if err != nil {
		return err
	}
	for name, service := range Services {
		containerExists := false
		for _, container := range containers {
			for _, cname := range container.Names {
				if cname == "/"+name {
					containerExists = true
				}
			}
		}
		if containerExists {
			continue
		}
		log.Infof("Spinning up container with name %v, service %v", name, service)
		bash := service.GenerateBash()
		outp, err := exec.Command(bash[0], bash[1:]...).Output()
		if err != nil {
			log.Warnf("Command for service %v failed with output %v and error: %v", name, string(outp), err)
		}
	}
	return nil
}
