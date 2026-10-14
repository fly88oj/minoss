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

package metrics

import (
	"slices"
	"strings"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

// Collector paths.
//
// These are paths under the top-level /minio/metrics/v3 metrics endpoint. Each
// of these paths returns a set of V3 metrics.
//
// Per-bucket metrics endpoints always start with /bucket and the bucket name is
// appended to the path. e.g. if the collector path is /bucket/api, the endpoint
// for the bucket "mybucket" would be /minio/metrics/v3/bucket/api/mybucket
const (
	apiRequestsCollectorPath collectorPath = "/api/requests"

	bucketAPICollectorPath         collectorPath = "/bucket/api"
	bucketReplicationCollectorPath collectorPath = "/bucket/replication"

	systemNetworkInternodeCollectorPath collectorPath = "/system/network/internode"
	systemDriveCollectorPath            collectorPath = "/system/drive"
	systemMemoryCollectorPath           collectorPath = "/system/memory"
	systemCPUCollectorPath              collectorPath = "/system/cpu"
	systemProcessCollectorPath          collectorPath = "/system/process"

	debugGoCollectorPath collectorPath = "/debug/go"

	clusterHealthCollectorPath       collectorPath = "/cluster/health"
	clusterUsageObjectsCollectorPath collectorPath = "/cluster/usage/objects"
	clusterUsageBucketsCollectorPath collectorPath = "/cluster/usage/buckets"
	clusterErasureSetCollectorPath   collectorPath = "/cluster/erasure-set"
	clusterIAMCollectorPath          collectorPath = "/cluster/iam"
	clusterConfigCollectorPath       collectorPath = "/cluster/config"

	ilmCollectorPath           collectorPath = "/ilm"
	auditCollectorPath         collectorPath = "/audit"
	loggerWebhookCollectorPath collectorPath = "/logger/webhook"
	replicationCollectorPath   collectorPath = "/replication"
	notificationCollectorPath  collectorPath = "/notification"
	scannerCollectorPath       collectorPath = "/scanner"
)

const (
	clusterBasePath = "/cluster"
)

type metricsV3Collection struct {
	mgMap       map[collectorPath]*Group
	bucketMGMap map[collectorPath]*Group

	// Gatherers for non-bucket Group's
	mgGatherers map[collectorPath]prometheus.Gatherer

	collectorPaths []collectorPath
}

func newMetricGroups(r *prometheus.Registry, d *Deps) *metricsV3Collection {
	// Create all metric groups.
	apiRequestsMG := NewGroup(apiRequestsCollectorPath,
		[]MetricDescriptor{
			apiRejectedAuthTotalMD,
			apiRejectedHeaderTotalMD,
			apiRejectedTimestampTotalMD,
			apiRejectedInvalidTotalMD,

			apiRequestsWaitingTotalMD,
			apiRequestsIncomingTotalMD,
			apiRequestsInFlightTotalMD,
			apiRequestsTotalMD,
			apiRequestsErrorsTotalMD,
			apiRequests5xxErrorsTotalMD,
			apiRequests4xxErrorsTotalMD,
			apiRequestsCanceledTotalMD,

			apiRequestsTTFBSecondsDistributionMD,

			apiTrafficSentBytesMD,
			apiTrafficRecvBytesMD,
		},
		JoinLoaders(d.loadAPIRequestsHTTPMetrics, d.loadAPIRequestsTTFBMetrics,
			d.loadAPIRequestsNetworkMetrics),
	)

	bucketAPIMG := NewBucketGroup(bucketAPICollectorPath,
		[]MetricDescriptor{
			bucketAPITrafficRecvBytesMD,
			bucketAPITrafficSentBytesMD,

			bucketAPIRequestsInFlightMD,
			bucketAPIRequestsTotalMD,
			bucketAPIRequestsCanceledMD,
			bucketAPIRequests4xxErrorsMD,
			bucketAPIRequests5xxErrorsMD,

			bucketAPIRequestsTTFBSecondsDistributionMD,
		},
		JoinBucketLoaders(d.loadBucketAPIHTTPMetrics, d.loadBucketAPITTFBMetrics),
	)

	bucketReplicationMG := NewBucketGroup(bucketReplicationCollectorPath,
		[]MetricDescriptor{
			bucketReplLastHrFailedBytesMD,
			bucketReplLastHrFailedCountMD,
			bucketReplLastMinFailedBytesMD,
			bucketReplLastMinFailedCountMD,
			bucketReplLatencyMsMD,
			bucketReplProxiedDeleteTaggingRequestsTotalMD,
			bucketReplProxiedGetRequestsFailuresMD,
			bucketReplProxiedGetRequestsTotalMD,
			bucketReplProxiedGetTaggingRequestsFailuresMD,
			bucketReplProxiedGetTaggingRequestsTotalMD,
			bucketReplProxiedHeadRequestsFailuresMD,
			bucketReplProxiedHeadRequestsTotalMD,
			bucketReplProxiedPutTaggingRequestsFailuresMD,
			bucketReplProxiedPutTaggingRequestsTotalMD,
			bucketReplSentBytesMD,
			bucketReplSentCountMD,
			bucketReplTotalFailedBytesMD,
			bucketReplTotalFailedCountMD,
			bucketReplProxiedDeleteTaggingRequestsFailuresMD,
		},
		d.loadBucketReplicationMetrics,
	)

	systemNetworkInternodeMG := NewGroup(systemNetworkInternodeCollectorPath,
		[]MetricDescriptor{
			internodeErrorsTotalMD,
			internodeDialedErrorsTotalMD,
			internodeDialAvgTimeNanosMD,
			internodeSentBytesTotalMD,
			internodeRecvBytesTotalMD,
		},
		d.loadNetworkInternodeMetrics,
	)

	systemMemoryMG := NewGroup(systemMemoryCollectorPath,
		[]MetricDescriptor{
			memTotalMD,
			memUsedMD,
			memFreeMD,
			memAvailableMD,
			memBuffersMD,
			memCacheMD,
			memSharedMD,
			memUsedPercMD,
		},
		d.loadMemoryMetrics,
	)

	systemCPUMG := NewGroup(systemCPUCollectorPath,
		[]MetricDescriptor{
			sysCPUAvgIdleMD,
			sysCPUAvgIOWaitMD,
			sysCPULoadMD,
			sysCPULoadPercMD,
			sysCPUNiceMD,
			sysCPUStealMD,
			sysCPUSystemMD,
			sysCPUUserMD,
		},
		d.loadCPUMetrics,
	)

	systemProcessMG := NewGroup(systemProcessCollectorPath,
		[]MetricDescriptor{
			processLocksReadTotalMD,
			processLocksWriteTotalMD,
			processCPUTotalSecondsMD,
			processGoRoutineTotalMD,
			processIORCharBytesMD,
			processIOReadBytesMD,
			processIOWCharBytesMD,
			processIOWriteBytesMD,
			processStarttimeSecondsMD,
			processUptimeSecondsMD,
			processFileDescriptorLimitTotalMD,
			processFileDescriptorOpenTotalMD,
			processSyscallReadTotalMD,
			processSyscallWriteTotalMD,
			processResidentMemoryBytesMD,
			processVirtualMemoryBytesMD,
			processVirtualMemoryMaxBytesMD,
		},
		d.loadProcessMetrics,
	)

	systemDriveMG := NewGroup(systemDriveCollectorPath,
		[]MetricDescriptor{
			driveUsedBytesMD,
			driveFreeBytesMD,
			driveTotalBytesMD,
			driveUsedInodesMD,
			driveFreeInodesMD,
			driveTotalInodesMD,
			driveTimeoutErrorsMD,
			driveIOErrorsMD,
			driveAvailabilityErrorsMD,
			driveWaitingIOMD,
			driveAPILatencyMD,
			driveHealthMD,

			driveOfflineCountMD,
			driveOnlineCountMD,
			driveCountMD,

			// iostat related
			driveReadsPerSecMD,
			driveReadsKBPerSecMD,
			driveReadsAwaitMD,
			driveWritesPerSecMD,
			driveWritesKBPerSecMD,
			driveWritesAwaitMD,
			drivePercUtilMD,
		},
		d.loadDriveMetrics,
	)

	clusterHealthMG := NewGroup(clusterHealthCollectorPath,
		[]MetricDescriptor{
			healthDrivesOfflineCountMD,
			healthDrivesOnlineCountMD,
			healthDrivesCountMD,

			healthNodesOfflineCountMD,
			healthNodesOnlineCountMD,

			healthCapacityRawTotalBytesMD,
			healthCapacityRawFreeBytesMD,
			healthCapacityUsableTotalBytesMD,
			healthCapacityUsableFreeBytesMD,
		},
		JoinLoaders(d.loadClusterHealthDriveMetrics,
			d.loadClusterHealthNodeMetrics,
			d.loadClusterHealthCapacityMetrics),
	)

	clusterUsageObjectsMG := NewGroup(clusterUsageObjectsCollectorPath,
		[]MetricDescriptor{
			usageSinceLastUpdateSecondsMD,
			usageTotalBytesMD,
			usageObjectsCountMD,
			usageVersionsCountMD,
			usageDeleteMarkersCountMD,
			usageBucketsCountMD,
			usageObjectsDistributionMD,
			usageVersionsDistributionMD,
		},
		d.loadClusterUsageObjectMetrics,
	)

	clusterUsageBucketsMG := NewGroup(clusterUsageBucketsCollectorPath,
		[]MetricDescriptor{
			usageSinceLastUpdateSecondsMD,
			usageBucketTotalBytesMD,
			usageBucketObjectsTotalMD,
			usageBucketVersionsCountMD,
			usageBucketDeleteMarkersCountMD,
			usageBucketQuotaTotalBytesMD,
			usageBucketObjectSizeDistributionMD,
			usageBucketObjectVersionCountDistributionMD,
		},
		d.loadClusterUsageBucketMetrics,
	)

	clusterErasureSetMG := NewGroup(clusterErasureSetCollectorPath,
		[]MetricDescriptor{
			erasureSetOverallWriteQuorumMD,
			erasureSetOverallHealthMD,
			erasureSetReadQuorumMD,
			erasureSetWriteQuorumMD,
			erasureSetOnlineDrivesCountMD,
			erasureSetHealingDrivesCountMD,
			erasureSetHealthMD,
			erasureSetReadToleranceMD,
			erasureSetWriteToleranceMD,
			erasureSetReadHealthMD,
			erasureSetWriteHealthMD,
		},
		d.loadClusterErasureSetMetrics,
	)

	clusterNotificationMG := NewGroup(notificationCollectorPath,
		[]MetricDescriptor{
			notificationCurrentSendInProgressMD,
			notificationEventsErrorsTotalMD,
			notificationEventsSentTotalMD,
			notificationEventsSkippedTotalMD,
		},
		d.loadClusterNotificationMetrics,
	)

	clusterIAMMG := NewGroup(clusterIAMCollectorPath,
		[]MetricDescriptor{
			lastSyncDurationMillisMD,
			pluginAuthnServiceFailedRequestsMinuteMD,
			pluginAuthnServiceLastFailSecondsMD,
			pluginAuthnServiceLastSuccSecondsMD,
			pluginAuthnServiceSuccAvgRttMsMinuteMD,
			pluginAuthnServiceSuccMaxRttMsMinuteMD,
			pluginAuthnServiceTotalRequestsMinuteMD,
			sinceLastSyncMillisMD,
			syncFailuresMD,
			syncSuccessesMD,
		},
		d.loadClusterIAMMetrics,
	)

	clusterReplicationMG := NewGroup(replicationCollectorPath,
		[]MetricDescriptor{
			replicationAverageActiveWorkersMD,
			replicationAverageQueuedBytesMD,
			replicationAverageQueuedCountMD,
			replicationAverageDataTransferRateMD,
			replicationCurrentActiveWorkersMD,
			replicationCurrentDataTransferRateMD,
			replicationLastMinuteQueuedBytesMD,
			replicationLastMinuteQueuedCountMD,
			replicationMaxActiveWorkersMD,
			replicationMaxQueuedBytesMD,
			replicationMaxQueuedCountMD,
			replicationMaxDataTransferRateMD,
			replicationRecentBacklogCountMD,
		},
		d.loadClusterReplicationMetrics,
	)

	clusterConfigMG := NewGroup(clusterConfigCollectorPath,
		[]MetricDescriptor{
			configRRSParityMD,
			configStandardParityMD,
		},
		d.loadClusterConfigMetrics,
	)

	scannerMG := NewGroup(scannerCollectorPath,
		[]MetricDescriptor{
			scannerBucketScansFinishedMD,
			scannerBucketScansStartedMD,
			scannerDirectoriesScannedMD,
			scannerObjectsScannedMD,
			scannerVersionsScannedMD,
			scannerLastActivitySecondsMD,
		},
		d.loadClusterScannerMetrics,
	)

	loggerWebhookMG := NewGroup(loggerWebhookCollectorPath,
		[]MetricDescriptor{
			webhookFailedMessagesMD,
			webhookQueueLengthMD,
			webhookTotalMessagesMD,
		},
		d.loadLoggerWebhookMetrics,
	)

	auditMG := NewGroup(auditCollectorPath,
		[]MetricDescriptor{
			auditFailedMessagesMD,
			auditTargetQueueLengthMD,
			auditTotalMessagesMD,
		},
		d.loadAuditMetrics,
	)

	ilmMG := NewGroup(ilmCollectorPath,
		[]MetricDescriptor{
			ilmExpiryPendingTasksMD,
			ilmTransitionActiveTasksMD,
			ilmTransitionPendingTasksMD,
			ilmTransitionMissedImmediateTasksMD,
			ilmVersionsScannedMD,
		},
		d.loadILMMetrics,
	)

	allMetricGroups := []*Group{
		apiRequestsMG,
		bucketAPIMG,
		bucketReplicationMG,

		systemNetworkInternodeMG,
		systemDriveMG,
		systemMemoryMG,
		systemCPUMG,
		systemProcessMG,

		clusterHealthMG,
		clusterUsageObjectsMG,
		clusterUsageBucketsMG,
		clusterErasureSetMG,
		clusterNotificationMG,
		clusterIAMMG,
		clusterReplicationMG,
		clusterConfigMG,

		ilmMG,
		scannerMG,
		auditMG,
		loggerWebhookMG,
	}

	// Bucket metrics are special, they always include the bucket label. These
	// metrics required a list of buckets to be passed to the loader, and the list
	// of buckets is not known until the request is made. So we keep a separate
	// map for bucket metrics and handle them specially.

	// Add the serverName and poolIndex labels to all non-cluster metrics.
	//
	// Also create metric group maps and set the cache.
	metricsCache := newMetricsCache(d)
	mgMap := make(map[collectorPath]*Group)
	bucketMGMap := make(map[collectorPath]*Group)
	for _, mg := range allMetricGroups {
		mg.ctx = d.Ctx
		if !strings.HasPrefix(string(mg.CollectorPath), clusterBasePath) {
			mg.AddExtraLabels(
				serverNameLabel, d.LocalNodeName,
				// poolIndex, strconv.Itoa(globalLocalPoolIdx),
			)
		}
		mg.SetCache(metricsCache)
		if mg.IsBucketGroup() {
			bucketMGMap[mg.CollectorPath] = mg
		} else {
			mgMap[mg.CollectorPath] = mg
		}
	}

	// Prepare to register the collectors. Other than `MetricGroup` collectors,
	// we also have standard collectors like `GoCollector`.

	// Create all Non-`MetricGroup` collectors here.
	collectors := map[collectorPath]prometheus.Collector{
		debugGoCollectorPath: collectors.NewGoCollector(),
	}

	// Add all `MetricGroup` collectors to the map.
	for _, mg := range allMetricGroups {
		collectors[mg.CollectorPath] = mg
	}

	// Helper function to register a collector and return a gatherer for it.
	mustRegister := func(c ...prometheus.Collector) prometheus.Gatherer {
		subRegistry := prometheus.NewRegistry()
		for _, col := range c {
			subRegistry.MustRegister(col)
		}
		r.MustRegister(subRegistry)
		return subRegistry
	}

	// Register all collectors and create gatherers for them.
	gatherers := make(map[collectorPath]prometheus.Gatherer, len(collectors))
	collectorPaths := make([]collectorPath, 0, len(collectors))
	for path, collector := range collectors {
		gatherers[path] = mustRegister(collector)
		collectorPaths = append(collectorPaths, path)
	}
	slices.Sort(collectorPaths)
	return &metricsV3Collection{
		mgMap:          mgMap,
		bucketMGMap:    bucketMGMap,
		mgGatherers:    gatherers,
		collectorPaths: collectorPaths,
	}
}

// serverNameLabel is the label key carrying the server name.
const serverNameLabel = "server"
