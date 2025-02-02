package settings

import (
	"crypto/tls"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path/filepath"

	"github.com/robocorp/rcc/blobs"
	"github.com/robocorp/rcc/common"
	"github.com/robocorp/rcc/pathlib"
	"github.com/robocorp/rcc/pretty"
)

const (
	pypiDefault  = "https://pypi.org/simple/"
	condaDefault = "https://conda.anaconda.org/"
)

var (
	httpTransport  *http.Transport
	cachedSettings *Settings
	Global         gateway
)

func cacheSettings(result *Settings) (*Settings, error) {
	if result != nil {
		cachedSettings = result
	}
	return result, nil
}

func SettingsFileLocation() string {
	return filepath.Join(common.RobocorpHome(), "settings.yaml")
}

func HasCustomSettings() bool {
	return pathlib.IsFile(SettingsFileLocation())
}

func DefaultSettings() ([]byte, error) {
	return blobs.Asset("assets/settings.yaml")
}

func rawSettings() (content []byte, location string, err error) {
	if HasCustomSettings() {
		location = SettingsFileLocation()
		content, err = ioutil.ReadFile(location)
		return content, location, err
	} else {
		content, err = DefaultSettings()
		return content, "builtin", err
	}
}

func SummonSettings() (*Settings, error) {
	if cachedSettings != nil {
		return cachedSettings, nil
	}
	content, source, err := rawSettings()
	if err != nil {
		return nil, err
	}
	config, err := FromBytes(content)
	if err != nil {
		return nil, err
	}
	return cacheSettings(config.Source(source))
}

func showDiagnosticsChecks(sink io.Writer, details *common.DiagnosticStatus) {
	fmt.Fprintln(sink, "Checks:")
	for _, check := range details.Checks {
		fmt.Fprintf(sink, " - %-8s %-8s %s\n", check.Type, check.Status, check.Message)
	}
}

func CriticalEnvironmentSettingsCheck() {
	config, err := SummonSettings()
	pretty.Guard(err == nil, 80, "Aborting! Could not even get setting, reason: %v", err)
	result := &common.DiagnosticStatus{
		Details: make(map[string]string),
		Checks:  []*common.DiagnosticCheck{},
	}
	config.CriticalEnvironmentDiagnostics(result)
	diagnose := result.Diagnose("Settings")
	if HasCustomSettings() {
		diagnose.Ok("Uses custom settings at %q.", SettingsFileLocation())
	} else {
		diagnose.Ok("Uses builtin settings.")
	}
	fatal, fail, _, _ := result.Counts()
	if (fatal + fail) > 0 {
		showDiagnosticsChecks(os.Stderr, result)
		pretty.Guard(false, 111, "\nBroken settings.yaml. Cannot continue!")
	}
}

func resolveLink(link, page string) string {
	docs, err := url.Parse(link)
	if err != nil {
		return page
	}
	local, err := url.Parse(page)
	if err != nil {
		return page
	}
	return docs.ResolveReference(local).String()
}

type gateway bool

func (it gateway) TemplatesYamlURL() string {
	config, err := SummonSettings()
	pretty.Guard(err == nil, 111, "Could not get settings, reason: %v", err)
	return config.Autoupdates["templates"]
}

func (it gateway) Diagnostics(target *common.DiagnosticStatus) {
	config, err := SummonSettings()
	pretty.Guard(err == nil, 111, "Could not get settings, reason: %v", err)
	config.Diagnostics(target)
}

func (it gateway) Endpoints() *Endpoints {
	config, err := SummonSettings()
	pretty.Guard(err == nil, 111, "Could not get settings, reason: %v", err)
	pretty.Guard(config.Endpoints != nil, 111, "settings.yaml: endpoints are missing")
	return config.Endpoints
}

func (it gateway) DefaultEndpoint() string {
	return it.Endpoints().CloudApi
}

func (it gateway) IssuesURL() string {
	return it.Endpoints().Issues
}

func (it gateway) TelemetryURL() string {
	return it.Endpoints().Telemetry
}

func (it gateway) PypiURL() string {
	return it.Endpoints().Pypi
}

func (it gateway) PypiTrustedHost() string {
	return justHostAndPort(it.Endpoints().PypiTrusted)
}

func (it gateway) CondaURL() string {
	return it.Endpoints().Conda
}

func (it gateway) DownloadsLink(resource string) string {
	return resolveLink(it.Endpoints().Downloads, resource)
}

func (it gateway) DocsLink(page string) string {
	return resolveLink(it.Endpoints().Docs, page)
}

func (it gateway) PypiLink(page string) string {
	endpoint := it.Endpoints().Pypi
	if len(endpoint) == 0 {
		endpoint = pypiDefault
	}
	return resolveLink(endpoint, page)
}

func (it gateway) CondaLink(page string) string {
	endpoint := it.Endpoints().Conda
	if len(endpoint) == 0 {
		endpoint = condaDefault
	}
	return resolveLink(endpoint, page)
}

func (it gateway) Hostnames() []string {
	config, err := SummonSettings()
	pretty.Guard(err == nil, 111, "Could not get settings, reason: %v", err)
	return config.Hostnames()
}

func (it gateway) ConfiguredHttpTransport() *http.Transport {
	return httpTransport
}

func init() {
	verifySsl := true
	Global = gateway(true)
	httpTransport = http.DefaultTransport.(*http.Transport).Clone()
	settings, err := SummonSettings()
	if err == nil && settings.Certificates != nil {
		verifySsl = settings.Certificates.VerifySsl
	}
	if !verifySsl {
		httpTransport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}
	}
}
