package test

import (
	"fmt"
	"sync"
	"time"

	"github.com/k8sdb/elasticsearch/pkg/controller"
	"k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	"k8s.io/kubernetes/pkg/client/restclient"
)

type elasticController struct {
	isControllerRunning bool
	controller          *controller.Controller
	once                sync.Once
}

var e2eController = elasticController{isControllerRunning: false}

const (
	configPath = "/home/shahriar/.kube/config"
)

func getController() (c *controller.Controller, err error) {

	// Controller is already running..
	if e2eController.isControllerRunning {
		c = e2eController.controller
		return
	}

	e2eController.once.Do(
		func() {
			fmt.Println("-- TestE2E: Waiting for controller")

			var config *restclient.Config
			config, err = clientcmd.BuildConfigFromFlags("", configPath)
			if err != nil {
				err = fmt.Errorf("Could not get kubernetes config: %s", err)
				return
			}
			c = controller.New(config, "canary", "canary", "k8sdb")

			e2eController.controller = c
			e2eController.isControllerRunning = true
			go c.RunAndHold()

			time.Sleep(time.Second * 30)
		},
	)
	return
}
