package controllers

import (
	"encoding/json"
	"fmt"

	"github.com/coreos/container-linux-config-transpiler/config/types"
)

const (
	firewallControllerName = "firewall-controller"
)

func generateFirewallIgnitionConfig(kubeconfig []byte) (string, error) {
	cfg := types.Config{}

	cfg.Systemd = types.Systemd{}
	enabled := true
	fcUnit := types.SystemdUnit{
		Name:    fmt.Sprintf("%s.service", firewallControllerName),
		Enable:  enabled,
		Enabled: &enabled,
	}
	cfg.Systemd.Units = append(cfg.Systemd.Units, fcUnit)

	cfg.Storage = types.Storage{}
	mode := 0600
	id := 0
	ignitionFile := types.File{
		Path:       "/etc/firewall-controller/.kubeconfig",
		Filesystem: "root",
		Mode:       &mode,
		User: &types.FileUser{
			Id: &id,
		},
		Group: &types.FileGroup{
			Id: &id,
		},
		Contents: types.FileContents{
			Inline: string(kubeconfig),
		},
	}
	cfg.Storage.Files = append(cfg.Storage.Files, ignitionFile)

	outCfg, report := types.Convert(cfg, "", nil)
	if report.IsFatal() {
		return "", fmt.Errorf("Could not transpile ignition config: %s", report.String())
	}

	userData, err := json.Marshal(outCfg)
	if err != nil {
		return "", err
	}

	return string(userData), nil
}
