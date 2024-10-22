package types

import (
	hash "crypto/sha1"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Service struct {
	Name string
	Bash string
	// Repo + tag enables pulling new versions of an image ie. eu.gcr.io/projectid/servicename:b3735a6bac3f17592d8344ac708ba1df4fcbd358
	Repo string // ie. eu.gcr.io/projectid/servicename
	Tag  string // ie. b3735a6bac3f17592d8344ac708ba1df4fcbd358, latest etc
	Node string // regexp on nodename to see if service should be deployed on a given instance, ie "database-box-*" should match "database-box-lvje" but not "api-box-ooek"
	// Fields not factoring into comparison with running containers
	LastUpdated time.Time `json:"-"` // the time the service definition file was last updated
	PullEvery   int64     // pull time in seconds
}

func (s Service) Valid() error {
	switch {
	case !strings.Contains(s.Bash, s.Repo):
		return errors.New("Service bash command does not contain repo")
	}
	return nil
}

func (s Service) Sum() string {
	// even the Node regexp is taken into account to ensure potential removal from the box
	return fmt.Sprintf("%x", hash.Sum([]byte(s.Bash+s.Repo+s.Tag+s.Node)))
}

// GenerateBash returns the final
func (s Service) GenerateBash() []string {
	bashParts := strings.Split(s.Bash, " ")
	for i, v := range bashParts {
		if !strings.Contains(v, s.Repo) {
			continue
		}
		if strings.Contains(v, ":") {
			bashParts[i] = strings.Split(v, ":")[0] + ":" + s.Tag
		} else {
			bashParts[i] += ":" + s.Tag
		}
	}
	return append([]string{bashParts[0], bashParts[1]}, append([]string{"--name", s.Name, "-d", "--label", "sum=" + s.Sum()}, bashParts[2:]...)...)
}

func (s Service) Marshal() ([]byte, error) {
	bs, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	return []byte(base64.StdEncoding.EncodeToString(bs)), nil
}

func (s *Service) Unmarshal(src []byte) error {
	dat, err := base64.StdEncoding.DecodeString(string(src))
	if err != nil {
		return err
	}
	return json.Unmarshal(dat, s)
}
