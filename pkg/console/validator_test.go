package console

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/harvester/harvester-installer/pkg/config"
	"github.com/harvester/harvester-installer/pkg/util"
)

type FakeValidator struct {
	hasInterfaces []string
	hasDevices    []string
}

func (v FakeValidator) Validate(cfg *config.HarvesterConfig) error {
	if err := v.checkMgmtInterface(cfg.Install.MgmtInterface); err != nil {
		return err
	}
	if err := v.checkDevice(cfg.Install.Device); err != nil {
		return err
	}
	return nil
}

func (v FakeValidator) checkMgmtInterface(name string) error {
	for _, i := range v.hasInterfaces {
		if i == name {
			return nil
		}
	}
	return prettyError(ErrMsgInterfaceNotFound, name)
}

func (v FakeValidator) checkDevice(device string) error {
	for _, d := range v.hasDevices {
		if d == device {
			return nil
		}
	}
	return prettyError(ErrMsgDeviceNotFound, device)
}

func createDefaultFakeValidator() FakeValidator {
	return FakeValidator{
		hasInterfaces: []string{"eth0"},
		hasDevices:    []string{"/dev/vda"},
	}
}

func loadConfig(t *testing.T, name string) *config.HarvesterConfig {
	c, err := config.LoadHarvesterConfig(util.LoadFixture(t, name))
	if err != nil {
		t.Fatal("fail to load config: ", err)
	}
	return c
}

func TestValidateConfig(t *testing.T) {
	createCreateConfig := func() *config.HarvesterConfig {
		return &config.HarvesterConfig{
			Token: "token",
			OS: config.OS{
				SSHAuthorizedKeys: []string{"github: someuser"},
				Password:          "password",
			},
			Install: config.Install{
				Mode:          config.ModeCreate,
				MgmtInterface: "eth0",
				Device:        "/dev/vda",
			},
		}
	}

	createJoinConfig := func() *config.HarvesterConfig {
		c := createCreateConfig()
		c.ServerURL = "https://somewhere"
		c.Mode = config.ModeJoin
		return c
	}

	testCases := []struct {
		name     string
		cfg      *config.HarvesterConfig
		preApply func(c *config.HarvesterConfig)
		errMsg   string
	}{
		{
			name: "valid create config",
			cfg:  createCreateConfig(),
		},
		{
			name: "invalid create config: contains server URL",
			cfg:  createCreateConfig(),
			preApply: func(c *config.HarvesterConfig) {
				c.ServerURL = "https://somewhere"
			},
			errMsg: ErrMsgModeCreateContainsServerURL,
		},
		{
			name: "invalid config: unknown mode",
			cfg:  createCreateConfig(),
			preApply: func(c *config.HarvesterConfig) {
				c.Mode = "asdf"
			},
			errMsg: ErrMsgModeUnknown,
		},
		{
			name: "valid join config",
			cfg:  createJoinConfig(),
		},
		{
			name: "invalid join config: no server URL",
			cfg:  createJoinConfig(),
			preApply: func(c *config.HarvesterConfig) {
				c.ServerURL = ""
			},
			errMsg: ErrMsgModeJoinServerURLNotSpecified,
		},
		{
			name: "invalid create config: contains no credential",
			cfg:  createCreateConfig(),
			preApply: func(c *config.HarvesterConfig) {
				c.SSHAuthorizedKeys = nil
				c.Password = ""
			},
			errMsg: ErrMsgNoCredentials,
		},
		{
			name: "invalid create config: device not found",
			cfg:  createCreateConfig(),
			preApply: func(c *config.HarvesterConfig) {
				c.Device = "/dev/vdb"
			},
			errMsg: ErrMsgDeviceNotFound,
		},
		{
			name: "invalid create config: interface not found",
			cfg:  createCreateConfig(),
			preApply: func(c *config.HarvesterConfig) {
				c.MgmtInterface = "eth1"
			},
			errMsg: ErrMsgInterfaceNotFound,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			if testCase.preApply != nil {
				testCase.preApply(testCase.cfg)
			}
			err := validateConfig(createDefaultFakeValidator(), testCase.cfg)
			if testCase.errMsg == "" {
				assert.Nil(t, err)
			} else {
				assert.NotNil(t, err)
				assert.Contains(t, err.Error(), testCase.errMsg)
			}
		})
	}
}
