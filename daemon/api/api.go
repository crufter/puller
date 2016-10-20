package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	log "github.com/cihub/seelog"
	"github.com/crufter/puller/daemon"
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
	r.PUT("/v1/services", auth(putServices))
	r.GET("/v1/services", auth(getServices))
	r.GET("/v1/services/:name", auth(getService))
	r.GET("/v1/propagate-and-pull/:serviceName", auth(pullAndPropagate))
	r.GET("/v1/pull/:serviceName", auth(pull))
	r.GET("/v1/health", health)
	//r.GET("/v1/members", getMembers)
	log.Info("Starting http server")
	log.Critical(http.ListenAndServe(fmt.Sprintf(":%v", *shared.Port+1), r))
}

func auth(f func(w http.ResponseWriter, r *http.Request, p httpr.Params)) func(w http.ResponseWriter, r *http.Request, p httpr.Params) {
	return func(w http.ResponseWriter, r *http.Request, p httpr.Params) {
		if len(*shared.ApiKey) > 0 && *shared.ApiKey != r.Header.Get("authorization") {
			panic("not authorized")
		}
	}
}

func pull(w http.ResponseWriter, r *http.Request, p httpr.Params) {
	s := p.ByName("serviceName")
	log.Infof("Received pull for %v", s)
	daemon.Pull(false, s)
}

func health(w http.ResponseWriter, r *http.Request, p httpr.Params) {
	return
}

func pullAndPropagate(w http.ResponseWriter, r *http.Request, p httpr.Params) {
	s := p.ByName("serviceName")
	log.Infof("Received pull and propagate for %s", s)
	// @todo this should be more robust, use gossip
	for _, member := range shared.List.Members() {
		req, err := http.NewRequest("GET", fmt.Sprintf("http://%v:%v/v1/pull/"+s, member.Addr.String(), member.Port+1), bytes.NewReader([]byte{}))
		if err != nil {
			log.Infof("Failed to build request: %v", err)
			continue
		}
		rsp, err := http.DefaultClient.Do(req)
		if err != nil {
			log.Infof("Failed to broadcast pull to node %v: %v", member, err)
			continue
		}
		if rsp.StatusCode != 200 {
			log.Infof("Response status code is not 200 when broadcasting pull to node %v, response: %v", member, rsp)
			continue
		}
	}
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
