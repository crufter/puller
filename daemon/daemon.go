package daemon

import (
	"bytes"
	//"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
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

var (
	Services        = map[string]types.Service{}
	serviceChanged  = map[string]bool{}
	badServiceFiles = map[string]bool{}
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
	conf.BindPort = *shared.InternalPort
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
		http.HandleFunc("/v1/service", putService)
		log.Info("Starting http server")
		log.Critical(http.ListenAndServe(fmt.Sprintf(":%v", *shared.HttpPort), nil))
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

func putService(w http.ResponseWriter, r *http.Request) {
	log.Info("Putting service")
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	s := &types.Service{}
	if err := s.Unmarshal(body); err != nil {
		panic(err)
	}
	bs, err := yaml.Marshal(s)
	if err != nil {
		panic(err)
	}
	ioutil.WriteFile(*shared.Dir+"/"+s.Name+".yml", bs, os.FileMode(0777))
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
			_, ok := badServiceFiles[fname]
			if !ok {
				log.Warnf("Service config file has unknown suffix: %v", fname)
				badServiceFiles[fname] = true
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
		Services[service.Name] = service
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
	if err != nil {
		return err
	}
	for name, service := range Services {
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
		if containerExists && cont.Labels["sum"] != service.Sum() {
			serviceChanged[service.Name] = true
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
	for serviceName, _ := range serviceChanged {
		log.Infof("Removing container for service %v because it has changed", serviceName)
		err = dclient.RemoveContainer(docker.RemoveContainerOptions{
			ID:    serviceName,
			Force: true,
		})
		if err != nil {
			log.Warnf("Failed to remove service %v: %v", serviceName, err)
			continue
		}
		var err2 error
		members := []*memberlist.Node{}
		for _, member := range list.Members() {
			if member.Name == *shared.Node {
				continue
			}
			members = append(members, member)
		}
		log.Infof("Broadcasting service change of %v to %v nodes", serviceName, len(members))
		for _, member := range members {
			bs, err2 := Services[serviceName].Marshal()
			if err2 != nil {
				panic(err2)
			}
			req, err2 := http.NewRequest("PUT", fmt.Sprintf("http://%v:%v/v1/service", member.Addr.String(), *shared.HttpPort), bytes.NewReader(bs))
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
			delete(serviceChanged, serviceName)
		}
	}
	return nil
}
