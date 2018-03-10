// Copyright 2015 Tigera Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.
package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"time"

	"github.com/projectcalico/libcalico-go/lib/apiconfig"
	client "github.com/projectcalico/libcalico-go/lib/clientv3"
	"github.com/projectcalico/libcalico-go/lib/ipam"
	"github.com/projectcalico/libcalico-go/lib/logutils"
	cnet "github.com/projectcalico/libcalico-go/lib/net"
	"github.com/sirupsen/logrus"
)

func main() {
	// Set up logging formatting.
	logrus.SetFormatter(&logutils.Formatter{})

	// Install a hook that adds file/line no information.
	logrus.AddHook(&logutils.ContextHook{})

	// First, move the host-local plugin so it can't be used.
	os.Rename("/etc/cni/net.d/10-calico.conflist", "/etc/cni/caliconflist.tmp")
	time.Sleep(1 * time.Second)

	// Now, do the stuff.
	readFiles()
}

func readFiles() {
	// Read in all the files in the host-local directory.
	path := "/var/lib/cni/networks/k8s-pod-network"
	files, err := ioutil.ReadDir(path)
	if err != nil {
		panic(err)
	}

	nodename := nodename()

	// For each file, convert it into an IP allocation. The name of the file
	// is its IP address, and the contents are the container ID.
	for _, f := range files {
		// Skip the last reserved IP.
		if strings.Contains(f.Name(), "last") {
			continue
		}

		b, err := ioutil.ReadFile(fmt.Sprintf("%s/%s", path, f.Name()))
		if err != nil {
			panic(err)
		}

		ip, _, err := cnet.ParseCIDR(fmt.Sprintf("%s/32", f.Name()))
		if err != nil {
			panic(err)
		}

		err = assignCalicoIP(*ip, string(b), nodename)
		if err != nil {
			panic(err)
		}
	}

}

func assignCalicoIP(ip cnet.IP, containerID, nodename string) error {
	handleID := fmt.Sprintf("%s.%s", "k8s-pod-network", containerID)
	assignArgs := ipam.AssignIPArgs{
		IP:       ip,
		HandleID: &handleID,
		Hostname: nodename,
	}

	calicoClient, err := createClient()
	if err != nil {
		return err
	}

	err = calicoClient.IPAM().AssignIP(context.Background(), assignArgs)
	if err != nil {
		return err
	}
	return nil
}

func nodename() string {
	return os.Getenv("KUBERNETES_NODE_NAME")
}

func createClient() (client.Interface, error) {
	// Load the client config from the current environment.
	clientConfig, err := apiconfig.LoadClientConfig("")
	if err != nil {
		return nil, err
	}
	clientConfig.Spec.DatastoreType = apiconfig.Kubernetes

	// Create a new client.
	calicoClient, err := client.New(*clientConfig)
	if err != nil {
		return nil, err
	}
	return calicoClient, nil
}
