package console

import (
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/harvester/harvester-installer/pkg/util"
)

func TestGetSSHKeysFromURL(t *testing.T) {
	testCases := []struct {
		name         string
		httpResp     string
		pubKeysCount int
		expectError  string
	}{
		{
			name:         "Two public keys",
			httpResp:     string(util.LoadFixture(t, "keys")),
			pubKeysCount: 2,
		},
		{
			name:        "Invalid public key",
			httpResp:    "\nooxx",
			expectError: "fail to parse on line 2: ooxx",
		},
		{
			name:        "No public key",
			httpResp:    "",
			expectError: "no key found",
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintln(w, testCase.httpResp)
			}))
			defer ts.Close()

			pubKeys, err := getRemoteSSHKeys(ts.URL)
			if testCase.expectError != "" {
				assert.EqualError(t, err, testCase.expectError)
			} else {
				assert.Equal(t, nil, err)
				assert.Equal(t, testCase.pubKeysCount, len(pubKeys))
			}
		})
	}
}

func TestGetHStatus(t *testing.T) {
	s := getHarvesterStatus()
	t.Log(s)
}

func TestGetFormattedServerURL(t *testing.T) {
	testCases := []struct {
		Name   string
		input  string
		output string
		err    error
	}{
		{
			Name:   "ip",
			input:  "1.2.3.4",
			output: "https://1.2.3.4:8443",
			err:    nil,
		},
		{
			Name:   "domain name",
			input:  "example.org",
			output: "https://example.org:8443",
			err:    nil,
		},
		{
			Name:   "invalid ip",
			input:  "1.2.3.4/",
			output: "",
			err:    errors.New("1.2.3.4/ is not a valid ip/domain"),
		},
		{
			Name:   "invalid domain",
			input:  "example.org/",
			output: "",
			err:    errors.New("example.org/ is not a valid ip/domain"),
		},
	}
	for _, testCase := range testCases {
		got, err := getFormattedServerURL(testCase.input)
		assert.Equal(t, testCase.output, got)
		assert.Equal(t, testCase.err, err)
	}
}

func TestF(t *testing.T) {
	ifaces, _ := net.Interfaces()
	for _, i := range ifaces {
		addrs, _ := i.Addrs()
		// handle err
		for _, addr := range addrs {
			if v, ok := addr.(*net.IPNet); ok && !v.IP.IsLoopback() && v.IP.To4() != nil {
				t.Log(v.IP.String())
			}
		}
	}
}

func TestGetServerURLFromRancherdConfig(t *testing.T) {
	testCases := []struct {
		input []byte
		url   string
		err   error
	}{
		{
			input: []byte("role: cluster-init\nkubernetesVersion: v1.21.2+rke2r1"),
			url:   "",
			err:   nil,
		},
		{
			input: []byte("role: agent\nkubernetesVersion: v1.21.2+rke2r1\nserver: https://172.0.0.1:8443"),
			url:   "https://172.0.0.1:8443",
			err:   nil,
		},
	}

	for _, testCase := range testCases {
		url, err := getServerURLFromRancherdConfig(testCase.input)
		assert.Equal(t, testCase.url, url)
		assert.Equal(t, testCase.err, err)
	}
}
