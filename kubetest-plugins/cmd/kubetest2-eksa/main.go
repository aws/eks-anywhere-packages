package main

import (
	"sigs.k8s.io/kubetest2/pkg/app"

	"github.com/aws/modelrocket-add-ons/kubetest-plugins/cmd/kubetest2-eksa/deployer"
)

func main() {
	app.Main(deployer.Name, deployer.New)
}
