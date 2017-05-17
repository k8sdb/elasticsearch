package controller

import (
	"fmt"

	tapi "github.com/k8sdb/apimachinery/api"
	amc "github.com/k8sdb/apimachinery/pkg/controller"
)

func (c *Controller) validateElastic(elastic *tapi.Elastic) error {
	if elastic.Spec.Version == "" {
		return fmt.Errorf(`Object 'Version' is missing in '%v'`, elastic.Spec)
	}

	if err := amc.CheckDockerImageVersion(imageElasticsearch, elastic.Spec.Version); err != nil {
		return fmt.Errorf(`Image %v:%v not found`, imageElasticsearch, elastic.Spec.Version)
	}

	if err := amc.CheckDockerImageVersion(imageOperatorElasticsearch, c.operatorTag); err != nil {
		return fmt.Errorf(`Image %v:%v not found`, imageOperatorElasticsearch, c.operatorTag)
	}

	storage := elastic.Spec.Storage
	if storage != nil {
		var err error
		if storage, err = c.ValidateStorageSpec(storage); err != nil {
			return err
		}
	}

	backupScheduleSpec := elastic.Spec.BackupSchedule
	if elastic.Spec.BackupSchedule != nil {
		if err := c.ValidateBackupSchedule(backupScheduleSpec); err != nil {
			return err
		}

		if err := c.CheckBucketAccess(backupScheduleSpec.SnapshotStorageSpec, elastic.Namespace); err != nil {
			return err
		}
	}
	return nil
}
