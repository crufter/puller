package api

import (
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/crufter/puller/shared"
	"github.com/crufter/puller/types"
	httpr "github.com/julienschmidt/httprouter"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"os"
)

func Start() {
	r := httpr.New()
	r.PUT("/v1/services", putServices)
	r.GET("/v1/services", getServices)
	r.GET("/v1/services/:name", getService)
	//r.GET("/v1/members", getMembers)
	log.Info("Starting http server")
	log.Critical(http.ListenAndServe(fmt.Sprintf(":%v", *shared.Port+1), r))
}

func putServices(w http.ResponseWriter, r *http.Request, p httpr.Params) {
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
	log.Debugf("Putting services %v", ss)
	updateFresherServices(ss)
}

func updateFresherServices(services []types.Service) {
	for _, v := range services {
		updateFresherService(v)
	}
}

func updateFresherService(v types.Service) {
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

func getServices(w http.ResponseWriter, r *http.Request, p httpr.Params) {
	smap := shared.Services.Items()
	ss := []types.Service{}
	for _, s := range smap {
		ss = append(ss, s.(types.Service))
	}
	bs, err := json.Marshal(ss)
	if err != nil {
		panic(err)
	}
	fmt.Fprint(w, string(bs))
}

func getService(w http.ResponseWriter, r *http.Request, p httpr.Params) {
	smap := shared.Services.Items()
	for name, s := range smap {
		if name == p.ByName("name") {
			bs, err := json.Marshal(s)
			if err != nil {
				panic(err)
			}
			fmt.Fprint(w, string(bs))
		}
	}
}
