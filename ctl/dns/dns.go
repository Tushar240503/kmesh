/*
 * Copyright The Kmesh Authors.
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at:
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package dns

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/spf13/cobra"

	"kmesh.net/kmesh/ctl/utils"
	"kmesh.net/kmesh/pkg/kube"
	"kmesh.net/kmesh/pkg/logger"
)

const (
	patternDNS = "/dns"
)

var log = logger.NewLoggerScope("kmeshctl/dns")

// NewCmd returns the root dns command with its subcommands.
func NewCmd() *cobra.Command {
	dnsCmd := &cobra.Command{
		Use:   "dns",
		Short: "Manage DNS proxy in Kmesh",
	}

	dnsCmd.AddCommand(NewEnableCmd())
	dnsCmd.AddCommand(NewDisableCmd())

	return dnsCmd
}

// NewEnableCmd creates a command to enable dns proxy.
func NewEnableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "enable [podNames...]",
		Short:   "Enable DNS proxy",
		Example: "kmeshctl dns enable\nkmeshctl dns enable pod1 pod2",
		Args:    cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			// If no pod names are given, apply to all kmesh daemon pods.
			SetDNSForPods(args, "true")
			log.Info("DNS Proxy has been enabled.")
		},
	}
	return cmd
}

// NewDisableCmd creates a command to disable dns proxy.
func NewDisableCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "disable [podNames...]",
		Short:   "Disable DNS proxy",
		Example: "kmeshctl dns disable\nkmeshctl dns disable pod1 pod2",
		Args:    cobra.ArbitraryArgs,
		Run: func(cmd *cobra.Command, args []string) {
			SetDNSForPods(args, "false")
			log.Info("DNS Proxy has been disabled.")
		},
	}
	return cmd
}

// SetDNSForPods applies the dns setting (enable/disable) for the given pod(s).
// If no pod names are specified, it applies the setting to all kmesh daemon pods.
func SetDNSForPods(podNames []string, info string) {
	cli, err := utils.CreateKubeClient()
	if err != nil {
		log.Errorf("failed to create cli client: %v", err)
		os.Exit(1)
	}

	if len(podNames) == 0 {
		// Apply to all kmesh daemon pods.
		podList, err := cli.PodsForSelector(context.TODO(), utils.KmeshNamespace, utils.KmeshLabel)
		if err != nil {
			log.Errorf("failed to get kmesh podList: %v", err)
			os.Exit(1)
		}
		for _, pod := range podList.Items {
			SetDNSPerKmeshDaemon(cli, pod.GetName(), info)
		}
	} else {
		// Process for specified pods.
		for _, podName := range podNames {
			SetDNSPerKmeshDaemon(cli, podName, info)
		}
	}
}

// SetDNSPerKmeshDaemon sends a POST request to a specific kmesh daemon pod
// to set the dns flag based on the info parameter ("true" or "false").
func SetDNSPerKmeshDaemon(cli kube.CLIClient, podName, info string) {
	fw, err := utils.CreateKmeshPortForwarder(cli, podName)
	if err != nil {
		log.Errorf("failed to create port forwarder for Kmesh daemon pod %s: %v", podName, err)
		os.Exit(1)
	}
	if err := fw.Start(); err != nil {
		log.Errorf("failed to start port forwarder for Kmesh daemon pod %s: %v", podName, err)
		os.Exit(1)
	}
	defer fw.Close()

	url := fmt.Sprintf("http://%s%s?enable=%s", fw.Address(), patternDNS, info)

	req, err := http.NewRequest(http.MethodPost, url, nil)
	if err != nil {
		log.Errorf("Error creating request: %v", err)
		return
	}

	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		log.Errorf("failed to make HTTP request: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Errorf("Error: received status code %d", resp.StatusCode)
		return
	}
}
