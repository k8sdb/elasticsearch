package mini

import (
	"time"

	"github.com/appscode/log"
	tapi "github.com/k8sdb/apimachinery/api"
	"github.com/k8sdb/elasticsearch/pkg/controller"
	k8serr "k8s.io/kubernetes/pkg/api/errors"
)

const durationCheckDormantDatabase = time.Minute * 30

func CheckDormantDatabasePhase(c *controller.Controller, elastic *tapi.Elastic, phase tapi.DormantDatabasePhase) (bool, error) {
	doneChecking := false
	then := time.Now()
	now := time.Now()

	for now.Sub(then) < durationCheckDormantDatabase {
		deletedDb, err := c.ExtClient.DormantDatabases(elastic.Namespace).Get(elastic.Name)
		if err != nil {
			if k8serr.IsNotFound(err) {
				time.Sleep(time.Second * 10)
				now = time.Now()
				continue
			} else {
				return false, err
			}
		}

		log.Debugf("DormantDatabase Phase: %v", deletedDb.Status.Phase)

		if deletedDb.Status.Phase == phase {
			doneChecking = true
			break
		}

		time.Sleep(time.Minute)
		now = time.Now()

	}

	if !doneChecking {
		return false, nil
	}

	return true, nil
}

func WipeOutDormantDatabase(c *controller.Controller, elastic *tapi.Elastic) error {
	deletedDb, err := c.ExtClient.DormantDatabases(elastic.Namespace).Get(elastic.Name)
	if err != nil {
		return err
	}

	deletedDb.Spec.WipeOut = true

	_, err = c.ExtClient.DormantDatabases(deletedDb.Namespace).Update(deletedDb)
	return err
}

func DeleteDormantDatabase(c *controller.Controller, elastic *tapi.Elastic) error {
	return c.ExtClient.DormantDatabases(elastic.Namespace).Delete(elastic.Name)
}
