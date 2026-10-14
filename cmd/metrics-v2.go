// Copyright (c) 2015-2024 MinIO, Inc.
//
// This file is part of MinIO Object Storage stack
//
// This program is free software: you can redistribute it and/or modify
// it under the terms of the GNU Affero General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// This program is distributed in the hope that it will be useful
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Affero General Public License for more details.
//
// You should have received a copy of the GNU Affero General Public License
// along with this program.  If not, see <http://www.gnu.org/licenses/>.

package cmd

import (
	"context"
	"fmt"
	"maps"
	"math"
	"runtime"
	"strings"
	"sync/atomic"
	"time"

	"github.com/minio/kms-go/kes"
	"github.com/minio/minio/internal/bucket/lifecycle"
	"github.com/minio/minio/internal/cachevalue"
	xioutil "github.com/minio/minio/internal/ioutil"
	"github.com/minio/minio/internal/logger"
	"github.com/minio/minio/internal/rest"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/prometheus/procfs"
)

//go:generate msgp -file=$GOFILE -unexported -io=false

var (
	peerMetricsGroups       []*MetricsGroupV2
	bucketPeerMetricsGroups []*MetricsGroupV2
)

// v2MetricsMaxBuckets enforces a bucket count limit on metrics for v2 calls.
// If people hit this limit, they should move to v3, as certain calls explode with high bucket count.
const v2MetricsMaxBuckets = 100

func init() {
	peerMetricsGroups = []*MetricsGroupV2{
		getGoMetrics(),
		getHTTPMetrics(MetricsGroupOpts{}),
		getNotificationMetrics(MetricsGroupOpts{}),
		getMinioProcMetrics(),
		getMinioVersionMetrics(),
		getNetworkMetrics(),
		getS3TTFBMetric(),
		getILMNodeMetrics(),
		getScannerNodeMetrics(),
		getIAMNodeMetrics(MetricsGroupOpts{dependGlobalAuthNPlugin: true, dependGlobalIAMSys: true}),
		getKMSNodeMetrics(MetricsGroupOpts{dependGlobalObjectAPI: true, dependGlobalKMS: true}),
		getMinioHealingMetrics(MetricsGroupOpts{dependGlobalBackgroundHealState: true}),
		getWebhookMetrics(),
		getTierMetrics(),
	}

	bucketPeerMetricsGroups = []*MetricsGroupV2{
		getHTTPMetrics(MetricsGroupOpts{bucketOnly: true}),
		getBucketTTFBMetric(),
	}
}

// MetricNamespace is top level grouping of metrics to create the metric name.
type MetricNamespace string

// MetricSubsystem is the sub grouping for metrics within a namespace.
type MetricSubsystem string

const (
	bucketMetricNamespace    MetricNamespace = "minio_bucket"
	clusterMetricNamespace   MetricNamespace = "minio_cluster"
	healMetricNamespace      MetricNamespace = "minio_heal"
	interNodeMetricNamespace MetricNamespace = "minio_inter_node"
	nodeMetricNamespace      MetricNamespace = "minio_node"
	minioMetricNamespace     MetricNamespace = "minio"
	s3MetricNamespace        MetricNamespace = "minio_s3"
)

const (
	cacheSubsystem            MetricSubsystem = "cache"
	capacityRawSubsystem      MetricSubsystem = "capacity_raw"
	capacityUsableSubsystem   MetricSubsystem = "capacity_usable"
	driveSubsystem            MetricSubsystem = "drive"
	interfaceSubsystem        MetricSubsystem = "if"
	memSubsystem              MetricSubsystem = "mem"
	cpuSubsystem              MetricSubsystem = "cpu_avg"
	storageClassSubsystem     MetricSubsystem = "storage_class"
	fileDescriptorSubsystem   MetricSubsystem = "file_descriptor"
	goRoutines                MetricSubsystem = "go_routine"
	ioSubsystem               MetricSubsystem = "io"
	nodesSubsystem            MetricSubsystem = "nodes"
	objectsSubsystem          MetricSubsystem = "objects"
	bucketsSubsystem          MetricSubsystem = "bucket"
	processSubsystem          MetricSubsystem = "process"
	replicationSubsystem      MetricSubsystem = "replication"
	requestsSubsystem         MetricSubsystem = "requests"
	requestsRejectedSubsystem MetricSubsystem = "requests_rejected"
	timeSubsystem             MetricSubsystem = "time"
	ttfbSubsystem             MetricSubsystem = "requests_ttfb"
	trafficSubsystem          MetricSubsystem = "traffic"
	softwareSubsystem         MetricSubsystem = "software"
	sysCallSubsystem          MetricSubsystem = "syscall"
	usageSubsystem            MetricSubsystem = "usage"
	quotaSubsystem            MetricSubsystem = "quota"
	ilmSubsystem              MetricSubsystem = "ilm"
	tierSubsystem             MetricSubsystem = "tier"
	scannerSubsystem          MetricSubsystem = "scanner"
	iamSubsystem              MetricSubsystem = "iam"
	kmsSubsystem              MetricSubsystem = "kms"
	notifySubsystem           MetricSubsystem = "notify"
	auditSubsystem            MetricSubsystem = "audit"
	webhookSubsystem          MetricSubsystem = "webhook"
)

// MetricName are the individual names for the metric.
type MetricName string

const (
	lastMinFailedCount                  MetricName = "last_minute_failed_count"
	lastMinFailedBytes                  MetricName = "last_minute_failed_bytes"
	lastHourFailedCount                 MetricName = "last_hour_failed_count"
	lastHourFailedBytes                 MetricName = "last_hour_failed_bytes"
	totalFailedCount                    MetricName = "total_failed_count"
	totalFailedBytes                    MetricName = "total_failed_bytes"
	currActiveWorkers                   MetricName = "current_active_workers"
	avgActiveWorkers                    MetricName = "average_active_workers"
	maxActiveWorkers                    MetricName = "max_active_workers"
	recentBacklogCount                  MetricName = "recent_backlog_count"
	currInQueueCount                    MetricName = "last_minute_queued_count"
	currInQueueBytes                    MetricName = "last_minute_queued_bytes"
	receivedCount                       MetricName = "received_count"
	sentCount                           MetricName = "sent_count"
	avgTransferRate                     MetricName = "average_transfer_rate"
	maxTransferRate                     MetricName = "max_transfer_rate"
	credentialErrors                    MetricName = "credential_errors"
	currLinkLatency                     MetricName = "current_link_latency_ms"
	avgLinkLatency                      MetricName = "average_link_latency_ms"
	linkOnline                          MetricName = "link_online"
	linkOfflineDuration                 MetricName = "link_offline_duration_seconds"
	linkDowntimeTotalDuration           MetricName = "link_downtime_duration_seconds"
	avgInQueueCount                     MetricName = "average_queued_count"
	avgInQueueBytes                     MetricName = "average_queued_bytes"
	maxInQueueCount                     MetricName = "max_queued_count"
	maxInQueueBytes                     MetricName = "max_queued_bytes"
	proxiedGetRequestsTotal             MetricName = "proxied_get_requests_total"
	proxiedHeadRequestsTotal            MetricName = "proxied_head_requests_total"
	proxiedPutTaggingRequestsTotal      MetricName = "proxied_put_tagging_requests_total"
	proxiedGetTaggingRequestsTotal      MetricName = "proxied_get_tagging_requests_total"
	proxiedDeleteTaggingRequestsTotal   MetricName = "proxied_delete_tagging_requests_total"
	proxiedGetRequestsFailures          MetricName = "proxied_get_requests_failures"
	proxiedHeadRequestsFailures         MetricName = "proxied_head_requests_failures"
	proxiedPutTaggingRequestFailures    MetricName = "proxied_put_tagging_requests_failures"
	proxiedGetTaggingRequestFailures    MetricName = "proxied_get_tagging_requests_failures"
	proxiedDeleteTaggingRequestFailures MetricName = "proxied_delete_tagging_requests_failures"
	latencyMicroSec                     MetricName = "latency_us"
	latencyNanoSec                      MetricName = "latency_ns"
	transitionedBytes                   MetricName = "transitioned_bytes"
	transitionedObjects                 MetricName = "transitioned_objects"
	transitionedVersions                MetricName = "transitioned_versions"
	authTotal                           MetricName = "auth_total"
	canceledTotal                       MetricName = "canceled_total"
	errorsTotal                         MetricName = "errors_total"
	headerTotal                         MetricName = "header_total"
	healTotal                           MetricName = "heal_total"
	hitsTotal                           MetricName = "hits_total"
	inflightTotal                       MetricName = "inflight_total"
	invalidTotal                        MetricName = "invalid_total"
	limitTotal                          MetricName = "limit_total"
	missedTotal                         MetricName = "missed_total"
	waitingTotal                        MetricName = "waiting_total"
	incomingTotal                       MetricName = "incoming_total"
	objectTotal                         MetricName = "object_total"
	versionTotal                        MetricName = "version_total"
	deleteMarkerTotal                   MetricName = "deletemarker_total"
	offlineTotal                        MetricName = "offline_total"
	onlineTotal                         MetricName = "online_total"
	openTotal                           MetricName = "open_total"
	readTotal                           MetricName = "read_total"
	timestampTotal                      MetricName = "timestamp_total"
	writeTotal                          MetricName = "write_total"
	total                               MetricName = "total"
	freeInodes                          MetricName = "free_inodes"

	freeBytes       MetricName = "free_bytes"
	readBytes       MetricName = "read_bytes"
	rcharBytes      MetricName = "rchar_bytes"
	receivedBytes   MetricName = "received_bytes"
	latencyMilliSec MetricName = "latency_ms"
	sentBytes       MetricName = "sent_bytes"
	totalBytes      MetricName = "total_bytes"
	usedBytes       MetricName = "used_bytes"
	writeBytes      MetricName = "write_bytes"
	wcharBytes      MetricName = "wchar_bytes"

	commitInfo  MetricName = "commit_info"
	usageInfo   MetricName = "usage_info"
	versionInfo MetricName = "version_info"

	currTransferRate    MetricName = "current_transfer_rate"
	maxLinkLatency      MetricName = "max_link_latency_ms"
	sizeDistribution               = "size_distribution"
	versionDistribution            = "version_distribution"
	ttfbDistribution               = "seconds_distribution"
	ttlbDistribution               = "ttlb_seconds_distribution"

	lastActivityTime = "last_activity_nano_seconds"
	startTime        = "starttime_seconds"
	upTime           = "uptime_seconds"
	memory           = "resident_memory_bytes"
	vmemory          = "virtual_memory_bytes"
	cpu              = "cpu_total_seconds"

	expiryMissedTasks            MetricName = "expiry_missed_tasks"
	expiryMissedFreeVersions     MetricName = "expiry_missed_freeversions"
	expiryMissedTierJournalTasks MetricName = "expiry_missed_tierjournal_tasks"
	expiryNumWorkers             MetricName = "expiry_num_workers"
	transitionMissedTasks        MetricName = "transition_missed_immediate_tasks"

	tierRequestsSuccess MetricName = "requests_success"
	tierRequestsFailure MetricName = "requests_failure"

	kmsOnline          = "online"
	kmsRequestsSuccess = "request_success"
	kmsRequestsError   = "request_error"
	kmsRequestsFail    = "request_failure"
	kmsUptime          = "uptime"

	webhookOnline = "online"
)

const (
	serverName = "server"
)

// MetricTypeV2 for the types of metrics supported
type MetricTypeV2 string

const (
	gaugeMetric     = "gaugeMetric"
	counterMetric   = "counterMetric"
	histogramMetric = "histogramMetric"
)

// MetricDescription describes the metric
type MetricDescription struct {
	Namespace MetricNamespace `json:"MetricNamespace"`
	Subsystem MetricSubsystem `json:"Subsystem"`
	Name      MetricName      `json:"MetricName"`
	Help      string          `json:"Help"`
	Type      MetricTypeV2    `json:"Type"`
}

// MetricV2 captures the details for a metric
type MetricV2 struct {
	Description          MetricDescription `json:"Description"`
	StaticLabels         map[string]string `json:"StaticLabels"`
	Value                float64           `json:"Value"`
	VariableLabels       map[string]string `json:"VariableLabels"`
	HistogramBucketLabel string            `json:"HistogramBucketLabel"`
	Histogram            map[string]uint64 `json:"Histogram"`
}

// MetricsGroupV2 are a group of metrics that are initialized together.
type MetricsGroupV2 struct {
	metricsCache     *cachevalue.Cache[[]MetricV2] `msg:"-"`
	cacheInterval    time.Duration
	metricsGroupOpts MetricsGroupOpts
}

// MetricsGroupOpts are a group of metrics opts to be used to initialize the metrics group.
type MetricsGroupOpts struct {
	dependGlobalObjectAPI           bool
	dependGlobalAuthNPlugin         bool
	dependGlobalSiteReplicationSys  bool
	dependGlobalNotificationSys     bool
	dependGlobalKMS                 bool
	bucketOnly                      bool
	dependGlobalLambdaTargetList    bool
	dependGlobalIAMSys              bool
	dependGlobalLockServer          bool
	dependGlobalIsDistErasure       bool
	dependGlobalBackgroundHealState bool
	dependBucketTargetSys           bool
}

// RegisterRead register the metrics populator function to be used
// to populate new values upon cache invalidation.
func (g *MetricsGroupV2) RegisterRead(read func(context.Context) []MetricV2) {
	g.metricsCache = cachevalue.NewFromFunc(g.cacheInterval,
		cachevalue.Opts{ReturnLastGood: true},
		func(ctx context.Context) ([]MetricV2, error) {
			if g.metricsGroupOpts.dependGlobalObjectAPI {
				objLayer := newObjectLayerFn()
				// Service not initialized yet
				if objLayer == nil {
					return []MetricV2{}, nil
				}
			}
			if g.metricsGroupOpts.dependGlobalAuthNPlugin {
				if globalAuthNPlugin == nil {
					return []MetricV2{}, nil
				}
			}
			if g.metricsGroupOpts.dependGlobalSiteReplicationSys {
				if !globalSiteReplicationSys.isEnabled() {
					return []MetricV2{}, nil
				}
			}
			if g.metricsGroupOpts.dependGlobalNotificationSys {
				if globalNotificationSys == nil {
					return []MetricV2{}, nil
				}
			}
			if g.metricsGroupOpts.dependGlobalKMS {
				if GlobalKMS == nil {
					return []MetricV2{}, nil
				}
			}
			if g.metricsGroupOpts.dependGlobalIAMSys {
				if globalIAMSys == nil {
					return []MetricV2{}, nil
				}
			}
			if g.metricsGroupOpts.dependGlobalLockServer {
				if globalLockServer == nil {
					return []MetricV2{}, nil
				}
			}
			if g.metricsGroupOpts.dependGlobalIsDistErasure {
				if !globalIsDistErasure {
					return []MetricV2{}, nil
				}
			}
			if g.metricsGroupOpts.dependGlobalBackgroundHealState {
				if globalBackgroundHealState == nil {
					return []MetricV2{}, nil
				}
			}
			if g.metricsGroupOpts.dependBucketTargetSys {
				if globalBucketTargetSys == nil {
					return []MetricV2{}, nil
				}
			}
			return read(GlobalContext), nil
		},
	)
}

func (m *MetricV2) clone() MetricV2 {
	metric := MetricV2{
		Description:          m.Description,
		Value:                m.Value,
		HistogramBucketLabel: m.HistogramBucketLabel,
		StaticLabels:         make(map[string]string, len(m.StaticLabels)),
		VariableLabels:       make(map[string]string, len(m.VariableLabels)),
		Histogram:            make(map[string]uint64, len(m.Histogram)),
	}
	maps.Copy(metric.StaticLabels, m.StaticLabels)
	maps.Copy(metric.VariableLabels, m.VariableLabels)
	maps.Copy(metric.Histogram, m.Histogram)
	return metric
}

// Get - returns cached value always upton the configured TTL,
// once the TTL expires "read()" registered function is called
// to return the new values and updated.
func (g *MetricsGroupV2) Get() (metrics []MetricV2) {
	m, _ := g.metricsCache.Get()
	if len(m) == 0 {
		return []MetricV2{}
	}

	metrics = make([]MetricV2, 0, len(m))
	for i := range m {
		metrics = append(metrics, m[i].clone())
	}
	return metrics
}

func getBucketTrafficReceivedBytes() MetricDescription {
	return MetricDescription{
		Namespace: bucketMetricNamespace,
		Subsystem: trafficSubsystem,
		Name:      receivedBytes,
		Help:      "Total number of S3 bytes received for this bucket",
		Type:      gaugeMetric,
	}
}

func getBucketTrafficSentBytes() MetricDescription {
	return MetricDescription{
		Namespace: bucketMetricNamespace,
		Subsystem: trafficSubsystem,
		Name:      sentBytes,
		Help:      "Total number of S3 bytes sent for this bucket",
		Type:      gaugeMetric,
	}
}

func getInternodeFailedRequests() MetricDescription {
	return MetricDescription{
		Namespace: interNodeMetricNamespace,
		Subsystem: trafficSubsystem,
		Name:      errorsTotal,
		Help:      "Total number of failed internode calls",
		Type:      counterMetric,
	}
}

func getInternodeTCPDialTimeout() MetricDescription {
	return MetricDescription{
		Namespace: interNodeMetricNamespace,
		Subsystem: trafficSubsystem,
		Name:      "dial_errors",
		Help:      "Total number of internode TCP dial timeouts and errors",
		Type:      counterMetric,
	}
}

func getInternodeTCPAvgDuration() MetricDescription {
	return MetricDescription{
		Namespace: interNodeMetricNamespace,
		Subsystem: trafficSubsystem,
		Name:      "dial_avg_time",
		Help:      "Average time of internodes TCP dial calls",
		Type:      gaugeMetric,
	}
}

func getInterNodeSentBytesMD() MetricDescription {
	return MetricDescription{
		Namespace: interNodeMetricNamespace,
		Subsystem: trafficSubsystem,
		Name:      sentBytes,
		Help:      "Total number of bytes sent to the other peer nodes",
		Type:      counterMetric,
	}
}

func getInterNodeReceivedBytesMD() MetricDescription {
	return MetricDescription{
		Namespace: interNodeMetricNamespace,
		Subsystem: trafficSubsystem,
		Name:      receivedBytes,
		Help:      "Total number of bytes received from other peer nodes",
		Type:      counterMetric,
	}
}

func getS3SentBytesMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: trafficSubsystem,
		Name:      sentBytes,
		Help:      "Total number of s3 bytes sent",
		Type:      counterMetric,
	}
}

func getS3ReceivedBytesMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: trafficSubsystem,
		Name:      receivedBytes,
		Help:      "Total number of s3 bytes received",
		Type:      counterMetric,
	}
}

func getS3RequestsInFlightMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: requestsSubsystem,
		Name:      inflightTotal,
		Help:      "Total number of S3 requests currently in flight",
		Type:      gaugeMetric,
	}
}

func getS3RequestsInQueueMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: requestsSubsystem,
		Name:      waitingTotal,
		Help:      "Total number of S3 requests in the waiting queue",
		Type:      gaugeMetric,
	}
}

func getIncomingS3RequestsMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: requestsSubsystem,
		Name:      incomingTotal,
		Help:      "Total number of incoming S3 requests",
		Type:      gaugeMetric,
	}
}

func getS3RequestsTotalMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: requestsSubsystem,
		Name:      total,
		Help:      "Total number of S3 requests",
		Type:      counterMetric,
	}
}

func getS3RequestsErrorsMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: requestsSubsystem,
		Name:      errorsTotal,
		Help:      "Total number of S3 requests with (4xx and 5xx) errors",
		Type:      counterMetric,
	}
}

func getS3Requests4xxErrorsMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: requestsSubsystem,
		Name:      "4xx_" + errorsTotal,
		Help:      "Total number of S3 requests with (4xx) errors",
		Type:      counterMetric,
	}
}

func getS3Requests5xxErrorsMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: requestsSubsystem,
		Name:      "5xx_" + errorsTotal,
		Help:      "Total number of S3 requests with (5xx) errors",
		Type:      counterMetric,
	}
}

func getS3RequestsCanceledMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: requestsSubsystem,
		Name:      canceledTotal,
		Help:      "Total number of S3 requests that were canceled by the client",
		Type:      counterMetric,
	}
}

func getS3RejectedAuthRequestsTotalMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: requestsRejectedSubsystem,
		Name:      authTotal,
		Help:      "Total number of S3 requests rejected for auth failure",
		Type:      counterMetric,
	}
}

func getS3RejectedHeaderRequestsTotalMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: requestsRejectedSubsystem,
		Name:      headerTotal,
		Help:      "Total number of S3 requests rejected for invalid header",
		Type:      counterMetric,
	}
}

func getS3RejectedTimestampRequestsTotalMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: requestsRejectedSubsystem,
		Name:      timestampTotal,
		Help:      "Total number of S3 requests rejected for invalid timestamp",
		Type:      counterMetric,
	}
}

func getS3RejectedInvalidRequestsTotalMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: requestsRejectedSubsystem,
		Name:      invalidTotal,
		Help:      "Total number of invalid S3 requests",
		Type:      counterMetric,
	}
}

func getHealObjectsTotalMD() MetricDescription {
	return MetricDescription{
		Namespace: healMetricNamespace,
		Subsystem: objectsSubsystem,
		Name:      total,
		Help:      "Objects scanned since server uptime",
		Type:      counterMetric,
	}
}

func getHealObjectsHealTotalMD() MetricDescription {
	return MetricDescription{
		Namespace: healMetricNamespace,
		Subsystem: objectsSubsystem,
		Name:      healTotal,
		Help:      "Objects healed since server uptime",
		Type:      counterMetric,
	}
}

func getHealObjectsFailTotalMD() MetricDescription {
	return MetricDescription{
		Namespace: healMetricNamespace,
		Subsystem: objectsSubsystem,
		Name:      errorsTotal,
		Help:      "Objects with healing failed since server uptime",
		Type:      counterMetric,
	}
}

func getHealLastActivityTimeMD() MetricDescription {
	return MetricDescription{
		Namespace: healMetricNamespace,
		Subsystem: timeSubsystem,
		Name:      lastActivityTime,
		Help:      "Time elapsed (in nano seconds) since last self healing activity",
		Type:      gaugeMetric,
	}
}

func getMinIOVersionMD() MetricDescription {
	return MetricDescription{
		Namespace: minioMetricNamespace,
		Subsystem: softwareSubsystem,
		Name:      versionInfo,
		Help:      "MinIO Release tag for the server",
		Type:      gaugeMetric,
	}
}

func getMinIOCommitMD() MetricDescription {
	return MetricDescription{
		Namespace: minioMetricNamespace,
		Subsystem: softwareSubsystem,
		Name:      commitInfo,
		Help:      "Git commit hash for the MinIO release",
		Type:      gaugeMetric,
	}
}

func getS3TTFBDistributionMD() MetricDescription {
	return MetricDescription{
		Namespace: s3MetricNamespace,
		Subsystem: ttfbSubsystem,
		Name:      ttfbDistribution,
		Help:      "Distribution of time to first byte across API calls",
		Type:      gaugeMetric,
	}
}

func getBucketTTFBDistributionMD() MetricDescription {
	return MetricDescription{
		Namespace: bucketMetricNamespace,
		Subsystem: ttfbSubsystem,
		Name:      ttfbDistribution,
		Help:      "Distribution of time to first byte across API calls per bucket",
		Type:      gaugeMetric,
	}
}

func getMinioFDOpenMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: fileDescriptorSubsystem,
		Name:      openTotal,
		Help:      "Total number of open file descriptors by the MinIO Server process",
		Type:      gaugeMetric,
	}
}

func getMinioFDLimitMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: fileDescriptorSubsystem,
		Name:      limitTotal,
		Help:      "Limit on total number of open file descriptors for the MinIO Server process",
		Type:      gaugeMetric,
	}
}

func getMinioProcessIOWriteBytesMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: ioSubsystem,
		Name:      writeBytes,
		Help:      "Total bytes written by the process to the underlying storage system, /proc/[pid]/io write_bytes",
		Type:      counterMetric,
	}
}

func getMinioProcessIOReadBytesMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: ioSubsystem,
		Name:      readBytes,
		Help:      "Total bytes read by the process from the underlying storage system, /proc/[pid]/io read_bytes",
		Type:      counterMetric,
	}
}

func getMinioProcessIOWriteCachedBytesMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: ioSubsystem,
		Name:      wcharBytes,
		Help:      "Total bytes written by the process to the underlying storage system including page cache, /proc/[pid]/io wchar",
		Type:      counterMetric,
	}
}

func getMinioProcessIOReadCachedBytesMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: ioSubsystem,
		Name:      rcharBytes,
		Help:      "Total bytes read by the process from the underlying storage system including cache, /proc/[pid]/io rchar",
		Type:      counterMetric,
	}
}

func getMinIOProcessSysCallRMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: sysCallSubsystem,
		Name:      readTotal,
		Help:      "Total read SysCalls to the kernel. /proc/[pid]/io syscr",
		Type:      counterMetric,
	}
}

func getMinIOProcessSysCallWMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: sysCallSubsystem,
		Name:      writeTotal,
		Help:      "Total write SysCalls to the kernel. /proc/[pid]/io syscw",
		Type:      counterMetric,
	}
}

func getMinIOGORoutineCountMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: goRoutines,
		Name:      total,
		Help:      "Total number of go routines running",
		Type:      gaugeMetric,
	}
}

func getMinIOProcessStartTimeMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: processSubsystem,
		Name:      startTime,
		Help:      "Start time for MinIO process per node, time in seconds since Unix epoc",
		Type:      gaugeMetric,
	}
}

func getMinIOProcessUptimeMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: processSubsystem,
		Name:      upTime,
		Help:      "Uptime for MinIO process per node in seconds",
		Type:      gaugeMetric,
	}
}

func getMinIOProcessResidentMemory() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: processSubsystem,
		Name:      memory,
		Help:      "Resident memory size in bytes",
		Type:      gaugeMetric,
	}
}

func getMinIOProcessVirtualMemory() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: processSubsystem,
		Name:      memory,
		Help:      "Virtual memory size in bytes",
		Type:      gaugeMetric,
	}
}

func getMinIOProcessCPUTime() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: processSubsystem,
		Name:      cpu,
		Help:      "Total user and system CPU time spent in seconds",
		Type:      counterMetric,
	}
}

func getMinioProcMetrics() *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval: 10 * time.Second,
	}
	mg.RegisterRead(func(ctx context.Context) (metrics []MetricV2) {
		if runtime.GOOS == globalWindowsOSName || runtime.GOOS == globalMacOSName {
			return nil
		}

		p, err := procfs.Self()
		if err != nil {
			internalLogOnceIf(ctx, err, string(nodeMetricNamespace))
			return metrics
		}

		openFDs, _ := p.FileDescriptorsLen()
		l, _ := p.Limits()
		io, _ := p.IO()
		stat, _ := p.Stat()
		startTime, _ := stat.StartTime()

		metrics = make([]MetricV2, 0, 20)

		if openFDs > 0 {
			metrics = append(metrics,
				MetricV2{
					Description: getMinioFDOpenMD(),
					Value:       float64(openFDs),
				},
			)
		}

		if l.OpenFiles > 0 {
			metrics = append(metrics,
				MetricV2{
					Description: getMinioFDLimitMD(),
					Value:       float64(l.OpenFiles),
				})
		}

		if io.SyscR > 0 {
			metrics = append(metrics,
				MetricV2{
					Description: getMinIOProcessSysCallRMD(),
					Value:       float64(io.SyscR),
				})
		}

		if io.SyscW > 0 {
			metrics = append(metrics,
				MetricV2{
					Description: getMinIOProcessSysCallWMD(),
					Value:       float64(io.SyscW),
				})
		}

		if io.ReadBytes > 0 {
			metrics = append(metrics,
				MetricV2{
					Description: getMinioProcessIOReadBytesMD(),
					Value:       float64(io.ReadBytes),
				})
		}

		if io.WriteBytes > 0 {
			metrics = append(metrics,
				MetricV2{
					Description: getMinioProcessIOWriteBytesMD(),
					Value:       float64(io.WriteBytes),
				})
		}

		if io.RChar > 0 {
			metrics = append(metrics,
				MetricV2{
					Description: getMinioProcessIOReadCachedBytesMD(),
					Value:       float64(io.RChar),
				})
		}

		if io.WChar > 0 {
			metrics = append(metrics,
				MetricV2{
					Description: getMinioProcessIOWriteCachedBytesMD(),
					Value:       float64(io.WChar),
				})
		}

		if startTime > 0 {
			metrics = append(metrics,
				MetricV2{
					Description: getMinIOProcessStartTimeMD(),
					Value:       startTime,
				})
		}

		if !globalBootTime.IsZero() {
			metrics = append(metrics,
				MetricV2{
					Description: getMinIOProcessUptimeMD(),
					Value:       time.Since(globalBootTime).Seconds(),
				})
		}

		if stat.ResidentMemory() > 0 {
			metrics = append(metrics,
				MetricV2{
					Description: getMinIOProcessResidentMemory(),
					Value:       float64(stat.ResidentMemory()),
				})
		}

		if stat.VirtualMemory() > 0 {
			metrics = append(metrics,
				MetricV2{
					Description: getMinIOProcessVirtualMemory(),
					Value:       float64(stat.VirtualMemory()),
				})
		}

		if stat.CPUTime() > 0 {
			metrics = append(metrics,
				MetricV2{
					Description: getMinIOProcessCPUTime(),
					Value:       stat.CPUTime(),
				})
		}
		return metrics
	})
	return mg
}

func getGoMetrics() *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval: 10 * time.Second,
	}
	mg.RegisterRead(func(ctx context.Context) (metrics []MetricV2) {
		metrics = append(metrics, MetricV2{
			Description: getMinIOGORoutineCountMD(),
			Value:       float64(runtime.NumGoroutine()),
		})
		return metrics
	})
	return mg
}

// getHistogramMetrics fetches histogram metrics and returns it in a []Metric
// Note: Typically used in MetricGroup.RegisterRead
//
// The toLowerAPILabels parameter is added for compatibility,
// if set, it lowercases the `api` label values.
// Metric name constants shared between v2 groups and the v3 ILM/logger
// subsystem (internal/metrics).
const (
	expiryPendingTasks     MetricName = "expiry_pending_tasks"
	transitionActiveTasks  MetricName = "transition_active_tasks"
	transitionPendingTasks MetricName = "transition_pending_tasks"
	webhookQueueLength     MetricName = "queue_length"
	webhookTotalMessages   MetricName = "total_messages"
	webhookFailedMessages  MetricName = "failed_messages"
)

func getHistogramMetrics(hist *prometheus.HistogramVec, desc MetricDescription, toLowerAPILabels, limitBuckets bool) []MetricV2 {
	ch := make(chan prometheus.Metric)
	go func() {
		defer xioutil.SafeClose(ch)
		// Collects prometheus metrics from hist and sends it over ch
		hist.Collect(ch)
	}()

	// Converts metrics received into internal []Metric type
	var metrics []MetricV2
	buckets := make(map[string][]MetricV2, v2MetricsMaxBuckets)
	for promMetric := range ch {
		dtoMetric := &dto.Metric{}
		err := promMetric.Write(dtoMetric)
		if err != nil {
			// Log error and continue to receive other metric
			// values
			bugLogIf(GlobalContext, err)
			continue
		}

		h := dtoMetric.GetHistogram()
		for _, b := range h.Bucket {
			labels := make(map[string]string)
			for _, lp := range dtoMetric.GetLabel() {
				if *lp.Name == "api" && toLowerAPILabels {
					labels[*lp.Name] = strings.ToLower(*lp.Value)
				} else {
					labels[*lp.Name] = *lp.Value
				}
			}
			labels["le"] = fmt.Sprintf("%.3f", *b.UpperBound)
			metric := MetricV2{
				Description:    desc,
				VariableLabels: labels,
				Value:          float64(b.GetCumulativeCount()),
			}
			if limitBuckets && labels["bucket"] != "" {
				buckets[labels["bucket"]] = append(buckets[labels["bucket"]], metric)
			} else {
				metrics = append(metrics, metric)
			}
		}
		// add metrics with +Inf label
		labels1 := make(map[string]string)
		for _, lp := range dtoMetric.GetLabel() {
			if *lp.Name == "api" && toLowerAPILabels {
				labels1[*lp.Name] = strings.ToLower(*lp.Value)
			} else {
				labels1[*lp.Name] = *lp.Value
			}
		}
		labels1["le"] = fmt.Sprintf("%.3f", math.Inf(+1))

		metric := MetricV2{
			Description:    desc,
			VariableLabels: labels1,
			Value:          float64(dtoMetric.Histogram.GetSampleCount()),
		}
		if limitBuckets && labels1["bucket"] != "" {
			buckets[labels1["bucket"]] = append(buckets[labels1["bucket"]], metric)
		} else {
			metrics = append(metrics, metric)
		}
	}

	// Limit bucket metrics...
	if limitBuckets {
		bucketNames := mapKeysSorted(buckets)
		bucketNames = bucketNames[:min(len(buckets), v2MetricsMaxBuckets)]
		for _, b := range bucketNames {
			metrics = append(metrics, buckets[b]...)
		}
	}
	return metrics
}

func getBucketTTFBMetric() *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval: 10 * time.Second,
	}
	mg.RegisterRead(func(ctx context.Context) []MetricV2 {
		return getHistogramMetrics(bucketHTTPRequestsDuration,
			getBucketTTFBDistributionMD(), true, true)
	})
	return mg
}

func getS3TTFBMetric() *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval: 10 * time.Second,
	}
	mg.RegisterRead(func(ctx context.Context) []MetricV2 {
		return getHistogramMetrics(httpRequestsDuration,
			getS3TTFBDistributionMD(), true, true)
	})
	return mg
}

func getTierMetrics() *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval: 10 * time.Second,
	}
	mg.RegisterRead(func(ctx context.Context) []MetricV2 {
		return globalTierMetrics.Report()
	})
	return mg
}

func getTransitionPendingTasksMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: ilmSubsystem,
		Name:      transitionPendingTasks,
		Help:      "Number of pending ILM transition tasks in the queue",
		Type:      gaugeMetric,
	}
}

func getTransitionActiveTasksMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: ilmSubsystem,
		Name:      transitionActiveTasks,
		Help:      "Number of active ILM transition tasks",
		Type:      gaugeMetric,
	}
}

func getTransitionMissedTasksMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: ilmSubsystem,
		Name:      transitionMissedTasks,
		Help:      "Number of missed immediate ILM transition tasks",
		Type:      gaugeMetric,
	}
}

func getExpiryPendingTasksMD() MetricDescription {
	return MetricDescription{
		Namespace: nodeMetricNamespace,
		Subsystem: ilmSubsystem,
		Name:      expiryPendingTasks,
		Help:      "Number of pending ILM expiry tasks in the queue",
		Type:      gaugeMetric,
	}
}

func getBucketS3RequestsInFlightMD() MetricDescription {
	return MetricDescription{
		Namespace: bucketMetricNamespace,
		Subsystem: requestsSubsystem,
		Name:      inflightTotal,
		Help:      "Total number of S3 requests currently in flight on a bucket",
		Type:      gaugeMetric,
	}
}

func getBucketS3RequestsTotalMD() MetricDescription {
	return MetricDescription{
		Namespace: bucketMetricNamespace,
		Subsystem: requestsSubsystem,
		Name:      total,
		Help:      "Total number of S3 requests on a bucket",
		Type:      counterMetric,
	}
}

func getBucketS3Requests4xxErrorsMD() MetricDescription {
	return MetricDescription{
		Namespace: bucketMetricNamespace,
		Subsystem: requestsSubsystem,
		Name:      "4xx_" + errorsTotal,
		Help:      "Total number of S3 requests with (4xx) errors on a bucket",
		Type:      counterMetric,
	}
}

func getBucketS3Requests5xxErrorsMD() MetricDescription {
	return MetricDescription{
		Namespace: bucketMetricNamespace,
		Subsystem: requestsSubsystem,
		Name:      "5xx_" + errorsTotal,
		Help:      "Total number of S3 requests with (5xx) errors on a bucket",
		Type:      counterMetric,
	}
}

func getBucketS3RequestsCanceledMD() MetricDescription {
	return MetricDescription{
		Namespace: bucketMetricNamespace,
		Subsystem: requestsSubsystem,
		Name:      canceledTotal,
		Help:      "Total number of S3 requests that were canceled from the client while processing on a bucket",
		Type:      counterMetric,
	}
}

func getILMNodeMetrics() *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval: 10 * time.Second,
	}
	mg.RegisterRead(func(_ context.Context) []MetricV2 {
		expPendingTasks := MetricV2{
			Description: getExpiryPendingTasksMD(),
		}
		expMissedTasks := MetricV2{
			Description: MetricDescription{
				Namespace: nodeMetricNamespace,
				Subsystem: ilmSubsystem,
				Name:      expiryMissedTasks,
				Help:      "Number of object version expiry missed due to busy system",
				Type:      counterMetric,
			},
		}
		expMissedFreeVersions := MetricV2{
			Description: MetricDescription{
				Namespace: nodeMetricNamespace,
				Subsystem: ilmSubsystem,
				Name:      expiryMissedFreeVersions,
				Help:      "Number of free versions expiry missed due to busy system",
				Type:      counterMetric,
			},
		}
		expMissedTierJournalTasks := MetricV2{
			Description: MetricDescription{
				Namespace: nodeMetricNamespace,
				Subsystem: ilmSubsystem,
				Name:      expiryMissedTierJournalTasks,
				Help:      "Number of tier journal entries cleanup missed due to busy system",
				Type:      counterMetric,
			},
		}
		expNumWorkers := MetricV2{
			Description: MetricDescription{
				Namespace: nodeMetricNamespace,
				Subsystem: ilmSubsystem,
				Name:      expiryNumWorkers,
				Help:      "Number of workers expiring object versions currently",
				Type:      gaugeMetric,
			},
		}
		trPendingTasks := MetricV2{
			Description: getTransitionPendingTasksMD(),
		}
		trActiveTasks := MetricV2{
			Description: getTransitionActiveTasksMD(),
		}
		trMissedTasks := MetricV2{
			Description: getTransitionMissedTasksMD(),
		}
		if globalExpiryState != nil {
			expPendingTasks.Value = float64(globalExpiryState.PendingTasks())
			expMissedTasks.Value = float64(globalExpiryState.stats.MissedTasks())
			expMissedFreeVersions.Value = float64(globalExpiryState.stats.MissedFreeVersTasks())
			expMissedTierJournalTasks.Value = float64(globalExpiryState.stats.MissedTierJournalTasks())
			expNumWorkers.Value = float64(globalExpiryState.stats.NumWorkers())
		}
		if globalTransitionState != nil {
			trPendingTasks.Value = float64(globalTransitionState.PendingTasks())
			trActiveTasks.Value = float64(globalTransitionState.ActiveTasks())
			trMissedTasks.Value = float64(globalTransitionState.MissedImmediateTasks())
		}
		return []MetricV2{
			expPendingTasks,
			expMissedTasks,
			expMissedFreeVersions,
			expMissedTierJournalTasks,
			expNumWorkers,
			trPendingTasks,
			trActiveTasks,
			trMissedTasks,
		}
	})
	return mg
}

func getScannerNodeMetrics() *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval: 10 * time.Second,
	}
	mg.RegisterRead(func(_ context.Context) []MetricV2 {
		metrics := []MetricV2{
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: scannerSubsystem,
					Name:      "objects_scanned",
					Help:      "Total number of unique objects scanned since server uptime",
					Type:      counterMetric,
				},
				Value: float64(globalScannerMetrics.lifetime(scannerMetricScanObject)),
			},
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: scannerSubsystem,
					Name:      "versions_scanned",
					Help:      "Total number of object versions scanned since server uptime",
					Type:      counterMetric,
				},
				Value: float64(globalScannerMetrics.lifetime(scannerMetricApplyVersion)),
			},
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: scannerSubsystem,
					Name:      "directories_scanned",
					Help:      "Total number of directories scanned since server uptime",
					Type:      counterMetric,
				},
				Value: float64(globalScannerMetrics.lifetime(scannerMetricScanFolder)),
			},
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: scannerSubsystem,
					Name:      "bucket_scans_started",
					Help:      "Total number of bucket scans started since server uptime",
					Type:      counterMetric,
				},
				Value: float64(globalScannerMetrics.lifetime(scannerMetricScanBucketDrive) + uint64(globalScannerMetrics.activeDrives())),
			},
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: scannerSubsystem,
					Name:      "bucket_scans_finished",
					Help:      "Total number of bucket scans finished since server uptime",
					Type:      counterMetric,
				},
				Value: float64(globalScannerMetrics.lifetime(scannerMetricScanBucketDrive)),
			},
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: ilmSubsystem,
					Name:      "versions_scanned",
					Help:      "Total number of object versions checked for ilm actions since server uptime",
					Type:      counterMetric,
				},
				Value: float64(globalScannerMetrics.lifetime(scannerMetricILM)),
			},
		}
		for i := range globalScannerMetrics.actions {
			action := lifecycle.Action(i)
			v := globalScannerMetrics.lifetimeActions(action)
			if v == 0 {
				continue
			}
			metrics = append(metrics, MetricV2{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: ilmSubsystem,
					Name:      MetricName("action_count_" + toSnake(action.String())),
					Help:      "Total action outcome of lifecycle checks since server uptime",
					Type:      counterMetric,
				},
				Value: float64(v),
			})
		}
		return metrics
	})
	return mg
}

func getIAMNodeMetrics(opts MetricsGroupOpts) *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval:    10 * time.Second,
		metricsGroupOpts: opts,
	}
	mg.RegisterRead(func(_ context.Context) (metrics []MetricV2) {
		lastSyncTime := atomic.LoadUint64(&globalIAMSys.LastRefreshTimeUnixNano)
		var sinceLastSyncMillis uint64
		if lastSyncTime != 0 {
			sinceLastSyncMillis = (uint64(time.Now().UnixNano()) - lastSyncTime) / uint64(time.Millisecond)
		}

		pluginAuthNMetrics := globalAuthNPlugin.Metrics()
		metrics = []MetricV2{
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: iamSubsystem,
					Name:      "last_sync_duration_millis",
					Help:      "Last successful IAM data sync duration in milliseconds",
					Type:      gaugeMetric,
				},
				Value: float64(atomic.LoadUint64(&globalIAMSys.LastRefreshDurationMilliseconds)),
			},
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: iamSubsystem,
					Name:      "since_last_sync_millis",
					Help:      "Time (in milliseconds) since last successful IAM data sync",
					Type:      gaugeMetric,
				},
				Value: float64(sinceLastSyncMillis),
			},
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: iamSubsystem,
					Name:      "sync_successes",
					Help:      "Number of successful IAM data syncs since server uptime",
					Type:      counterMetric,
				},
				Value: float64(atomic.LoadUint64(&globalIAMSys.TotalRefreshSuccesses)),
			},
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: iamSubsystem,
					Name:      "sync_failures",
					Help:      "Number of failed IAM data syncs since server uptime",
					Type:      counterMetric,
				},
				Value: float64(atomic.LoadUint64(&globalIAMSys.TotalRefreshFailures)),
			},
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: iamSubsystem,
					Name:      "plugin_authn_service_last_succ_seconds",
					Help:      "When plugin authentication is configured, returns time (in seconds) since the last successful request to the service",
					Type:      gaugeMetric,
				},
				Value: pluginAuthNMetrics.LastReachableSecs,
			},
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: iamSubsystem,
					Name:      "plugin_authn_service_last_fail_seconds",
					Help:      "When plugin authentication is configured, returns time (in seconds) since the last failed request to the service",
					Type:      gaugeMetric,
				},
				Value: pluginAuthNMetrics.LastUnreachableSecs,
			},
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: iamSubsystem,
					Name:      "plugin_authn_service_total_requests_minute",
					Help:      "When plugin authentication is configured, returns total requests count in the last full minute",
					Type:      gaugeMetric,
				},
				Value: float64(pluginAuthNMetrics.TotalRequests),
			},
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: iamSubsystem,
					Name:      "plugin_authn_service_failed_requests_minute",
					Help:      "When plugin authentication is configured, returns failed requests count in the last full minute",
					Type:      gaugeMetric,
				},
				Value: float64(pluginAuthNMetrics.FailedRequests),
			},
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: iamSubsystem,
					Name:      "plugin_authn_service_succ_avg_rtt_ms_minute",
					Help:      "When plugin authentication is configured, returns average round-trip-time of successful requests in the last full minute",
					Type:      gaugeMetric,
				},
				Value: pluginAuthNMetrics.AvgSuccRTTMs,
			},
			{
				Description: MetricDescription{
					Namespace: nodeMetricNamespace,
					Subsystem: iamSubsystem,
					Name:      "plugin_authn_service_succ_max_rtt_ms_minute",
					Help:      "When plugin authentication is configured, returns maximum round-trip-time of successful requests in the last full minute",
					Type:      gaugeMetric,
				},
				Value: pluginAuthNMetrics.MaxSuccRTTMs,
			},
		}

		return metrics
	})
	return mg
}

// replication metrics for each node - published to the cluster endpoint with nodename as label
// replication metrics for site replication
func getMinioVersionMetrics() *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval: 10 * time.Second,
	}
	mg.RegisterRead(func(_ context.Context) (metrics []MetricV2) {
		metrics = append(metrics, MetricV2{
			Description:    getMinIOCommitMD(),
			VariableLabels: map[string]string{"commit": CommitID},
		})
		metrics = append(metrics, MetricV2{
			Description:    getMinIOVersionMD(),
			VariableLabels: map[string]string{"version": Version},
		})
		return metrics
	})
	return mg
}

func getMinioHealingMetrics(opts MetricsGroupOpts) *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval:    10 * time.Second,
		metricsGroupOpts: opts,
	}
	mg.RegisterRead(func(_ context.Context) (metrics []MetricV2) {
		bgSeq, exists := globalBackgroundHealState.getHealSequenceByToken(bgHealingUUID)
		if !exists {
			return metrics
		}

		if bgSeq.lastHealActivity.IsZero() {
			return metrics
		}

		metrics = make([]MetricV2, 0, 5)
		metrics = append(metrics, MetricV2{
			Description: getHealLastActivityTimeMD(),
			Value:       float64(time.Since(bgSeq.lastHealActivity)),
		})
		metrics = append(metrics, getObjectsScanned(bgSeq)...)
		metrics = append(metrics, getHealedItems(bgSeq)...)
		metrics = append(metrics, getFailedItems(bgSeq)...)
		return metrics
	})
	return mg
}

func getFailedItems(seq *healSequence) (m []MetricV2) {
	items := seq.getHealFailedItemsMap()
	m = make([]MetricV2, 0, len(items))
	for k, v := range items {
		m = append(m, MetricV2{
			Description:    getHealObjectsFailTotalMD(),
			VariableLabels: map[string]string{"type": string(k)},
			Value:          float64(v),
		})
	}
	return m
}

func getHealedItems(seq *healSequence) (m []MetricV2) {
	items := seq.getHealedItemsMap()
	m = make([]MetricV2, 0, len(items))
	for k, v := range items {
		m = append(m, MetricV2{
			Description:    getHealObjectsHealTotalMD(),
			VariableLabels: map[string]string{"type": string(k)},
			Value:          float64(v),
		})
	}
	return m
}

func getObjectsScanned(seq *healSequence) (m []MetricV2) {
	items := seq.getScannedItemsMap()
	m = make([]MetricV2, 0, len(items))
	for k, v := range items {
		m = append(m, MetricV2{
			Description:    getHealObjectsTotalMD(),
			VariableLabels: map[string]string{"type": string(k)},
			Value:          float64(v),
		})
	}
	return m
}

func getNotificationMetrics(opts MetricsGroupOpts) *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval:    10 * time.Second,
		metricsGroupOpts: opts,
	}
	mg.RegisterRead(func(ctx context.Context) []MetricV2 {
		metrics := make([]MetricV2, 0, 3)

		if globalEventNotifier != nil {
			nstats := globalEventNotifier.targetList.Stats()
			metrics = append(metrics, MetricV2{
				Description: MetricDescription{
					Namespace: minioNamespace,
					Subsystem: notifySubsystem,
					Name:      "current_send_in_progress",
					Help:      "Number of concurrent async Send calls active to all targets (deprecated, please use 'minio_notify_target_current_send_in_progress' instead)",
					Type:      gaugeMetric,
				},
				Value: float64(nstats.CurrentSendCalls),
			})
			metrics = append(metrics, MetricV2{
				Description: MetricDescription{
					Namespace: minioNamespace,
					Subsystem: notifySubsystem,
					Name:      "events_skipped_total",
					Help:      "Events that were skipped to be sent to the targets due to the in-memory queue being full",
					Type:      counterMetric,
				},
				Value: float64(nstats.EventsSkipped),
			})
			metrics = append(metrics, MetricV2{
				Description: MetricDescription{
					Namespace: minioNamespace,
					Subsystem: notifySubsystem,
					Name:      "events_errors_total",
					Help:      "Events that were failed to be sent to the targets (deprecated, please use 'minio_notify_target_failed_events' instead)",
					Type:      counterMetric,
				},
				Value: float64(nstats.EventsErrorsTotal),
			})
			metrics = append(metrics, MetricV2{
				Description: MetricDescription{
					Namespace: minioNamespace,
					Subsystem: notifySubsystem,
					Name:      "events_sent_total",
					Help:      "Total number of events sent to the targets (deprecated, please use 'minio_notify_target_total_events' instead)",
					Type:      counterMetric,
				},
				Value: float64(nstats.TotalEvents),
			})
			for id, st := range nstats.TargetStats {
				metrics = append(metrics, MetricV2{
					Description: MetricDescription{
						Namespace: minioNamespace,
						Subsystem: notifySubsystem,
						Name:      "target_total_events",
						Help:      "Total number of events sent (or) queued to the target",
						Type:      counterMetric,
					},
					VariableLabels: map[string]string{"target_id": id.ID, "target_name": id.Name},
					Value:          float64(st.TotalEvents),
				})
				metrics = append(metrics, MetricV2{
					Description: MetricDescription{
						Namespace: minioNamespace,
						Subsystem: notifySubsystem,
						Name:      "target_failed_events",
						Help:      "Number of events failed to be sent (or) queued to the target",
						Type:      counterMetric,
					},
					VariableLabels: map[string]string{"target_id": id.ID, "target_name": id.Name},
					Value:          float64(st.FailedEvents),
				})
				metrics = append(metrics, MetricV2{
					Description: MetricDescription{
						Namespace: minioNamespace,
						Subsystem: notifySubsystem,
						Name:      "target_current_send_in_progress",
						Help:      "Number of concurrent async Send calls active to the target",
						Type:      gaugeMetric,
					},
					VariableLabels: map[string]string{"target_id": id.ID, "target_name": id.Name},
					Value:          float64(st.CurrentSendCalls),
				})
				metrics = append(metrics, MetricV2{
					Description: MetricDescription{
						Namespace: minioNamespace,
						Subsystem: notifySubsystem,
						Name:      "target_queue_length",
						Help:      "Number of events currently staged in the queue_dir configured for the target",
						Type:      gaugeMetric,
					},
					VariableLabels: map[string]string{"target_id": id.ID, "target_name": id.Name},
					Value:          float64(st.CurrentQueue),
				})
			}
		}
		return metrics
	})
	return mg
}

func getHTTPMetrics(opts MetricsGroupOpts) *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval:    10 * time.Second,
		metricsGroupOpts: opts,
	}
	mg.RegisterRead(func(ctx context.Context) (metrics []MetricV2) {
		if !mg.metricsGroupOpts.bucketOnly {
			httpStats := globalHTTPStats.toServerHTTPStats(true)
			metrics = make([]MetricV2, 0, 3+
				len(httpStats.CurrentS3Requests.APIStats)+
				len(httpStats.TotalS3Requests.APIStats)+
				len(httpStats.TotalS3Errors.APIStats)+
				len(httpStats.TotalS35xxErrors.APIStats)+
				len(httpStats.TotalS34xxErrors.APIStats))
			metrics = append(metrics, MetricV2{
				Description: getS3RejectedAuthRequestsTotalMD(),
				Value:       float64(httpStats.TotalS3RejectedAuth),
			})
			metrics = append(metrics, MetricV2{
				Description: getS3RejectedTimestampRequestsTotalMD(),
				Value:       float64(httpStats.TotalS3RejectedTime),
			})
			metrics = append(metrics, MetricV2{
				Description: getS3RejectedHeaderRequestsTotalMD(),
				Value:       float64(httpStats.TotalS3RejectedHeader),
			})
			metrics = append(metrics, MetricV2{
				Description: getS3RejectedInvalidRequestsTotalMD(),
				Value:       float64(httpStats.TotalS3RejectedInvalid),
			})
			metrics = append(metrics, MetricV2{
				Description: getS3RequestsInQueueMD(),
				Value:       float64(httpStats.S3RequestsInQueue),
			})
			metrics = append(metrics, MetricV2{
				Description: getIncomingS3RequestsMD(),
				Value:       float64(httpStats.S3RequestsIncoming),
			})

			for api, value := range httpStats.CurrentS3Requests.APIStats {
				metrics = append(metrics, MetricV2{
					Description:    getS3RequestsInFlightMD(),
					Value:          float64(value),
					VariableLabels: map[string]string{"api": api},
				})
			}
			for api, value := range httpStats.TotalS3Requests.APIStats {
				metrics = append(metrics, MetricV2{
					Description:    getS3RequestsTotalMD(),
					Value:          float64(value),
					VariableLabels: map[string]string{"api": api},
				})
			}
			for api, value := range httpStats.TotalS3Errors.APIStats {
				metrics = append(metrics, MetricV2{
					Description:    getS3RequestsErrorsMD(),
					Value:          float64(value),
					VariableLabels: map[string]string{"api": api},
				})
			}
			for api, value := range httpStats.TotalS35xxErrors.APIStats {
				metrics = append(metrics, MetricV2{
					Description:    getS3Requests5xxErrorsMD(),
					Value:          float64(value),
					VariableLabels: map[string]string{"api": api},
				})
			}
			for api, value := range httpStats.TotalS34xxErrors.APIStats {
				metrics = append(metrics, MetricV2{
					Description:    getS3Requests4xxErrorsMD(),
					Value:          float64(value),
					VariableLabels: map[string]string{"api": api},
				})
			}
			for api, value := range httpStats.TotalS3Canceled.APIStats {
				metrics = append(metrics, MetricV2{
					Description:    getS3RequestsCanceledMD(),
					Value:          float64(value),
					VariableLabels: map[string]string{"api": api},
				})
			}
			return metrics
		}

		// If we have too many, limit them
		bConnStats := globalBucketConnStats.getS3InOutBytes()
		buckets := mapKeysSorted(bConnStats)
		buckets = buckets[:min(v2MetricsMaxBuckets, len(buckets))]

		for _, bucket := range buckets {
			inOut := bConnStats[bucket]
			recvBytes := inOut.In
			if recvBytes > 0 {
				metrics = append(metrics, MetricV2{
					Description:    getBucketTrafficReceivedBytes(),
					Value:          float64(recvBytes),
					VariableLabels: map[string]string{"bucket": bucket},
				})
			}
			sentBytes := inOut.Out
			if sentBytes > 0 {
				metrics = append(metrics, MetricV2{
					Description:    getBucketTrafficSentBytes(),
					Value:          float64(sentBytes),
					VariableLabels: map[string]string{"bucket": bucket},
				})
			}

			httpStats := globalBucketHTTPStats.load(bucket)
			for k, v := range httpStats.currentS3Requests.Load(true) {
				metrics = append(metrics, MetricV2{
					Description:    getBucketS3RequestsInFlightMD(),
					Value:          float64(v),
					VariableLabels: map[string]string{"bucket": bucket, "api": k},
				})
			}

			for k, v := range httpStats.totalS3Requests.Load(true) {
				metrics = append(metrics, MetricV2{
					Description:    getBucketS3RequestsTotalMD(),
					Value:          float64(v),
					VariableLabels: map[string]string{"bucket": bucket, "api": k},
				})
			}

			for k, v := range httpStats.totalS3Canceled.Load(true) {
				metrics = append(metrics, MetricV2{
					Description:    getBucketS3RequestsCanceledMD(),
					Value:          float64(v),
					VariableLabels: map[string]string{"bucket": bucket, "api": k},
				})
			}

			for k, v := range httpStats.totalS34xxErrors.Load(true) {
				metrics = append(metrics, MetricV2{
					Description:    getBucketS3Requests4xxErrorsMD(),
					Value:          float64(v),
					VariableLabels: map[string]string{"bucket": bucket, "api": k},
				})
			}

			for k, v := range httpStats.totalS35xxErrors.Load(true) {
				metrics = append(metrics, MetricV2{
					Description:    getBucketS3Requests5xxErrorsMD(),
					Value:          float64(v),
					VariableLabels: map[string]string{"bucket": bucket, "api": k},
				})
			}
		}

		return metrics
	})
	return mg
}

func getNetworkMetrics() *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval: 10 * time.Second,
	}
	mg.RegisterRead(func(ctx context.Context) (metrics []MetricV2) {
		metrics = make([]MetricV2, 0, 10)
		connStats := globalConnStats.toServerConnStats()
		rpcStats := rest.GetRPCStats()
		if globalIsDistErasure {
			metrics = append(metrics, MetricV2{
				Description: getInternodeFailedRequests(),
				Value:       float64(rpcStats.Errs),
			})
			metrics = append(metrics, MetricV2{
				Description: getInternodeTCPDialTimeout(),
				Value:       float64(rpcStats.DialErrs),
			})
			metrics = append(metrics, MetricV2{
				Description: getInternodeTCPAvgDuration(),
				Value:       float64(rpcStats.DialAvgDuration),
			})
			metrics = append(metrics, MetricV2{
				Description: getInterNodeSentBytesMD(),
				Value:       float64(connStats.internodeOutputBytes),
			})
			metrics = append(metrics, MetricV2{
				Description: getInterNodeReceivedBytesMD(),
				Value:       float64(connStats.internodeInputBytes),
			})
		}
		metrics = append(metrics, MetricV2{
			Description: getS3SentBytesMD(),
			Value:       float64(connStats.s3OutputBytes),
		})
		metrics = append(metrics, MetricV2{
			Description: getS3ReceivedBytesMD(),
			Value:       float64(connStats.s3InputBytes),
		})
		return metrics
	})
	return mg
}

func getKMSNodeMetrics(opts MetricsGroupOpts) *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval:    10 * time.Second,
		metricsGroupOpts: opts,
	}

	mg.RegisterRead(func(ctx context.Context) (metrics []MetricV2) {
		const (
			Online  = 1
			Offline = 0
		)
		desc := MetricDescription{
			Namespace: clusterMetricNamespace,
			Subsystem: kmsSubsystem,
			Name:      kmsOnline,
			Help:      "Reports whether the KMS is online (1) or offline (0)",
			Type:      gaugeMetric,
		}
		_, err := GlobalKMS.Metrics(ctx)
		if _, ok := kes.IsConnError(err); ok {
			return []MetricV2{{
				Description: desc,
				Value:       float64(Offline),
			}}
		}
		return []MetricV2{{
			Description: desc,
			Value:       float64(Online),
		}}
	})
	return mg
}

func getWebhookMetrics() *MetricsGroupV2 {
	mg := &MetricsGroupV2{
		cacheInterval: 10 * time.Second,
	}
	mg.RegisterRead(func(ctx context.Context) []MetricV2 {
		tgts := append(logger.SystemTargets(), logger.AuditTargets()...)
		metrics := make([]MetricV2, 0, len(tgts)*4)
		for _, t := range tgts {
			isOnline := 0
			if t.IsOnline(ctx) {
				isOnline = 1
			}
			labels := map[string]string{
				"name":     t.String(),
				"endpoint": t.Endpoint(),
			}
			metrics = append(metrics, MetricV2{
				Description: MetricDescription{
					Namespace: clusterMetricNamespace,
					Subsystem: webhookSubsystem,
					Name:      webhookOnline,
					Help:      "Is the webhook online?",
					Type:      gaugeMetric,
				},
				VariableLabels: labels,
				Value:          float64(isOnline),
			})
			metrics = append(metrics, MetricV2{
				Description: MetricDescription{
					Namespace: clusterMetricNamespace,
					Subsystem: webhookSubsystem,
					Name:      webhookQueueLength,
					Help:      "Webhook queue length",
					Type:      gaugeMetric,
				},
				VariableLabels: labels,
				Value:          float64(t.Stats().QueueLength),
			})
			metrics = append(metrics, MetricV2{
				Description: MetricDescription{
					Namespace: clusterMetricNamespace,
					Subsystem: webhookSubsystem,
					Name:      webhookTotalMessages,
					Help:      "Total number of messages sent to this target",
					Type:      counterMetric,
				},
				VariableLabels: labels,
				Value:          float64(t.Stats().TotalMessages),
			})
			metrics = append(metrics, MetricV2{
				Description: MetricDescription{
					Namespace: clusterMetricNamespace,
					Subsystem: webhookSubsystem,
					Name:      webhookFailedMessages,
					Help:      "Number of messages that failed to send",
					Type:      counterMetric,
				},
				VariableLabels: labels,
				Value:          float64(t.Stats().FailedMessages),
			})
		}

		return metrics
	})
	return mg
}

func populateAndPublish(metricsGroups []*MetricsGroupV2, publish func(m MetricV2) bool) {
	for _, mg := range metricsGroups {
		if mg == nil {
			continue
		}
		for _, metric := range mg.Get() {
			if !publish(metric) {
				return
			}
		}
	}
}

func toSnake(camel string) (snake string) {
	var b strings.Builder
	l := len(camel)
	for i, v := range camel {
		// A is 65, a is 97
		if v >= 'a' {
			b.WriteRune(v)
			continue
		}
		// v is capital letter here
		// disregard first letter
		// add underscore if last letter is capital letter
		// add underscore when previous letter is lowercase
		// add underscore when next letter is lowercase
		if (i != 0 || i == l-1) && ((i > 0 && rune(camel[i-1]) >= 'a') ||
			(i < l-1 && rune(camel[i+1]) >= 'a')) {
			b.WriteRune('_')
		}
		b.WriteRune(v + 'a' - 'A')
	}
	return b.String()
}
