package config

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/harvester/harvester-installer/pkg/util"
	yipSchema "github.com/mudler/yip/pkg/schema"
)

const (
	cosLoginUser = "rancher"
)

// ConvertToCOS converts HarvesterConfig to cOS configuration.
func ConvertToCOS(config *HarvesterConfig) (*yipSchema.YipConfig, error) {
	cfg, err := config.DeepCopy()
	if err != nil {
		return nil, err
	}

	initramfs := yipSchema.Stage{
		SSHKeys:   make(map[string][]string),
		Users:     make(map[string]yipSchema.User),
		TimeSyncd: make(map[string]string),
	}

	// TOP
	if err := initRancherdStage(config, &initramfs); err != nil {
		return nil, err
	}

	// OS
	initramfs.SSHKeys[cosLoginUser] = cfg.OS.SSHAuthorizedKeys

	for _, ff := range cfg.OS.WriteFiles {
		perm, err := strconv.ParseUint(ff.RawFilePermissions, 8, 0)
		if err != nil {
			perm = 0600
		}
		initramfs.Files = append(initramfs.Files, yipSchema.File{
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
	initramfs.TimeSyncd["NTP"] = strings.Join(cfg.OS.NTPServers, " ")
	initramfs.Dns.Nameservers = cfg.OS.DNSNameservers

	// TODO(kiefer): wicked WIFI? Can we improve `harvester-configure-network` script?
	// cloudConfig.K3OS.Wifi = copyWifi(cfg.OS.Wifi)

	// TODO(kiefer): suggest using hash value in the doc
	initramfs.Users[cosLoginUser] = yipSchema.User{
		PasswordHash: cfg.OS.Password,
	}

	initramfs.Environment = cfg.OS.Environment

	// TODO(kiefer): Install

	cosConfig := &yipSchema.YipConfig{
		Name: "Harvester Configuration",
		Stages: map[string][]yipSchema.Stage{
			"initramfs": {initramfs},
		},
	}

	return cosConfig, nil
}

func initRancherdStage(config *HarvesterConfig, stage *yipSchema.Stage) error {
	rancherdConfig, err := render("rancherd-config.yaml", config)
	if err != nil {
		return err
	}

	rke2Config, err := render("rke2-99-harvester.yaml", config)
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
			Content:     rancherdConfig,
			Permissions: 0600,
			Owner:       0,
			Group:       0,
		},
	)

	// server role: add network settings
	if config.ServerURL == "" {
		stage.Files = append(stage.Files,
			yipSchema.File{
				Path:        "/etc/rancher/rke2/config.yaml.d/99-harvester.yaml",
				Content:     rke2Config,
				Permissions: 0600,
				Owner:       0,
				Group:       0,
			},
		)
	}

	return nil
}
