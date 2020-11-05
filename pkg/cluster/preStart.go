/*
Copyright © 2020 The k3d Author(s)

Permission is hereby granted, free of charge, to any person obtaining a copy
of this software and associated documentation files (the "Software"), to deal
in the Software without restriction, including without limitation the rights
to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
copies of the Software, and to permit persons to whom the Software is
furnished to do so, subject to the following conditions:

The above copyright notice and this permission notice shall be included in
all copies or substantial portions of the Software.

THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
THE SOFTWARE.
*/
package cluster

import (
	"context"
	"fmt"

	k3drt "github.com/rancher/k3d/v3/pkg/runtimes"
	k3d "github.com/rancher/k3d/v3/pkg/types"
	log "github.com/sirupsen/logrus"
)

// prepInjectHostIP adds /etc/hosts and CoreDNS entry for host.k3d.internal, referring to the host system
func prepInjectHostIP(ctx context.Context, runtime k3drt.Runtime, cluster *k3d.Cluster) {
	if runtime == k3drt.Docker {
		log.Infoln("(Optional) Trying to get IP of the docker host and inject it into the cluster as 'host.k3d.internal' for easy access")
		hostIP, err := GetHostIP(ctx, runtime, cluster)
		if err != nil {
			log.Warnf("Failed to get HostIP: %+v", err)
		}
		if hostIP != nil {
			hostRecordSuccessMessage := ""
			etcHostsFailureCount := 0
			hostsEntry := fmt.Sprintf("%s %s", hostIP, k3d.DefaultK3dInternalHostRecord)
			log.Debugf("Adding extra host entry '%s'...", hostsEntry)
			for _, node := range cluster.Nodes {
				if err := runtime.ExecInNode(ctx, node, []string{"sh", "-c", fmt.Sprintf("echo '%s' >> /etc/hosts", hostsEntry)}); err != nil {
					log.Warnf("Failed to add extra entry '%s' to /etc/hosts in node '%s'", hostsEntry, node.Name)
					etcHostsFailureCount++
				}
			}
			if etcHostsFailureCount < len(cluster.Nodes) {
				hostRecordSuccessMessage += fmt.Sprintf("Successfully added host record to /etc/hosts in %d/%d nodes", (len(cluster.Nodes) - etcHostsFailureCount), len(cluster.Nodes))
			}

			patchCmd := `test=$(kubectl get cm coredns -n kube-system --template='{{.data.NodeHosts}}' | sed -n -E -e '/[0-9\.]{4,12}\s+host\.k3d\.internal$/!p' -e '$a` + hostsEntry + `' | tr '\n' '^' | busybox xargs -0 printf '{"data": {"NodeHosts":"%s"}}'| sed -E 's%\^%\\n%g') && kubectl patch cm coredns -n kube-system -p="$test"`
			if err = runtime.ExecInNode(ctx, cluster.Nodes[0], []string{"sh", "-c", patchCmd}); err != nil {
				log.Warnf("Failed to patch CoreDNS ConfigMap to include entry '%s': %+v", hostsEntry, err)
			} else {
				hostRecordSuccessMessage += " and to the CoreDNS ConfigMap"
			}

			if hostRecordSuccessMessage != "" {
				log.Infoln(hostRecordSuccessMessage)
			}

		}
	}
}
