package config

import (
	"bytes"
	"io/ioutil"
	"strconv"
	"strings"
	"text/template"

	"github.com/coreos/yaml"
	yipSchema "github.com/mudler/yip/pkg/schema"
)

const (
	CosLoginUser           = "rancher"
	CosRancherdConfigTempl = `{{if .server -}}
server: {{.server}}
role: agent
{{- else -}}
role: cluster-init
{{- end }}
token: {{ .token }}
kubernetesVersion: v1.21.2+rke2r1
labels:
 - harvesterhci.io/managed=true
`
	CosRKE2Config = `cni: multus,canal
disable: rke2-ingress-nginx
cluster-cidr: 10.52.0.0/16
service-cidr: 10.53.0.0/16
cluster-dns: 10.53.0.10
`
)

func toYAMLFile(o interface{}, file string) error {
	bytes, err := yaml.Marshal(o)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(file, bytes, 0644)
}

// ConvertToCOS convert HarvesterConfig to cOS configuration.
func ConvertToCOS(config *HarvesterConfig) (*yipSchema.YipConfig, error) {
	cfg, err := config.DeepCopy()
	if err != nil {
		return nil, err
	}

	initramfs := yipSchema.Stage{}
	fs := yipSchema.Stage{
		SSHKeys:     make(map[string][]string),
		Users:       make(map[string]yipSchema.User),
		Environment: make(map[string]string),
		Files:       make([]yipSchema.File, 0),
		TimeSyncd:   make(map[string]string),
	}

	// TOP
	if err := initRancherdStage(cfg.ServerURL, cfg.Token, &fs); err != nil {
		return nil, err
	}

	// OS
	fs.SSHKeys[CosLoginUser] = cfg.OS.SSHAuthorizedKeys

	for _, ff := range cfg.OS.WriteFiles {
		perm, err := strconv.ParseUint(ff.RawFilePermissions, 8, 0)
		if err != nil {
			perm = 0600
		}
		fs.Files = append(fs.Files, yipSchema.File{
			Path:        ff.Path,
			Content:     ff.Content,
			Permissions: uint32(perm),
			OwnerString: ff.Owner,
			Group:       0,
		})

	}

	initramfs.Hostname = cfg.OS.Hostname
	initramfs.Modules = cfg.OS.Modules
	initramfs.Sysctl = cfg.OS.Sysctls
	fs.TimeSyncd["NTP"] = strings.Join(cfg.OS.NTPServers, " ")
	initramfs.Dns.Nameservers = cfg.OS.DNSNameservers

	// TODO(kiefer): wicked WIFI? Can we improve `harvester-configure-network` script?
	// cloudConfig.K3OS.Wifi = copyWifi(cfg.OS.Wifi)

	// TODO(kiefer): suggest using hash value in the doc
	// Not working yet: https://github.com/rancher-sandbox/cOS-toolkit/issues/397
	fs.Users[CosLoginUser] = yipSchema.User{
		PasswordHash: cfg.OS.Password,
	}

	fs.Environment = cfg.OS.Environment

	// TODO(kiefer): Install

	cosConfig := &yipSchema.YipConfig{
		Name: "Harvester Configuration",
		Stages: map[string][]yipSchema.Stage{
			"initramfs": {initramfs},
			"fs":        {fs},
		},
	}

	return cosConfig, nil
}

func initRancherdStage(server string, token string, stage *yipSchema.Stage) error {
	context := map[string]string{}
	context["server"] = server
	context["token"] = token

	rancherdConfig := bytes.NewBufferString("")
	tmpl, err := template.New("rancherd").Parse(CosRancherdConfigTempl)
	if err != nil {
		return err
	}
	err = tmpl.Execute(rancherdConfig, context)
	if err != nil {
		return err
	}

	stage.Directories = append(stage.Directories,
		yipSchema.Directory{
			Path:        "/etc/rancher/rancherd",
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		}, yipSchema.Directory{
			Path:        "/etc/rancher/rke2/config.yaml.d",
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		})

	stage.Files = append(stage.Files,
		yipSchema.File{
			Path:        "/etc/rancher/rancherd/config.yaml",
			Content:     rancherdConfig.String(),
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		},
	)

	// server role: add network settings
	if server == "" {
		stage.Files = append(stage.Files,
			yipSchema.File{
				Path:        "/etc/rancher/rke2/config.yaml.d/99-harvester.yaml",
				Content:     CosRKE2Config,
				Permissions: 0600,
				Owner:       0,
				Group:       0,
			},
		)
	}

	return nil
}
