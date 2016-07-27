package api

import (
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/crufter/puller/shared"
	"github.com/crufter/puller/types"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os"
)

func Start() {
	http.HandleFunc("/v1/service", putService)
	log.Info("Starting http server")
	log.Critical(http.ListenAndServe(fmt.Sprintf(":%v", *shared.Port+1), nil))
}

func putService(w http.ResponseWriter, r *http.Request) {
	log.Debug("Putting service")
	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		panic(err)
	}
	b64encoded := []string{}
	err = json.Unmarshal(body, &b64encoded)
	if err != nil {
		panic(err)
	}
	ss := []types.Service{}
	for _, v := range b64encoded {
		s := &types.Service{}
		if err := s.Unmarshal([]byte(v)); err != nil {
			panic(err)
		}
		ss = append(ss, *s)
	}
	err = updateFresherServices(ss)
	if err != nil {
		panic(err)
	}
}

func updateFresherServices(services []types.Service) error {
	for _, v := range services {
		current, ok := shared.Services.Get(v.Name)
		if !ok || (v.Sum() != current.(types.Service).Sum() && v.LastUpdated.After(current.(types.Service).LastUpdated)) {
			log.Infof("Writing file for %v, new: %v", v.Name, !ok)
			bs, err := yaml.Marshal(v)
			if err != nil {
				panic(err)
			}
			err = ioutil.WriteFile(*shared.Dir+"/"+v.Name+".yml", bs, os.FileMode(0777))
			if err != nil {
				panic(err)
			}
		}
	}
	return nil
}
