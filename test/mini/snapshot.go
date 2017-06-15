package mini

import (
	"errors"
	"fmt"
	"time"

	"github.com/appscode/go/crypto/rand"
	"github.com/appscode/log"
	"github.com/ghodss/yaml"
	"github.com/graymeta/stow"
	tapi "github.com/k8sdb/apimachinery/api"
	amc "github.com/k8sdb/apimachinery/pkg/controller"
	"github.com/k8sdb/elasticsearch/pkg/controller"
	kerr "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	apiv1 "k8s.io/client-go/pkg/api/v1"
)

const durationCheckSnapshot = time.Minute * 30

func CreateSnapshot(c *controller.Controller, namespace string, snapshotSpec tapi.SnapshotSpec) (*tapi.Snapshot, error) {
	snapshot := &tapi.Snapshot{
		ObjectMeta: apiv1.ObjectMeta{
			Name:      rand.WithUniqSuffix("e2e-db-snapshot"),
			Namespace: namespace,
			Labels: map[string]string{
				amc.LabelDatabaseKind: tapi.ResourceKindElastic,
			},
		},
		Spec: snapshotSpec,
	}

	return c.ExtClient.Snapshots(namespace).Create(snapshot)
}

func CheckSnapshot(c *controller.Controller, snapshot *tapi.Snapshot) (bool, error) {
	doneChecking := false
	then := time.Now()
	now := time.Now()

	for now.Sub(then) < durationCheckSnapshot {
		snapshot, err := c.ExtClient.Snapshots(snapshot.Namespace).Get(snapshot.Name)
		if err != nil {
			if kerr.IsNotFound(err) {
				time.Sleep(time.Second * 10)
				now = time.Now()
				continue
			} else {
				return false, err
			}
		}

		log.Debugf("Snapshot Phase: %v", snapshot.Status.Phase)

		if snapshot.Status.Phase == tapi.SnapshotPhaseSuccessed {
			doneChecking = true
			break
		}

		time.Sleep(time.Second * 10)
		now = time.Now()

	}

	if !doneChecking {
		return false, nil
	}

	return true, nil
}

const (
	keyProvider = "provider"
	keyConfig   = "config"
)

func CheckSnapshotData(c *controller.Controller, snapshot *tapi.Snapshot) (int, error) {
	secret, err := c.Client.Core().Secrets(snapshot.Namespace).Get(snapshot.Spec.StorageSecret.SecretName)
	if err != nil {
		return 0, err
	}

	provider := secret.Data[keyProvider]
	if provider == nil {
		return 0, errors.New("Missing provider key")
	}
	configData := secret.Data[keyConfig]
	if configData == nil {
		return 0, errors.New("Missing config key")
	}

	var config stow.ConfigMap
	if err := yaml.Unmarshal(configData, &config); err != nil {
		return 0, err
	}

	loc, err := stow.Dial(string(provider), config)
	if err != nil {
		return 0, err
	}

	container, err := loc.Container(snapshot.Spec.BucketName)
	if err != nil {
		return 0, err
	}

	folderName := fmt.Sprintf("%v/%v/%v", amc.DatabaseNamePrefix, snapshot.Namespace, snapshot.Spec.DatabaseName)
	prefix := fmt.Sprintf("%v/%v", folderName, snapshot.Name)
	cursor := stow.CursorStart
	totalItem := 0
	for {
		items, next, err := container.Items(prefix, cursor, 50)
		if err != nil {
			return 0, err
		}

		totalItem = totalItem + len(items)

		cursor = next
		if stow.IsCursorEnd(cursor) {
			break
		}
	}

	return totalItem, nil
}

func CheckSnapshotScheduler(c *controller.Controller, elastic *tapi.Elastic) error {
	labelMap := map[string]string{
		amc.LabelDatabaseKind: tapi.ResourceKindElastic,
		amc.LabelDatabaseName: elastic.Name,
	}

	then := time.Now()
	now := time.Now()

	for now.Sub(then) < durationCheckSnapshot {

		snapshotList, err := c.ExtClient.Snapshots(elastic.Namespace).List(apiv1.ListOptions{
			LabelSelector: labels.SelectorFromSet(labels.Set(labelMap)),
		})

		if err != nil {
			return err
		}

		if len(snapshotList.Items) >= 2 {
			return nil
		}

		time.Sleep(time.Second * 10)
		now = time.Now()
	}

	return errors.New("Scheduler is not working")
}
