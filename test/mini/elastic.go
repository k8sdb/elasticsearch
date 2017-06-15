package mini

import (
	"errors"
	"fmt"
	"time"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/log"
	tapi "github.com/k8sdb/apimachinery/api"
	"github.com/k8sdb/elasticsearch/pkg/controller"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

const durationCheckElastic = time.Minute * 30

func NewElastic() *tapi.Elastic {
	elastic := &tapi.Elastic{
		ObjectMeta: metav1.ObjectMeta{
			Name: rand.WithUniqSuffix("e2e-elastic"),
		},
		Spec: tapi.ElasticSpec{
			Version:  "canary",
			Replicas: 1,
		},
	}
	return elastic
}

func CheckElasticStatus(c *controller.Controller, elastic *tapi.Elastic) (bool, error) {
	elasticReady := false
	then := time.Now()
	now := time.Now()
	for now.Sub(then) < durationCheckElastic {
		_elastic, err := c.ExtClient.Elastics(elastic.Namespace).Get(elastic.Name)
		if err != nil {
			return false, err
		}

		log.Debugf("Pod Phase: %v", _elastic.Status.Phase)

		if _elastic.Status.Phase == tapi.DatabasePhaseRunning {
			elasticReady = true
			break
		}
		time.Sleep(time.Second * 10)
		now = time.Now()
	}

	if !elasticReady {
		return false, nil
	}

	return true, nil
}

func CheckElasticWorkload(c *controller.Controller, elastic *tapi.Elastic) error {
	if _, err := c.Client.CoreV1().Services(elastic.Namespace).Get(elastic.Name, metav1.GetOptions{}); err != nil {
		return err
	}

	// SatatefulSet for Elastic database
	statefulSetName := fmt.Sprintf("%v-%v", elastic.Name, tapi.ResourceCodeElastic)
	if _, err := c.Client.AppsV1beta1().StatefulSets(elastic.Namespace).Get(statefulSetName, metav1.GetOptions{}); err != nil {
		return err
	}

	podName := fmt.Sprintf("%v-%v", statefulSetName, 0)
	pod, err := c.Client.CoreV1().Pods(elastic.Namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		return err
	}

	// If job is success
	if pod.Status.Phase != apiv1.PodRunning {
		return errors.New("Pod is not running")
	}

	return nil
}

func DeleteElastic(c *controller.Controller, elastic *tapi.Elastic) error {
	return c.ExtClient.Elastics(elastic.Namespace).Delete(elastic.Name)
}

func UpdateElastic(c *controller.Controller, elastic *tapi.Elastic) (*tapi.Elastic, error) {
	return c.ExtClient.Elastics(elastic.Namespace).Update(elastic)
}
