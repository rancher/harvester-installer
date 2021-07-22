package console

import (
	"bufio"
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/jroimartin/gocui"
	"github.com/sirupsen/logrus"

	"github.com/harvester/harvester-installer/pkg/config"
	"github.com/harvester/harvester-installer/pkg/util"
	"github.com/harvester/harvester-installer/pkg/version"
	"github.com/harvester/harvester-installer/pkg/widgets"
)

const (
	colorBlack = iota
	colorRed
	colorGreen
	colorYellow
	colorBlue

	statusReady     = "Ready"
	statusNotReady  = "NotReady"
	statusSettingUp = "Setting up Harvester"

	logo string = `
██╗░░██╗░█████╗░██████╗░██╗░░░██╗███████╗░██████╗████████╗███████╗██████╗░
██║░░██║██╔══██╗██╔══██╗██║░░░██║██╔════╝██╔════╝╚══██╔══╝██╔════╝██╔══██╗
███████║███████║█████╔╝╚██╗░░██╔╝█████╗░░╚█████╗░░░░██║░░░█████╗░░██████╔╝
██╔══██║██╔══██║██╔══██╗░╚████╔╝░██╔══╝░░░╚═══██╗░░░██║░░░██╔══╝░░██╔══██╗
██║░░██║██║░░██║██║░░██║░░╚██╔╝░░███████╗██████╔╝░░░██║░░░███████╗██║░░██║
╚═╝░░╚═╝╚═╝░░╚═╝╚═╝░░╚═╝░░░╚═╝░░░╚══════╝╚═════╝░░░░╚═╝░░░╚══════╝╚═╝░░╚═╝`
)

type state struct {
	installed bool
	firstHost bool
}

var (
	current state
)

func (c *Console) layoutDashboard(g *gocui.Gui) error {
	once.Do(func() {
		if err := initState(); err != nil {
			logrus.Error(err)
		}
		if err := g.SetKeybinding("", gocui.KeyF12, gocui.ModNone, toShell); err != nil {
			logrus.Error(err)
		}
		logrus.Infof("state: %+v", current)
	})
	maxX, maxY := g.Size()
	if v, err := g.SetView("url", maxX/2-40, 10, maxX/2+40, 14); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Wrap = true
		go syncManagementURL(context.Background(), g)
	}
	if v, err := g.SetView("status", maxX/2-40, 14, maxX/2+40, 18); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		v.Wrap = true
		fmt.Fprintf(v, "Current status: ")
		go syncHarvesterStatus(context.Background(), g)
	}
	if v, err := g.SetView("footer", 0, maxY-2, maxX, maxY); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		fmt.Fprintf(v, "<Use F12 to switch between Harvester console and Shell>")
	}
	if err := logoPanel(g); err != nil {
		return err
	}
	return nil
}

func logoPanel(g *gocui.Gui) error {
	maxX, _ := g.Size()
	if v, err := g.SetView("logo", maxX/2-40, 1, maxX/2+40, 9); err != nil {
		if err != gocui.ErrUnknownView {
			return err
		}
		v.Frame = false
		fmt.Fprintf(v, logo)
		versionStr := "version: " + version.Version
		logoLength := 74
		nSpace := logoLength - len(versionStr)
		fmt.Fprintf(v, "\n%*s", nSpace, "")
		fmt.Fprintf(v, "%s", versionStr)
	}
	return nil
}

func toShell(g *gocui.Gui, v *gocui.View) error {
	g.Cursor = true
	maxX, _ := g.Size()
	adminPasswordFrameV := widgets.NewPanel(g, "adminPasswordFrame")
	adminPasswordFrameV.Frame = true
	adminPasswordFrameV.SetLocation(maxX/2-35, 10, maxX/2+35, 17)
	if err := adminPasswordFrameV.Show(); err != nil {
		return err
	}
	adminPasswordV, err := widgets.NewInput(g, "adminPassword", "Input password: ", true)
	if err != nil {
		return err
	}
	adminPasswordV.SetLocation(maxX/2-30, 12, maxX/2+30, 14)
	validatorV := widgets.NewPanel(g, validatorPanel)
	validatorV.SetLocation(maxX/2-30, 14, maxX/2+30, 16)
	validatorV.FgColor = gocui.ColorRed
	validatorV.Focus = false

	adminPasswordV.KeyBindings = map[gocui.Key]func(*gocui.Gui, *gocui.View) error{
		gocui.KeyEnter: func(g *gocui.Gui, v *gocui.View) error {
			passwd, err := adminPasswordV.GetData()
			if err != nil {
				return err
			}
			if validateAdminPassword(passwd) {
				return gocui.ErrQuit
			}
			if err := validatorV.Show(); err != nil {
				return err
			}
			validatorV.SetContent("Invalid credential")
			return nil
		},
		gocui.KeyEsc: func(g *gocui.Gui, v *gocui.View) error {
			g.Cursor = false
			if err := adminPasswordFrameV.Close(); err != nil {
				return err
			}
			if err := adminPasswordV.Close(); err != nil {
				return err
			}
			return validatorV.Close()
		},
	}
	return adminPasswordV.Show()
}

func validateAdminPassword(passwd string) bool {
	file, err := os.Open("/etc/shadow")
	if err != nil {
		return false
	}
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "rancher:") {
			if util.CompareByShadow(passwd, line) {
				return true
			}
			return false
		}
	}
	return false
}

func initState() error {
	envFile := config.RancherdConfigFile
	if _, err := os.Stat(envFile); os.IsNotExist(err) {
		return err
	}
	content, err := ioutil.ReadFile(envFile)
	if err != nil {
		return err
	}
	serverURL, err := getServerURLFromRancherdConfig(content)
	if err != nil {
		return err
	}

	if serverURL != "" {
		os.Setenv("KUBECONFIG", "/var/lib/rancher/rke2/agent/kubelet.kubeconfig")
	} else {
		current.firstHost = true
	}

	return nil
}

func syncManagementURL(ctx context.Context, g *gocui.Gui) {
	// sync url at the beginning
	doSyncManagementURL(g)

	syncDuration := 30 * time.Second
	ticker := time.NewTicker(syncDuration)
	go func() {
		<-ctx.Done()
		ticker.Stop()
	}()
	for range ticker.C {
		doSyncManagementURL(g)
	}
}

func doSyncManagementURL(g *gocui.Gui) {
	managementURL := "Unavailable"
	if managementIP := getFirstReadyMasterIP(); managementIP != "" {
		managementURL = fmt.Sprintf("https://%s:%s", managementIP, harvesterNodePort)
	}
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View("url")
		if err != nil {
			return err
		}
		v.Clear()
		fmt.Fprintf(v, "Harvester management URL: \n\n%s", managementURL)
		return nil
	})
}

func getFirstReadyMasterIP() string {
	// get first ready master node's internal ip
	cmd := exec.Command("/bin/sh", "-c", `kubectl get no -l 'node-role.kubernetes.io/master=true' --sort-by='.metadata.creationTimestamp' \
-o jsonpath='{range .items[*]}{@.metadata.name}:{range @.status.conditions[*]}{@.type}={@.status};{end}{range @.status.addresses[*]}{@.type}={@.address};{end}{"\n"}{end}' 2>/dev/null \
| grep 'Ready=True' | head -n 1 | tr ';' '\n' | awk -F '=' '/InternalIP/{printf $2}'`)
	cmd.Env = os.Environ()
	output, err := cmd.Output()
	outStr := string(output)
	if err != nil {
		logrus.Error(err, outStr)
		return ""
	}
	return outStr
}

func syncHarvesterStatus(ctx context.Context, g *gocui.Gui) {
	// sync status at the beginning
	doSyncHarvesterStatus(g)

	syncDuration := 30 * time.Second
	ticker := time.NewTicker(syncDuration)
	go func() {
		<-ctx.Done()
		ticker.Stop()
	}()
	for range ticker.C {
		doSyncHarvesterStatus(g)
	}
}

func doSyncHarvesterStatus(g *gocui.Gui) {
	status := getHarvesterStatus()
	g.Update(func(g *gocui.Gui) error {
		v, err := g.View("status")
		if err != nil {
			return err
		}
		v.Clear()
		fmt.Fprintln(v, "Current status: \n\n"+status)
		return nil
	})
}

func k8sIsReady() bool {
	cmd := exec.Command("/bin/sh", "-c", `kubectl get no -o jsonpath='{.items[*].metadata.name}'`)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Error(err, string(output))
		return false
	}
	if string(output) == "" {
		//no node is added
		return false
	}
	return true
}

func chartIsInstalled() bool {
	cmd := exec.Command("/bin/sh", "-c", `kubectl -n fleet-local get ManagedChart harvester -o jsonpath='{.status.conditions}' | jq 'map(select(.type == "Processed" and .status == "True")) | length'`)
	cmd.Env = os.Environ()
	output, err := cmd.Output()
	outStr := string(output)
	if err != nil {
		logrus.Error(err, outStr)
		return false
	}
	if len(outStr) == 0 {
		return false
	}
	processed, err := strconv.Atoi(strings.Trim(outStr, "\n"))
	if err != nil {
		logrus.Error(err, outStr)
		return false
	}

	return processed >= 1
}

func isPodReady(namespace, labelSelector string) bool {
	command := fmt.Sprintf(`kubectl get po -n %s -l "%s" -o jsonpath='{range .items[*]}{range @.status.conditions[*]}{@.type}={@.status};{end}{"\n"}' | grep "Ready=True"`, namespace, labelSelector)
	cmd := exec.Command("/bin/sh", "-c", command)
	cmd.Env = os.Environ()
	_, err := cmd.CombinedOutput()
	return err == nil
}

func nodeIsPresent() bool {
	hostname, err := os.Hostname()
	if err != nil {
		logrus.Errorf("failed to get hostname: %v", err)
		return false
	}

	kcmd := fmt.Sprintf("kubectl get no %s", hostname)
	cmd := exec.Command("/bin/sh", "-c", kcmd)
	cmd.Env = os.Environ()
	output, err := cmd.CombinedOutput()
	if err != nil {
		logrus.Error(err, string(output))
		return false
	}

	return true
}

func getHarvesterStatus() string {
	if current.firstHost && !current.installed {
		if !k8sIsReady() || !chartIsInstalled() {
			return statusSettingUp
		}
		current.installed = true
	}

	if !nodeIsPresent() {
		return wrapColor(statusNotReady, colorYellow)
	}

	harvesterReady := isPodReady("harvester-system", "app.kubernetes.io/name=harvester")
	rancherReady := isPodReady("cattle-system", "app=rancher")
	if harvesterReady && rancherReady {
		return wrapColor(statusReady, colorGreen)
	}
	return wrapColor(statusNotReady, colorYellow)
}

func wrapColor(s string, color int) string {
	return fmt.Sprintf("\033[3%d;7m%s\033[0m", color, s)
}
