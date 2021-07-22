package console

import (
	"fmt"
	"net"
	"os"
	"os/exec"
	"strings"
	"sync"

	"github.com/imdario/mergo"
	"github.com/jroimartin/gocui"
	"github.com/sirupsen/logrus"
	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
	"k8s.io/apimachinery/pkg/util/validation"

	"github.com/harvester/harvester-installer/pkg/config"
	"github.com/harvester/harvester-installer/pkg/util"
	"github.com/harvester/harvester-installer/pkg/version"
	"github.com/harvester/harvester-installer/pkg/widgets"
)

type UserInputData struct {
	ServerURL       string
	SSHKeyURL       string
	Password        string
	PasswordConfirm string
	Address         string
	DNSServers      string
}

const (
	NICStateNotFound = iota
	NICStateDown
	NICStateLowerDown
	NICStateUP
)

var (
	once          sync.Once
	userInputData = UserInputData{}
	mgmtNetwork   = config.Network{}
)

func (c *Console) layoutInstall(g *gocui.Gui) error {
	var err error
	once.Do(func() {
		setPanels(c)
		initPanel := askCreatePanel

		c.config.OS.NTPServers = []string{"ntp.ubuntu.com"}
		c.config.OS.Modules = []string{"kvm", "vhost_net"}

		if cfg, err := config.ReadConfig(); err == nil {
			if cfg.Install.Automatic {
				logrus.Info("Start automatic installation...")
				mergo.Merge(c.config, cfg, mergo.WithAppendSlice)
				if cfg.Install.Mode == config.ModeUpgrade {
					initPanel = upgradePanel
				} else {
					initPanel = installPanel
				}
			}
		}

		initElements := []string{
			titlePanel,
			validatorPanel,
			notePanel,
			footerPanel,
			initPanel,
		}
		var e widgets.Element
		for _, name := range initElements {
			e, err = c.GetElement(name)
			if err != nil {
				return
			}
			if err = e.Show(); err != nil {
				return
			}
		}
	})
	return err
}

func setPanels(c *Console) error {
	funcs := []func(*Console) error{
		addTitlePanel,
		addValidatorPanel,
		addNotePanel,
		addFooterPanel,
		addAskCreatePanel,
		addDiskPanel,
		addNetworkPanel,
		addServerURLPanel,
		addTokenPanel,
		addPasswordPanels,
		addSSHKeyPanel,
		addProxyPanel,
		addCloudInitPanel,
		addConfirmInstallPanel,
		addConfirmUpgradePanel,
		addInstallPanel,
		addSpinnerPanel,
		addUpgradePanel,
	}
	for _, f := range funcs {
		if err := f(c); err != nil {
			return err
		}
	}
	return nil
}

func addTitlePanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	titleV := widgets.NewPanel(c.Gui, titlePanel)
	titleV.SetLocation(maxX/8, maxY/8-3, maxX/8*7, maxY/8)
	titleV.Focus = false
	c.AddElement(titlePanel, titleV)
	return nil
}

func addValidatorPanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	validatorV := widgets.NewPanel(c.Gui, validatorPanel)
	validatorV.SetLocation(maxX/8, maxY/8+5, maxX/8*7, maxY/8*7)
	validatorV.FgColor = gocui.ColorRed
	validatorV.Wrap = true
	validatorV.Focus = false
	c.AddElement(validatorPanel, validatorV)
	return nil
}

func addNotePanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	noteV := widgets.NewPanel(c.Gui, notePanel)
	noteV.SetLocation(maxX/8, maxY/8+3, maxX/8*7, maxY/8+5)
	noteV.Wrap = true
	noteV.Focus = false
	c.AddElement(notePanel, noteV)
	return nil
}

func addFooterPanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	footerV := widgets.NewPanel(c.Gui, footerPanel)
	footerV.SetLocation(0, maxY-2, maxX, maxY)
	footerV.Focus = false
	c.AddElement(footerPanel, footerV)
	return nil
}

func addDiskPanel(c *Console) error {
	diskV, err := widgets.NewSelect(c.Gui, diskPanel, "", getDiskOptions)
	if err != nil {
		return err
	}
	diskV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			device, err := diskV.GetData()
			if err != nil {
				return err
			}
			c.config.Install.Device = device
			diskV.Close()
			return showNetworkPage(c)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			diskV.Close()
			return showNext(c, askCreatePanel)
		},
	}
	diskV.PreShow = func() error {
		diskV.Value = c.config.Install.Device
		return c.setContentByName(titlePanel, "Choose installation target. Device will be formatted")
	}
	c.AddElement(diskPanel, diskV)
	return nil
}

func getDiskOptions() ([]widgets.Option, error) {
	output, err := exec.Command("/bin/sh", "-c", `lsblk -r -o NAME,SIZE,TYPE | grep -w disk|cut -d ' ' -f 1,2`).CombinedOutput()
	if err != nil {
		return nil, err
	}
	lines := strings.Split(strings.TrimSuffix(string(output), "\n"), "\n")
	var options []widgets.Option
	for _, line := range lines {
		splits := strings.SplitN(line, " ", 2)
		if len(splits) == 2 {
			options = append(options, widgets.Option{
				Value: "/dev/" + splits[0],
				Text:  line,
			})
		}
	}

	return options, nil
}

func addAskCreatePanel(c *Console) error {
	askOptionsFunc := func() ([]widgets.Option, error) {
		options := []widgets.Option{
			{
				Value: config.ModeCreate,
				Text:  "Create a new Harvester cluster",
			}, {
				Value: config.ModeJoin,
				Text:  "Join an existing Harvester cluster",
			},
		}
		installed, err := harvesterInstalled()
		if err != nil {
			logrus.Error(err)
		} else if installed {
			options = append(options, widgets.Option{
				Value: config.ModeUpgrade,
				Text:  "Upgrade Harvester",
			})
		}
		return options, nil
	}
	// new cluster or join existing cluster
	askCreateV, err := widgets.NewSelect(c.Gui, askCreatePanel, "", askOptionsFunc)
	if err != nil {
		return err
	}
	askCreateV.FirstPage = true
	askCreateV.PreShow = func() error {
		askCreateV.Value = c.config.Install.Mode
		return c.setContentByName(titlePanel, "Choose installation mode")
	}
	askCreateV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			selected, err := askCreateV.GetData()
			if err != nil {
				return err
			}
			c.config.Install.Mode = selected
			askCreateV.Close()

			if selected == config.ModeCreate {
				c.config.ServerURL = ""
				userInputData.ServerURL = ""
			} else if selected == config.ModeUpgrade {
				return showNext(c, confirmUpgradePanel)
			}
			return showNext(c, diskPanel)
		},
	}
	c.AddElement(askCreatePanel, askCreateV)
	return nil
}

func addServerURLPanel(c *Console) error {
	serverURLV, err := widgets.NewInput(c.Gui, serverURLPanel, "Management address", false)
	if err != nil {
		return err
	}
	serverURLV.PreShow = func() error {
		c.Gui.Cursor = true
		serverURLV.Value = userInputData.ServerURL
		if err := c.setContentByName(titlePanel, "Configure management address"); err != nil {
			return err
		}
		return c.setContentByName(notePanel, serverURLNote)
	}
	serverURLV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			asyncTaskV, err := c.GetElement(spinnerPanel)
			if err != nil {
				return err
			}
			asyncTaskV.Close()

			userInputData.ServerURL, err = serverURLV.GetData()
			if err != nil {
				return err
			}

			if userInputData.ServerURL == "" {
				return c.setContentByName(validatorPanel, "Management address is required")
			}

			fmtServerURL, err := getFormattedServerURL(userInputData.ServerURL)
			if err != nil {
				return c.setContentByName(validatorPanel, err.Error())
			}
			c.CloseElement(validatorPanel)

			// focus on task panel to prevent input
			asyncTaskV.Show()

			pingServerURL := fmtServerURL + "/ping"
			spinner := NewSpinner(c.Gui, spinnerPanel, fmt.Sprintf("Checking %q...", pingServerURL))
			spinner.Start()
			go func(g *gocui.Gui) {
				if err = validatePingServerURL(pingServerURL); err != nil {
					spinner.Stop(true, err.Error())
					g.Update(func(g *gocui.Gui) error {
						return showNext(c, serverURLPanel)
					})
					return
				}
				spinner.Stop(false, "")
				c.config.ServerURL = fmtServerURL
				g.Update(func(g *gocui.Gui) error {
					serverURLV.Close()
					return showNext(c, tokenPanel)
				})
			}(c.Gui)
			return nil
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			g.Cursor = false
			serverURLV.Close()
			return showNetworkPage(c)
		},
	}
	serverURLV.PostClose = func() error {
		asyncTaskV, err := c.GetElement(spinnerPanel)
		if err != nil {
			return err
		}
		return asyncTaskV.Close()
	}
	c.AddElement(serverURLPanel, serverURLV)
	return nil
}

func addPasswordPanels(c *Console) error {
	maxX, maxY := c.Gui.Size()
	passwordV, err := widgets.NewInput(c.Gui, passwordPanel, "Password", true)
	if err != nil {
		return err
	}
	passwordConfirmV, err := widgets.NewInput(c.Gui, passwordConfirmPanel, "Confirm password", true)
	if err != nil {
		return err
	}

	passwordV.PreShow = func() error {
		passwordV.Value = userInputData.Password
		return nil
	}

	passwordVConfirm := func(g *gocui.Gui, v *gocui.View) error {
		password1V, err := c.GetElement(passwordPanel)
		if err != nil {
			return err
		}
		userInputData.Password, err = password1V.GetData()
		if err != nil {
			return err
		}
		if userInputData.Password == "" {
			return c.setContentByName(validatorPanel, "Password is required")
		}
		return showNext(c, passwordConfirmPanel)
	}
	passwordV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter:     passwordVConfirm,
		gocui.KeyArrowDown: passwordVConfirm,
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			passwordV.Close()
			passwordConfirmV.Close()
			if err := c.setContentByName(notePanel, ""); err != nil {
				return err
			}
			return showNext(c, tokenPanel)
		},
	}
	passwordV.SetLocation(maxX/8, maxY/8, maxX/8*7, maxY/8+2)
	c.AddElement(passwordPanel, passwordV)

	passwordConfirmV.PreShow = func() error {
		c.Gui.Cursor = true
		passwordConfirmV.Value = userInputData.PasswordConfirm
		c.setContentByName(notePanel, "")
		return c.setContentByName(titlePanel, "Configure the password to access the node")
	}
	passwordConfirmV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp: func(g *gocui.Gui, v *gocui.View) error {
			userInputData.PasswordConfirm, err = passwordConfirmV.GetData()
			if err != nil {
				return err
			}
			return showNext(c, passwordPanel)
		},
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			userInputData.PasswordConfirm, err = passwordConfirmV.GetData()
			if err != nil {
				return err
			}
			if userInputData.Password != userInputData.PasswordConfirm {
				return c.setContentByName(validatorPanel, "Password mismatching")
			}
			passwordV.Close()
			passwordConfirmV.Close()
			encrypted, err := util.GetEncrptedPasswd(userInputData.Password)
			if err != nil {
				return err
			}
			c.config.Password = encrypted
			return showNext(c, proxyPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			passwordV.Close()
			passwordConfirmV.Close()
			if err := c.setContentByName(notePanel, ""); err != nil {
				return err
			}
			return showNext(c, tokenPanel)
		},
	}
	passwordConfirmV.SetLocation(maxX/8, maxY/8+3, maxX/8*7, maxY/8+5)
	c.AddElement(passwordConfirmPanel, passwordConfirmV)

	return nil
}

func addSSHKeyPanel(c *Console) error {
	sshKeyV, err := widgets.NewInput(c.Gui, sshKeyPanel, "HTTP URL", false)
	if err != nil {
		return err
	}
	sshKeyV.PreShow = func() error {
		c.Gui.Cursor = true
		sshKeyV.Value = userInputData.SSHKeyURL
		if err = c.setContentByName(titlePanel, "Optional: import SSH keys"); err != nil {
			return err
		}
		return c.setContentByName(notePanel, sshKeyNote)
	}
	closeThisPage := func() error {
		c.CloseElement(notePanel)
		return sshKeyV.Close()
	}
	gotoNextPage := func() error {
		closeThisPage()
		return showNext(c, cloudInitPanel)
	}
	sshKeyV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			url, err := sshKeyV.GetData()
			if err != nil {
				return err
			}
			userInputData.SSHKeyURL = url
			c.config.SSHAuthorizedKeys = []string{}
			if url != "" {
				// focus on task panel to prevent ssh input
				asyncTaskV, err := c.GetElement(spinnerPanel)
				if err != nil {
					return err
				}
				asyncTaskV.Close()
				asyncTaskV.Show()

				spinner := NewSpinner(c.Gui, spinnerPanel, fmt.Sprintf("Checking %q...", url))
				spinner.Start()

				go func(g *gocui.Gui) {
					pubKeys, err := getRemoteSSHKeys(url)
					if err != nil {
						spinner.Stop(true, err.Error())
						g.Update(func(g *gocui.Gui) error {
							return showNext(c, sshKeyPanel)
						})
						return
					}
					spinner.Stop(false, "")
					logrus.Debug("SSH public keys: ", pubKeys)
					c.config.SSHAuthorizedKeys = pubKeys
					g.Update(func(g *gocui.Gui) error {
						return gotoNextPage()
					})
				}(c.Gui)
				return nil
			}
			return gotoNextPage()
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			closeThisPage()
			return showNext(c, proxyPanel)
		},
	}
	sshKeyV.PostClose = func() error {
		if err := c.setContentByName(notePanel, ""); err != nil {
			return err
		}
		asyncTaskV, err := c.GetElement(spinnerPanel)
		if err != nil {
			return err
		}
		return asyncTaskV.Close()
	}
	c.AddElement(sshKeyPanel, sshKeyV)
	return nil
}

func addTokenPanel(c *Console) error {
	tokenV, err := widgets.NewInput(c.Gui, tokenPanel, "Cluster token", false)
	if err != nil {
		return err
	}
	tokenV.PreShow = func() error {
		c.Gui.Cursor = true
		tokenV.Value = c.config.Token
		tokenNote := clusterTokenJoinNote
		if c.config.Install.Mode == config.ModeCreate {
			tokenNote = clusterTokenCreateNote
		}
		if err = c.setContentByName(notePanel, tokenNote); err != nil {
			return err
		}
		return c.setContentByName(titlePanel, "Configure cluster token")
	}
	closeThisPage := func() error {
		c.CloseElement(notePanel)
		return tokenV.Close()
	}
	tokenV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			token, err := tokenV.GetData()
			if err != nil {
				return err
			}
			if token == "" {
				return c.setContentByName(validatorPanel, "Cluster token is required")
			}
			c.config.Token = token
			closeThisPage()
			return showNext(c, passwordConfirmPanel, passwordPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			closeThisPage()
			if c.config.Install.Mode == config.ModeCreate {
				g.Cursor = false
				return showNetworkPage(c)
			}
			return showNext(c, serverURLPanel)
		},
	}
	c.AddElement(tokenPanel, tokenV)
	return nil
}

func showNetworkPage(c *Console) error {
	if mgmtNetwork.Method != config.NetworkMethodStatic {
		return showNext(c, askInterfacePanel, askNetworkMethodPanel, hostNamePanel)
	}
	return showNext(c, askInterfacePanel, askNetworkMethodPanel, addressPanel, gatewayPanel, dnsServersPanel, hostNamePanel)
}

func addNetworkPanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	lastY := maxY / 8
	setLocation := func(p *widgets.Panel, height int) {
		var (
			x0 = maxX / 8
			y0 = lastY
			x1 = maxX / 8 * 7
			y1 int
		)
		if height == 0 {
			y1 = maxY / 8 * 7
		} else {
			y1 = y0 + height
		}
		lastY += height
		p.SetLocation(x0, y0, x1, y1)
	}

	hostNameV, err := widgets.NewInput(c.Gui, hostNamePanel, hostNameLabel, false)
	if err != nil {
		return err
	}

	askInterfaceV, err := widgets.NewDropDown(c.Gui, askInterfacePanel, askInterfaceLabel, getNetworkInterfaceOptions)
	if err != nil {
		return err
	}

	askNetworkMethodV, err := widgets.NewDropDown(c.Gui, askNetworkMethodPanel, askNetworkMethodLabel, getNetworkMethodOptions)
	if err != nil {
		return err
	}

	addressV, err := widgets.NewInput(c.Gui, addressPanel, addressLabel, false)
	if err != nil {
		return err
	}

	gatewayV, err := widgets.NewInput(c.Gui, gatewayPanel, gatewayLabel, false)
	if err != nil {
		return err
	}

	dnsServersV, err := widgets.NewInput(c.Gui, dnsServersPanel, dnsServersLabel, false)
	if err != nil {
		return err
	}

	networkValidatorV := widgets.NewPanel(c.Gui, networkValidatorPanel)

	gotoNextPanel := func(c *Console, name string, hooks ...func() (string, error)) func(g *gocui.Gui, v *gocui.View) error {
		return func(g *gocui.Gui, v *gocui.View) error {
			c.CloseElement(networkValidatorPanel)
			for _, hook := range hooks {
				msg, err := hook()
				if err != nil {
					return err
				}
				if msg != "" {
					return c.setContentByName(networkValidatorPanel, msg)
				}
			}
			return showNext(c, name)
		}
	}

	closeThisPage := func() {
		c.CloseElements(
			hostNamePanel,
			askInterfacePanel,
			askNetworkMethodPanel,
			addressPanel,
			gatewayPanel,
			dnsServersPanel,
			networkValidatorPanel)
	}

	setupNetwork := func() ([]byte, error) {
		return applyNetworks([]config.Network{mgmtNetwork})
	}

	preGotoNextPage := func() (string, error) {
		output, err := setupNetwork()
		if err != nil {
			return fmt.Sprintf("Configure network failed: %s %s", string(output), err), nil
		}
		logrus.Infof("Network configuration is applied: %s", output)

		c.config.Networks = []config.Network{
			mgmtNetwork,
		}
		closeThisPage()
		return "", nil
	}

	getNextPagePanel := func() string {
		if c.config.Install.Mode == config.ModeCreate {
			return tokenPanel
		}
		return serverURLPanel
	}

	gotoNextPage := func() error {
		msg, err := preGotoNextPage()
		if err != nil {
			return err
		}
		if msg != "" {
			return c.setContentByName(networkValidatorPanel, msg)
		}
		return showNext(c, getNextPagePanel())
	}

	gotoPrevPage := func(g *gocui.Gui, v *gocui.View) error {
		closeThisPage()
		return showNext(c, diskPanel)
	}

	// hostNameV
	hostNameV.PreShow = func() error {
		c.Gui.Cursor = true
		hostNameV.Value = c.config.Hostname
		return c.setContentByName(titlePanel, networkTitle)
	}
	validateHostName := func() (string, error) {
		hostName, err := hostNameV.GetData()
		if err != nil {
			return "", err
		}
		if hostName == "" {
			return "must specify hostname", nil
		}
		if errs := validation.IsQualifiedName(hostName); len(errs) > 0 {
			return fmt.Sprintf("%s is not a valid hostname", hostName), nil
		}
		c.config.Hostname = hostName
		return "", nil
	}
	hostNameVConfirm := gotoNextPanel(c, askInterfacePanel, validateHostName)
	hostNameV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowDown: hostNameVConfirm,
		gocui.KeyEnter:     hostNameVConfirm,
		gocui.KeyEsc:       gotoPrevPage,
	}
	setLocation(hostNameV.Panel, 3)
	c.AddElement(hostNamePanel, hostNameV)

	// askInterfaceV
	interfaceVConfirm := func(g *gocui.Gui, v *gocui.View) error {
		selected, err := askInterfaceV.GetData()
		if err != nil {
			return err
		}
		c.CloseElement(networkValidatorPanel)
		switch nicState := getNICState(selected); nicState {
		case NICStateNotFound:
			return c.setContentByName(networkValidatorPanel, fmt.Sprintf("NIC %s not found", selected))
		case NICStateDown:
			return c.setContentByName(networkValidatorPanel, fmt.Sprintf("NIC %s is down", selected))
		case NICStateLowerDown:
			return c.setContentByName(networkValidatorPanel, fmt.Sprintf("NIC %s is down\nNetwork cable isn't plugged in", selected))
		}
		c.config.Install.MgmtInterface = selected
		mgmtNetwork.Interface = selected
		if mgmtNetwork.Method != config.NetworkMethodStatic {
			return showNext(c, askNetworkMethodPanel)
		}
		return showNext(c, dnsServersPanel, gatewayPanel, addressPanel, askNetworkMethodPanel)
	}
	askInterfaceV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp:   gotoNextPanel(c, hostNamePanel),
		gocui.KeyArrowDown: interfaceVConfirm,
		gocui.KeyEnter:     interfaceVConfirm,
		gocui.KeyEsc:       gotoPrevPage,
	}
	setLocation(askInterfaceV.Panel, 3)
	c.AddElement(askInterfacePanel, askInterfaceV)

	// askNetworkMethodV
	validateDHCPAddresses := func() (string, error) {
		if mgmtNetwork.Method == config.NetworkMethodStatic {
			return "", nil
		}
		nic, err := net.InterfaceByName(mgmtNetwork.Interface)
		if err != nil {
			return "", err
		}
		addrList, err := nic.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrList {
			if ipNet, ok := addr.(*net.IPNet); ok {
				if ipNet.IP.To4() != nil {
					_, cidrIPNet, err := net.ParseCIDR(ipNet.String())
					if err != nil {
						return err.Error(), nil
					}
					if cidrIPNet.String() != "169.254.0.0/16" {
						return "", nil
					}
				}
			}
		}
		askInterfaceV.Close()
		askInterfaceV.Show()
		if output, err := setupNetwork(); err != nil {
			return fmt.Sprintf("Configure network failed: %s", string(output)), nil
		}
		return "Can't get a valid IP address from DHCP server", nil
	}
	askNetworkMethodVConfirm := func(g *gocui.Gui, _ *gocui.View) error {
		selected, err := askNetworkMethodV.GetData()
		if err != nil {
			return err
		}
		mgmtNetwork.Method = selected
		c.CloseElement(networkValidatorPanel)
		msg, err := validateDHCPAddresses()
		if err != nil {
			return err
		}
		if msg != "" {
			return c.setContentByName(networkValidatorPanel, msg)
		}
		if selected != config.NetworkMethodStatic {
			userInputData.Address = ""
			userInputData.DNSServers = ""
			mgmtNetwork.IP = ""
			mgmtNetwork.SubnetMask = ""
			mgmtNetwork.Gateway = ""
			mgmtNetwork.DNSNameservers = nil
			c.config.OS.DNSNameservers = nil
			return gotoNextPage()
		}
		return showNext(c, dnsServersPanel, gatewayPanel, addressPanel)
	}
	askNetworkMethodV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp:   gotoNextPanel(c, askInterfacePanel),
		gocui.KeyArrowDown: askNetworkMethodVConfirm,
		gocui.KeyEnter:     askNetworkMethodVConfirm,
		gocui.KeyEsc:       gotoPrevPage,
	}
	setLocation(askNetworkMethodV.Panel, 3)
	c.AddElement(askNetworkMethodPanel, askNetworkMethodV)

	// AddressV
	addressV.PreShow = func() error {
		c.Gui.Cursor = true
		addressV.Value = userInputData.Address
		return nil
	}
	validateAddress := func() (string, error) {
		address, err := addressV.GetData()
		if err != nil {
			return "", err
		}
		if err = checkStaticRequiredString("address", address); err != nil {
			return err.Error(), nil
		}
		ip, ipNet, err := net.ParseCIDR(address)
		if err != nil {
			return err.Error(), nil
		}
		mask := ipNet.Mask
		userInputData.Address = address
		mgmtNetwork.IP = ip.String()
		mgmtNetwork.SubnetMask = fmt.Sprintf("%d.%d.%d.%d", mask[0], mask[1], mask[2], mask[3])
		return "", nil
	}
	addressVConfirm := gotoNextPanel(c, gatewayPanel, validateAddress)
	addressV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp: gotoNextPanel(c, askNetworkMethodPanel, func() (string, error) {
			userInputData.Address, err = addressV.GetData()
			return "", err
		}),
		gocui.KeyArrowDown: addressVConfirm,
		gocui.KeyEnter:     addressVConfirm,
		gocui.KeyEsc:       gotoPrevPage,
	}
	setLocation(addressV.Panel, 3)
	c.AddElement(addressPanel, addressV)

	// gatewayV
	gatewayV.PreShow = func() error {
		c.Gui.Cursor = true
		gatewayV.Value = mgmtNetwork.Gateway
		return nil
	}
	validateGateway := func() (string, error) {
		gateway, err := gatewayV.GetData()
		if err != nil {
			return "", err
		}
		if err = checkStaticRequiredString("gateway", gateway); err != nil {
			return err.Error(), nil
		}
		if err = checkIP(gateway); err != nil {
			return err.Error(), nil
		}
		mgmtNetwork.Gateway = gateway
		return "", nil
	}
	gatewayVConfirm := gotoNextPanel(c, dnsServersPanel, validateGateway)
	gatewayV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp: gotoNextPanel(c, addressPanel, func() (string, error) {
			mgmtNetwork.Gateway, err = gatewayV.GetData()
			return "", err
		}),
		gocui.KeyArrowDown: gatewayVConfirm,
		gocui.KeyEnter:     gatewayVConfirm,
		gocui.KeyEsc:       gotoPrevPage,
	}
	setLocation(gatewayV.Panel, 3)
	c.AddElement(gatewayPanel, gatewayV)

	// dnsServersV
	dnsServersV.PreShow = func() error {
		c.Gui.Cursor = true
		dnsServersV.Value = userInputData.DNSServers
		return nil
	}
	validateDNSServers := func() (string, error) {
		dnsServers, err := dnsServersV.GetData()
		if err != nil {
			return "", err
		}
		if err = checkStaticRequiredString("dns servers", dnsServers); err != nil {
			return err.Error(), nil
		}
		dnsServerList := strings.Split(dnsServers, ",")
		if err = checkIPList(dnsServerList); err != nil {
			return err.Error(), nil
		}
		userInputData.DNSServers = dnsServers
		mgmtNetwork.DNSNameservers = dnsServerList
		c.config.OS.DNSNameservers = dnsServerList
		return "", nil
	}
	dnsServersVConfirm := func(g *gocui.Gui, v *gocui.View) error {
		return gotoNextPanel(c, getNextPagePanel(), validateDNSServers, preGotoNextPage)(g, v)
	}
	dnsServersV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyArrowUp: gotoNextPanel(c, gatewayPanel, func() (string, error) {
			userInputData.DNSServers, err = dnsServersV.GetData()
			return "", err
		}),
		gocui.KeyEnter: dnsServersVConfirm,
		gocui.KeyEsc:   gotoPrevPage,
	}
	setLocation(dnsServersV.Panel, 3)
	c.AddElement(dnsServersPanel, dnsServersV)

	// networkValidatorV
	networkValidatorV.FgColor = gocui.ColorRed
	networkValidatorV.Wrap = true
	networkValidatorV.Focus = false
	setLocation(networkValidatorV, 0)
	c.AddElement(networkValidatorPanel, networkValidatorV)

	return nil
}

func getNICState(name string) int {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return NICStateNotFound
	}
	up := link.Attrs().RawFlags&unix.IFF_UP != 0
	lowerUp := link.Attrs().RawFlags&unix.IFF_LOWER_UP != 0
	if !up {
		return NICStateDown
	}
	if !lowerUp {
		return NICStateLowerDown
	}
	return NICStateUP
}

func getNetworkInterfaceOptions() ([]widgets.Option, error) {
	var options = []widgets.Option{}
	ifaces, err := net.Interfaces()
	if err != nil {
		return nil, err
	}
	for _, i := range ifaces {
		if i.Flags&net.FlagLoopback != 0 {
			continue
		}
		link, err := netlink.LinkByName(i.Name)
		if err != nil {
			return nil, err
		}
		if link.Type() != "device" {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil {
			return nil, err
		}
		var ips []string
		for _, addr := range addrs {
			if ipnet, ok := addr.(*net.IPNet); ok {
				if ipnet.IP.To4() != nil {
					ips = append(ips, ipnet.String())
				}
			}
		}
		option := widgets.Option{
			Value: i.Name,
			Text:  i.Name,
		}
		if len(ips) > 0 {
			option.Text = fmt.Sprintf("%s (%s)", i.Name, strings.Join(ips, ","))
		}
		options = append(options, option)
	}
	return options, nil
}

func getNetworkMethodOptions() ([]widgets.Option, error) {
	return []widgets.Option{
		{
			Value: config.NetworkMethodDHCP,
			Text:  networkMethodDHCPText,
		},
		{
			Value: config.NetworkMethodStatic,
			Text:  networkMethodStaticText,
		},
	}, nil
}

func addProxyPanel(c *Console) error {
	proxyV, err := widgets.NewInput(c.Gui, proxyPanel, "Proxy address", false)
	if err != nil {
		return err
	}
	proxyV.PreShow = func() error {
		c.Gui.Cursor = true
		proxyV.Value = os.Getenv("HTTP_PROXY")
		if err := c.setContentByName(titlePanel, "Optional: configure proxy"); err != nil {
			return err
		}
		return c.setContentByName(notePanel, proxyNote)
	}
	proxyV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			proxy, err := proxyV.GetData()
			if err != nil {
				return err
			}
			if proxy != "" {
				os.Setenv("HTTP_PROXY", proxy)
				os.Setenv("HTTPS_PROXY ", proxy)
			} else {
				os.Unsetenv("HTTP_PROXY")
				os.Unsetenv("HTTPS_PROXY")
			}
			proxyV.Close()
			noteV, err := c.GetElement(notePanel)
			if err != nil {
				return err
			}
			noteV.Close()
			return showNext(c, sshKeyPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			proxyV.Close()
			c.CloseElement(notePanel)
			return showNext(c, passwordConfirmPanel, passwordPanel)
		},
	}
	c.AddElement(proxyPanel, proxyV)
	return nil
}

func addCloudInitPanel(c *Console) error {
	cloudInitV, err := widgets.NewInput(c.Gui, cloudInitPanel, "HTTP URL", false)
	if err != nil {
		return err
	}
	cloudInitV.PreShow = func() error {
		c.Gui.Cursor = true
		cloudInitV.Value = c.config.Install.ConfigURL
		return c.setContentByName(titlePanel, "Optional: remote Harvester config")
	}
	gotoNextPage := func() error {
		cloudInitV.Close()
		return showNext(c, confirmInstallPanel)
	}
	cloudInitV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			configURL, err := cloudInitV.GetData()
			if err != nil {
				return err
			}
			c.config.Install.ConfigURL = configURL
			if configURL != "" {
				asyncTaskV, err := c.GetElement(spinnerPanel)
				if err != nil {
					return err
				}
				asyncTaskV.Close()
				asyncTaskV.Show()

				spinner := NewSpinner(c.Gui, spinnerPanel, fmt.Sprintf("Checking %q...", configURL))
				spinner.Start()

				go func(g *gocui.Gui) {
					if _, err = getRemoteConfig(configURL); err != nil {
						spinner.Stop(true, err.Error())
						g.Update(func(g *gocui.Gui) error {
							return showNext(c, cloudInitPanel)
						})
						return
					}
					spinner.Stop(false, "")
					g.Update(func(g *gocui.Gui) error {
						return gotoNextPage()
					})
				}(c.Gui)
				return nil
			}
			return gotoNextPage()
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			cloudInitV.Close()
			return showNext(c, sshKeyPanel)
		},
	}
	cloudInitV.PostClose = func() error {
		asyncTaskV, err := c.GetElement(spinnerPanel)
		if err != nil {
			return err
		}
		return asyncTaskV.Close()
	}
	c.AddElement(cloudInitPanel, cloudInitV)
	return nil
}

func addConfirmInstallPanel(c *Console) error {
	askOptionsFunc := func() ([]widgets.Option, error) {
		return []widgets.Option{
			{
				Value: "yes",
				Text:  "Yes",
			}, {
				Value: "no",
				Text:  "No",
			},
		}, nil
	}
	confirmV, err := widgets.NewSelect(c.Gui, confirmInstallPanel, "", askOptionsFunc)
	if err != nil {
		return err
	}
	confirmV.PreShow = func() error {
		installBytes, err := config.PrintInstall(*c.config)
		if err != nil {
			return err
		}
		options := fmt.Sprintf("install mode: %v\n", c.config.Install.Mode)
		options += fmt.Sprintf("hostname: %v\n", c.config.OS.Hostname)
		if proxy := os.Getenv("HTTP_PROXY"); proxy != "" {
			options += fmt.Sprintf("proxy address: %v\n", proxy)
		}
		if userInputData.SSHKeyURL != "" {
			options += fmt.Sprintf("ssh key url: %v\n", userInputData.SSHKeyURL)
		}
		options += string(installBytes)
		logrus.Debug("cfm cfg: ", fmt.Sprintf("%+v", c.config.Install))
		if !c.config.Install.Silent {
			confirmV.SetContent(options +
				"\nYour disk will be formatted and Harvester will be installed with \nthe above configuration. Continue?\n")
		}
		c.Gui.Cursor = false
		return c.setContentByName(titlePanel, "Confirm installation options")
	}
	confirmV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			confirmed, err := confirmV.GetData()
			if err != nil {
				return err
			}
			if confirmed == "no" {
				confirmV.Close()
				c.setContentByName(titlePanel, "")
				c.setContentByName(footerPanel, "")
				go util.SleepAndReboot()
				return c.setContentByName(notePanel, "Installation halted. Rebooting system in 5 seconds")
			}
			confirmV.Close()
			return showNext(c, installPanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			confirmV.Close()
			return showNext(c, cloudInitPanel)
		},
	}
	c.AddElement(confirmInstallPanel, confirmV)
	return nil
}

func addConfirmUpgradePanel(c *Console) error {
	askOptionsFunc := func() ([]widgets.Option, error) {
		return []widgets.Option{
			{
				Value: "yes",
				Text:  "Yes",
			}, {
				Value: "no",
				Text:  "No",
			},
		}, nil
	}
	confirmV, err := widgets.NewSelect(c.Gui, confirmUpgradePanel, "", askOptionsFunc)
	if err != nil {
		return err
	}
	confirmV.PreShow = func() error {
		return c.setContentByName(titlePanel, fmt.Sprintf("Confirm upgrading Harvester to %s?", version.Version))
	}
	confirmV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			confirmed, err := confirmV.GetData()
			if err != nil {
				return err
			}
			confirmV.Close()
			if confirmed == "no" {
				return showNext(c, askCreatePanel)
			}
			return showNext(c, upgradePanel)
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			confirmV.Close()
			return showNext(c, askCreatePanel)
		},
	}
	c.AddElement(confirmUpgradePanel, confirmV)
	return nil
}

func addInstallPanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	installV := widgets.NewPanel(c.Gui, installPanel)
	installV.PreShow = func() error {
		go func() {
			logrus.Info("Local config: ", c.config)
			if c.config.Install.ConfigURL != "" {
				printToPanel(c.Gui, fmt.Sprintf("Fetching %s...", c.config.Install.ConfigURL), installPanel)
				remoteConfig, err := retryRemoteConfig(c.config.Install.ConfigURL, c.Gui)
				if err != nil {
					logrus.Error(err)
					printToPanel(c.Gui, err.Error(), installPanel)
					return
				}
				logrus.Info("Remote config: ", remoteConfig)
				if err := mergo.Merge(c.config, remoteConfig, mergo.WithAppendSlice); err != nil {
					printToPanel(c.Gui, fmt.Sprintf("fail to merge config: %s", err), installPanel)
					return
				}
				logrus.Info("Local config (merged): ", c.config)
			}
			if c.config.Hostname == "" {
				c.config.Hostname = generateHostName()
			}
			if c.config.TTY == "" {
				c.config.TTY = getLastTTY()
			}
			if err := validateConfig(ConfigValidator{}, c.config); err != nil {
				printToPanel(c.Gui, err.Error(), installPanel)
				return
			}

			webhooks, err := PrepareWebhooks(c.config.Webhooks, getWebhookContext(c.config))
			if err != nil {
				printToPanel(c.Gui, fmt.Sprintf("invalid webhook: %s", err), installPanel)
			}

			cOSConfig, err := config.ConvertToCOS(c.config)
			if err != nil {
				printToPanel(c.Gui, err.Error(), installPanel)
				return
			}
			doInstall(c.Gui, c.config, cOSConfig, webhooks)
		}()
		return c.setContentByName(footerPanel, "")
	}
	installV.Title = " Installing Harvester "
	installV.SetLocation(maxX/8, maxY/8, maxX/8*7, maxY/8*7)
	installV.Wrap = true
	installV.Autoscroll = true
	c.AddElement(installPanel, installV)
	installV.Frame = true
	return nil
}

func addSpinnerPanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	asyncTaskV := widgets.NewPanel(c.Gui, spinnerPanel)
	asyncTaskV.SetLocation(maxX/8, maxY/8+7, maxX/8*7, maxY/8*7)
	asyncTaskV.Wrap = true
	c.AddElement(spinnerPanel, asyncTaskV)
	return nil
}

func addUpgradePanel(c *Console) error {
	maxX, maxY := c.Gui.Size()
	upgradeV := widgets.NewPanel(c.Gui, upgradePanel)
	upgradeV.PreShow = func() error {
		go doUpgrade(c.Gui)
		return c.setContentByName(footerPanel, "")
	}
	upgradeV.Title = " Upgrading Harvester "
	upgradeV.SetLocation(maxX/8, maxY/8, maxX/8*7, maxY/8*7)
	c.AddElement(upgradePanel, upgradeV)
	upgradeV.Frame = true
	return nil
}
