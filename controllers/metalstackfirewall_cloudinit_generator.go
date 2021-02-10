package controllers

import (
	"bytes"
	"fmt"
	"strings"
	"text/template"
)

const (
	firewallCloudInitTemplateName   = "firewall cloud-init"
	firewallCloudInitConfigTemplate = `#cloud-config

write_files:
-   path: /etc/firewall-controller/.kubeconfig
    owner: root:root
    permissions: '0600'
    content: |
{{. | Indent 6}}

runcmd:
	- "sudo systemctl start firewall-controller"
`
)

var (
	defaultTemplateFuncMap = template.FuncMap{
		"Indent": templateYAMLIndent,
	}
)

func templateYAMLIndent(i int, input string) string {
	split := strings.Split(input, "\n")
	ident := "\n" + strings.Repeat(" ", i)
	return strings.Repeat(" ", i) + strings.Join(split, ident)
}

func generateFirewallCloudInitConfig(kubeconfig []byte) (string, error) {
	tmpl, err := template.
		New(firewallCloudInitTemplateName).
		Funcs(defaultTemplateFuncMap).
		Parse(firewallCloudInitConfigTemplate)
	if err != nil {
		return "", fmt.Errorf("Failed to create %s template: %w", firewallCloudInitTemplateName, err)
	}

	var out bytes.Buffer
	if err := tmpl.Execute(&out, string(kubeconfig)); err != nil {
		return "", fmt.Errorf("Failed to generate %s template: %w", firewallCloudInitTemplateName, err)
	}

	return out.String(), nil
}
