package utils

import (
	"fmt"
	"time"

	"sigs.k8s.io/controller-runtime/pkg/log"
)

const (
	PERF_ACTIVE           = true
	PERF_PREFIX           = "[PERF]"
	EVENT_GROUP_RECONCILE = "reconcile"
	EVENT_GROUP_S3        = "s3"
	EVENT_OBJECT_WORKLOAD = "workload"
	EVENT_OBJECT_TARGET   = "target"
	EVENT_OBJECT_OVERLAY  = "overlay"
	EVENT_TYPE_START      = "start"
	EVENT_TYPE_STOP       = "stop"
)

type PerfMeasurement struct {
	startTime   time.Time
	stopTime    time.Time
	elapsedTime time.Duration
	eventGroup  string
	itemType    string
	itemName    string
}

func NewMeasurement(eventGroup string, itemType string, itemName string) *PerfMeasurement {
	pm := new(PerfMeasurement)
	if PERF_ACTIVE {
		pm.eventGroup = eventGroup
		pm.itemType = itemType
		pm.itemName = itemName
		pm.startTime = time.Now()
		pm.printEvent(EVENT_TYPE_START)
	}
	return pm
}

func (pm PerfMeasurement) StopMeasurement() {
	if PERF_ACTIVE {
		pm.stopTime = time.Now()
		pm.elapsedTime = pm.stopTime.Sub(pm.startTime)
		pm.printEvent(EVENT_TYPE_STOP)
	}
}

func (pm PerfMeasurement) printEvent(eventType string) {
	log.Log.Info(fmt.Sprintf("%s,%d,%s,%s,%s,%s,%s,%s,%s,%d,%d",
		PERF_PREFIX,
		time.Now().UnixNano(),
		time.Now().Format(time.RFC3339Nano),
		pm.eventGroup,
		eventType,
		pm.itemType,
		pm.itemName,
		pm.startTime,
		pm.stopTime,
		int64(pm.elapsedTime/time.Nanosecond),
		int64(pm.elapsedTime/time.Millisecond),
	))
}
