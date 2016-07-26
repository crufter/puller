package daemon

import (
	"bytes"
	//"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/crufter/pauler/daemon/api"
	"github.com/crufter/pauler/shared"
	"github.com/crufter/pauler/types"
	docker "github.com/fsouza/go-dockerclient"
	"github.com/hashicorp/memberlist"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"
)

func Start() error {
	log.Infof("Started daemon")
	first := true
	if len(*shared.Node) == 0 {
		hostname, err := os.Hostname()
		if err != nil {
			return err
		}
		shared.Node = &hostname
	}
	conf := memberlist.DefaultLocalConfig()
	conf.BindPort = *shared.Port
	conf.Name = *shared.Node
	list, err := memberlist.Create(conf)
	if err != nil {
		return err
	}
	if len(*shared.Join) > 0 {
		log.Infof("Joining memberlist cluster on %v", *shared.Join)
		_, err := list.Join([]string{*shared.Join})
		if err != nil {
			return err
		}
	}
	go func() {
		api.Start()
	}()
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
		if err := remove(list); err != nil {
			log.Warnf("Failed to remove services: %v", err)
			continue
		}
	}
}

func load() error {
	finfos, err := ioutil.ReadDir(*shared.Dir)
	if err != nil {
		return err
	}
	for _, finfo := range finfos {
		fname := finfo.Name()
		fcontents, err := ioutil.ReadFile(*shared.Dir + "/" + fname)
		if err != nil {
			log.Warnf("Service config can't be read: %v", err)
			continue
		}
		service := types.Service{}
		switch {
		//case strings.HasSuffix(fname, "json"):
		//	err = json.Unmarshal(fcontents, &service)
		//	if err != nil {
		//		log.Warnf("Can't read %v: %v", fname, err)
		//		continue
		//	}
		case strings.HasSuffix(fname, "yaml") || strings.HasSuffix(fname, "yml"):
			err = yaml.Unmarshal(fcontents, &service)
			if err != nil {
				log.Warnf("Can't read %v: %v", fname, err)
				continue
			}
		default:
			_, ok := shared.BadServiceFiles[fname]
			if !ok {
				log.Warnf("Service config file has unknown suffix: %v", fname)
				shared.BadServiceFiles[fname] = true
			}
			continue
		}
		if err := service.Valid(); err != nil {
			log.Warnf("Service %v failed validation: %v", service.Name, err)
			continue
		}
		// we do not care about any service definition not matching this node
		if !matchesNode(service) {
			continue
		}
		shared.Services[service.Name] = service
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

func matchesNode(service types.Service) bool {
	return *shared.Node != "" && regexp.MustCompile(service.Node).Match([]byte(*shared.Node))
}

func launch() error {
	dclient, err := getDockerClient()
	if err != nil {
		return err
	}
	containers, err := dclient.ListContainers(docker.ListContainersOptions{
		All: true,
	})
	images, err := dclient.ListImages(docker.ListImagesOptions{
		All: true,
	})
	imageIndex := map[string]docker.APIImages{}
	for _, image := range images {
		for _, repoTag := range image.RepoTags {
			imageIndex[repoTag] = image
		}
	}
	if err != nil {
		return err
	}
	for name, service := range shared.Services {
		containerExists := false
		var cont docker.APIContainers
		for _, container := range containers {
			for _, cname := range container.Names {
				if cname == "/"+name {
					containerExists = true
					cont = container
				}
			}
		}
		if containerExists {
			if cont.Labels["sum"] != service.Sum() {
				shared.ServiceChanged[service.Name] = true
			}
			img, foundImage := imageIndex[service.Repo+":"+service.Tag]
			if !foundImage {
				log.Warnf("Can't find image %v for running container %v %v", cont.Image, service.Name)
			} else if containerExists && img.Created > cont.Created {
				log.Infof("Image for %v is fresher %v %v", service.Name, img.Created, cont.Created)
				shared.ServiceOutdated[service.Name] = true
			}
		}
		if containerExists {
			continue
		}
		bash := service.GenerateBash()
		log.Infof("Spinning up container with name %v with bash %v, service %v", name, bash, service)
		outp, err := exec.Command(bash[0], bash[1:]...).Output()
		if err != nil {
			log.Warnf("Command for service %v failed with output %v and error: %v", name, string(outp), err)
		}
		log.Infof("Done spinning up %v", name)
	}
	return nil
}

// changed services
func remove(list *memberlist.Memberlist) error {
	dclient, err := getDockerClient()
	if err != nil {
		return err
	}
	for serviceName, _ := range shared.ServiceOutdated {
		log.Infof("Removing container for service %v because it has fresher image", serviceName)
		// Redundant removal here, see ServiceChanged loop
		err = dclient.RemoveContainer(docker.RemoveContainerOptions{
			ID:    serviceName,
			Force: true,
		})
		if err != nil {
			log.Warnf("Failed to remove service %v: %v", serviceName, err)
			continue
		}
		delete(shared.ServiceOutdated, serviceName)
	}
	for serviceName, _ := range shared.ServiceChanged {
		log.Infof("Removing container for service %v because it has changed", serviceName)
		err = dclient.RemoveContainer(docker.RemoveContainerOptions{
			ID:    serviceName,
			Force: true,
		})
		if err != nil {
			log.Warnf("Failed to remove service %v: %v", serviceName, err)
			continue
		}
		service := shared.Services[serviceName]
		if service.Origin != "" && service.Origin != *shared.Node { // Do not propagate change service definition coming from an other node
			continue
		}
		var err2 error
		members := []*memberlist.Node{}
		log.Info(list.Members())
		for _, member := range list.Members() {
			log.Info(member.Name, *shared.Node)
			if member.Name == *shared.Node {
				log.Info("Skipping")
				continue
			}
			members = append(members, member)
		}
		service.Origin = *shared.Node // set the origin to this node so receiving service will know to not propagate this change
		log.Infof("Broadcasting service change of %v to %v nodes", serviceName, len(members))
		for _, member := range members {
			bs, err2 := service.Marshal()
			if err2 != nil {
				panic(err2)
			}
			req, err2 := http.NewRequest("PUT", fmt.Sprintf("http://%v:%v/v1/service", member.Addr.String(), member.Port+1), bytes.NewReader(bs))
			if err2 != nil {
				log.Warn(err)
				continue
			}
			rsp, err2 := http.DefaultClient.Do(req)
			if err2 != nil {
				log.Warnf("Failed to broadcast service change to node %v: %v", member, err2)
				continue
			}
			if rsp.StatusCode != 200 {
				log.Warnf("Response status code is not 200 when talking to node %v", member)
				continue
			}
		}
		if err == nil && err2 == nil {
			delete(shared.ServiceChanged, serviceName)
		}
	}
	return nil
}
