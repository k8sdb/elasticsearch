package controller

import (
	"fmt"
	"time"

	"github.com/appscode/log"
	tapi "github.com/k8sdb/apimachinery/api"
	tcs "github.com/k8sdb/apimachinery/client/clientset"
	"github.com/k8sdb/apimachinery/pkg/eventer"
	cmap "github.com/orcaman/concurrent-map"
	"gopkg.in/robfig/cron.v2"
	kapi "k8s.io/kubernetes/pkg/api"
	clientset "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
)

type CronControllerInterface interface {
	ScheduleBackup(runtime.Object, kapi.ObjectMeta, *tapi.BackupScheduleSpec) error
	StopScheduleBackup(kapi.ObjectMeta)
}

type cronController struct {
	// ThirdPartyExtension client
	extClient tcs.ExtensionInterface
	// For Internal Cron Job
	cron *cron.Cron
	// Store Cron Job EntryID for further use
	cronEntryIDs cmap.ConcurrentMap
	// Event Recorder
	eventRecorder eventer.EventRecorderInterface
}

func NewCronController(
	// Kubernetes client
	client clientset.Interface,
	// ThirdPartyExtension client
	extClient tcs.ExtensionInterface,
) CronControllerInterface {
	return &cronController{
		extClient:     extClient,
		cron:          cron.New(),
		cronEntryIDs:  cmap.New(),
		eventRecorder: eventer.NewEventRecorder(client, "Cron Controller"),
	}
}

func (c *cronController) ScheduleBackup(
	// Runtime Object to push event
	runtimeObj runtime.Object,
	// ObjectMeta of Database TPR object
	om kapi.ObjectMeta,
	// BackupScheduleSpec
	spec *tapi.BackupScheduleSpec,
) error {
	// cronEntry name
	cronEntryName := fmt.Sprintf("%v@%v", om.Name, om.Namespace)

	// Remove previous cron job if exist
	if id, exists := c.cronEntryIDs.Pop(cronEntryName); exists {
		c.cron.Remove(id.(cron.EntryID))
	}

	invoker := &snapshotInvoker{
		extClient:     c.extClient,
		runtimeObject: runtimeObj,
		om:            om,
		spec:          spec,
		eventRecorder: c.eventRecorder,
	}

	// Set cron job
	entryID, err := c.cron.AddFunc(spec.CronExpression, invoker.createDatabaseSnapshot)
	if err != nil {
		return err
	}

	// Add job entryID
	c.cronEntryIDs.Set(cronEntryName, entryID)
	return nil
}

func (c *cronController) StopScheduleBackup(om kapi.ObjectMeta) {
	// cronEntry name
	cronEntryName := fmt.Sprintf("%v@%v", om.Name, om.Namespace)

	if id, exists := c.cronEntryIDs.Pop(cronEntryName); exists {
		c.cron.Remove(id.(cron.EntryID))
	}
}

type snapshotInvoker struct {
	extClient     tcs.ExtensionInterface
	runtimeObject runtime.Object
	om            kapi.ObjectMeta
	spec          *tapi.BackupScheduleSpec
	eventRecorder eventer.EventRecorderInterface
}

const (
	LabelDatabaseType   = "k8sdb.com/type"
	LabelDatabaseName   = "k8sdb.com/name"
	LabelSnapshotStatus = "snapshot.k8sdb.com/status"
)

func (s *snapshotInvoker) createDatabaseSnapshot() {
	typeLabel := s.om.Labels[LabelDatabaseType]
	nameLabel := s.om.Labels[LabelDatabaseName]

	labelMap := map[string]string{
		LabelDatabaseType:   typeLabel,
		LabelDatabaseName:   nameLabel,
		LabelSnapshotStatus: string(tapi.StatusSnapshotRunning),
	}

	snapshotList, err := s.extClient.DatabaseSnapshots(s.om.Namespace).List(kapi.ListOptions{
		LabelSelector: labels.SelectorFromSet(labels.Set(labelMap)),
	})
	if err != nil {
		message := fmt.Sprintf(`Failed to list DatabaseSnapshots. Reason: %v`, err)
		s.eventRecorder.PushEvent(
			kapi.EventTypeWarning, eventer.EventReasonFailedToList, message,
			s.runtimeObject,
		)
		log.Errorln(err)
		return
	}

	if len(snapshotList.Items) > 0 {
		s.eventRecorder.PushEvent(
			kapi.EventTypeNormal, eventer.EventReasonIgnoredSnapshot,
			"Skipping scheduled Backup. One is still active.",
			s.runtimeObject,
		)
		log.Debugln("Skipping scheduled Backup. One is still active.")
		return
	}

	// Set label. Elastic controller will detect this using label selector
	labelMap = map[string]string{
		LabelDatabaseType: typeLabel,
		LabelDatabaseName: nameLabel,
	}

	now := time.Now().UTC()
	snapshotName := fmt.Sprintf("%v-%v", s.om.Name, now.Format("20060102-150405"))

	snapshot := &tapi.DatabaseSnapshot{
		ObjectMeta: kapi.ObjectMeta{
			Name:      snapshotName,
			Namespace: s.om.Namespace,
			Labels:    labelMap,
		},
		Spec: tapi.DatabaseSnapshotSpec{
			DatabaseName: s.om.Name,
			SnapshotSpec: s.spec.SnapshotSpec,
		},
	}

	if _, err := s.extClient.DatabaseSnapshots(snapshot.Namespace).Create(snapshot); err != nil {
		message := fmt.Sprintf(`Failed to create DatabaseSnapshot. Reason: %v`, err)
		s.eventRecorder.PushEvent(
			kapi.EventTypeWarning, eventer.EventReasonFailedToCreate, message,
			s.runtimeObject,
		)
		log.Errorln(err)
	}
}
