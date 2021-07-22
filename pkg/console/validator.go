package console

import (
	"fmt"
	"net"

	"github.com/pkg/errors"
	"github.com/sirupsen/logrus"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/harvester/harvester-installer/pkg/config"
)

var (
	ErrMsgModeCreateContainsServerURL   = fmt.Sprintf("ServerURL need to be empty in %s mode", config.ModeCreate)
	ErrMsgModeJoinServerURLNotSpecified = fmt.Sprintf("ServerURL can't empty in %s mode", config.ModeJoin)
	ErrMsgModeUnknown                   = "unknown mode"
	ErrMsgTokenNotSpecified             = "token not specified"

	ErrMsgMgmtInterfaceNotSpecified = "no management interface specified"
	ErrMsgInterfaceNotSpecified     = "no interface specified"
	ErrMsgInterfaceNotFound         = "interface not found"
	ErrMsgInterfaceIsLoop           = "interface is a loopback interface"
	ErrMsgDeviceNotSpecified        = "no device specified"
	ErrMsgDeviceNotFound            = "device not found"
	ErrMsgNoCredentials             = "no SSH authorized keys or passwords are set"

	ErrMsgNetworkMethodUnknown = "unknown network method"
)

type ValidatorInterface interface {
	Validate(cfg *config.HarvesterConfig) error
}

type ConfigValidator struct {
}

func prettyError(errMsg string, value string) error {
	return errors.Errorf("%s: %s", errMsg, value)
}

func checkInterface(name string) error {
	if name == "" {
		return errors.New(ErrMsgInterfaceNotSpecified)
	}
	ifaces, err := net.Interfaces()
	if err != nil {
		return err
	}
	for _, i := range ifaces {
		if i.Name == name {
			if i.Flags&net.FlagLoopback != 0 {
				return prettyError(ErrMsgInterfaceIsLoop, name)
			}
			return nil
		}
	}
	return prettyError(ErrMsgInterfaceNotFound, name)
}

func checkDevice(device string) error {
	if device == "" {
		return errors.New(ErrMsgDeviceNotSpecified)
	}
	options, err := getDiskOptions()
	if err != nil {
		return err
	}
	for _, option := range options {
		if device == option.Value {
			return nil
		}
	}
	return prettyError(ErrMsgDeviceNotFound, device)
}

func checkStaticRequiredString(field, value string) error {
	if len(value) == 0 {
		return fmt.Errorf("must specify %s in static method", field)
	}
	return nil
}

func checkStaticRequiredSlice(field string, value []string) error {
	if len(value) == 0 {
		return fmt.Errorf("must specify %s in static method", field)
	}
	return nil
}

func checkDomain(domain string) error {
	if errs := validation.IsDNS1123Subdomain(domain); len(errs) > 0 {
		return fmt.Errorf("%s is not a valid domain", domain)
	}
	return nil
}

func checkIP(addr string) error {
	if ip := net.ParseIP(addr); ip == nil || ip.To4() == nil {
		return fmt.Errorf("%s is not a valid IP address", addr)
	}
	return nil
}

func checkIPList(ipList []string) error {
	for _, ip := range ipList {
		if err := checkIP(ip); err != nil {
			return err
		}
	}
	return nil
}

func checkNetworks(networks []config.Network) error {
	for _, network := range networks {
		if err := checkInterface(network.Interface); err != nil {
			return err
		}
		switch networkMethod := network.Method; networkMethod {
		case config.NetworkMethodDHCP, "":
			return nil
		case config.NetworkMethodStatic:
			if err := checkStaticRequiredString("ip", network.IP); err != nil {
				return err
			}
			if err := checkIP(network.IP); err != nil {
				return err
			}
			if err := checkStaticRequiredString("subnetMask", network.SubnetMask); err != nil {
				return err
			}
			if err := checkIP(network.SubnetMask); err != nil {
				return err
			}
			if err := checkStaticRequiredString("gateway", network.Gateway); err != nil {
				return err
			}
			if err := checkIP(network.Gateway); err != nil {
				return err
			}
			if err := checkStaticRequiredSlice("dns servers", network.DNSNameservers); err != nil {
				return err
			}
			if err := checkIPList(network.DNSNameservers); err != nil {
				return err
			}
		default:
			return prettyError(ErrMsgNetworkMethodUnknown, networkMethod)
		}
	}

	return nil
}

func (v ConfigValidator) Validate(cfg *config.HarvesterConfig) error {
	if cfg.Install.Mode == config.ModeCreate && cfg.Install.MgmtInterface == "" {
		return errors.New(ErrMsgMgmtInterfaceNotSpecified)
	}

	if err := checkInterface(cfg.Install.MgmtInterface); err != nil {
		return err
	}

	if err := checkDevice(cfg.Install.Device); err != nil {
		return err
	}

	if err := checkNetworks(cfg.Install.Networks); err != nil {
		return err
	}

	return nil
}

func commonCheck(cfg *config.HarvesterConfig) error {
	// modes
	switch mode := cfg.Install.Mode; mode {
	case config.ModeUpgrade:
		return nil
	case config.ModeCreate:
		if cfg.ServerURL != "" {
			return errors.New(ErrMsgModeCreateContainsServerURL)
		}
	case config.ModeJoin:
		if cfg.ServerURL == "" {
			return errors.New(ErrMsgModeJoinServerURLNotSpecified)
		}
	default:
		return prettyError(ErrMsgModeUnknown, mode)
	}

	if cfg.Token == "" {
		return errors.New(ErrMsgTokenNotSpecified)
	}

	if len(cfg.SSHAuthorizedKeys) == 0 && cfg.Password == "" {
		return errors.New(ErrMsgNoCredentials)
	}
	return nil
}

func validateConfig(v ValidatorInterface, cfg *config.HarvesterConfig) error {
	logrus.Debug("Validating config: ", cfg)
	if err := commonCheck(cfg); err != nil {
		return err
	}
	return v.Validate(cfg)
}
