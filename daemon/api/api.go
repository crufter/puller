package api

import (
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/crufter/pauler/shared"
	"github.com/crufter/pauler/types"
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
	err = ioutil.WriteFile(*shared.Dir+"/"+s.Name+".yml", bs, os.FileMode(0777))
	if err != nil {
		panic(err)
	}
}
