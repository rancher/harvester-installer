package console

import (
	"io/ioutil"
	"os"
	"os/exec"

	yipSchema "github.com/mudler/yip/pkg/schema"
	"gopkg.in/yaml.v2"

	"github.com/harvester/harvester-installer/pkg/config"
)

func applyNetworks(networks []config.Network) ([]byte, error) {
	conf := &yipSchema.YipConfig{
		Name: "Network Configuration",
		Stages: map[string][]yipSchema.Stage{
			"live": {
				yipSchema.Stage{},
			},
		},
	}
	err := config.UpdateNetworkConfig(&conf.Stages["live"][0], networks, true)
	if err != nil {
		return nil, err
	}

	tempFile, err := ioutil.TempFile("/tmp", "live.XXXXXXXX")
	if err != nil {
		return nil, err
	}
	defer tempFile.Close()

	bytes, err := yaml.Marshal(conf)
	if err != nil {
		return nil, err
	}
	if _, err := tempFile.Write(bytes); err != nil {
		return nil, err
	}
	defer os.Remove(tempFile.Name())

	cmd := exec.Command("/usr/bin/yip", "-s", "live", tempFile.Name())
	cmd.Env = os.Environ()
	return cmd.CombinedOutput()
}
