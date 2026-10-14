// Copyright (c) 2015-2026 MinIO, Inc.
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

// Wiring for the metrics v3 subsystem: internal/metrics owns the
// exposition machinery; this file adapts server state (globals and cmd
// types) into the narrow metrics.Deps seam.

package cmd

import (
	"context"
	"sync/atomic"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/minio/madmin-go/v3"
	idplugin "github.com/minio/minio/internal/config/identity/plugin"
	"github.com/minio/minio/internal/metrics"
)

// newMetricsDeps builds the dependency bundle for the metrics v3 server
// from package-level server state. Called once when the router is
// registered; every subsystem may still be nil/disabled at that point.
func newMetricsDeps() *metrics.Deps {
	return &metrics.Deps{
		Ctx:           GlobalContext,
		LocalNodeName: globalLocalNodeName,
		LocalPoolIdx:  globalLocalPoolIdx,
		IsDistErasure: globalIsDistErasure,
		BootTime:      globalBootTime,

		LogIf: func(ctx context.Context, err error, tags ...any) {
			metricsLogIf(ctx, err, tags...)
		},

		HTTPStats: func() metrics.ServerHTTPStats {
			return toMetricsHTTPStats(globalHTTPStats.toServerHTTPStats(false))
		},
		ConnStats: func() metrics.ServerConnStats {
			cs := globalConnStats.toServerConnStats()
			return metrics.ServerConnStats{
				S3InputBytes:         cs.s3InputBytes,
				S3OutputBytes:        cs.s3OutputBytes,
				InternodeInputBytes:  cs.internodeInputBytes,
				InternodeOutputBytes: cs.internodeOutputBytes,
			}
		},
		BucketConnStats: func(buckets []string) map[string]metrics.InOutBytes {
			out := make(map[string]metrics.InOutBytes)
			for b, v := range globalBucketConnStats.getBucketS3InOutBytes(buckets) {
				out[b] = metrics.InOutBytes{In: v.In, Out: v.Out}
			}
			return out
		},
		BucketHTTPStats: func(bucket string) metrics.BucketHTTPStats {
			hs := globalBucketHTTPStats.load(bucket)
			return metrics.BucketHTTPStats{
				CurrentS3Requests: hs.currentS3Requests.Load(false),
				TotalS3Requests:   hs.totalS3Requests.Load(false),
				TotalS3Canceled:   hs.totalS3Canceled.Load(false),
				TotalS34xxErrors:  hs.totalS34xxErrors.Load(false),
				TotalS35xxErrors:  hs.totalS35xxErrors.Load(false),
			}
		},
		TTFBHistogram:       func() *prometheus.HistogramVec { return httpRequestsDuration },
		BucketTTFBHistogram: func() *prometheus.HistogramVec { return bucketHTTPRequestsDuration },

		PeerOnlineCount: func() (online, offline int) {
			if globalNotificationSys == nil {
				return 0, 0
			}
			return globalNotificationSys.GetPeerOnlineCount()
		},
		LoadDataUsage: func(ctx context.Context) (metrics.DataUsageInfo, error) {
			objLayer := newObjectLayerFn()
			if objLayer == nil {
				return metrics.DataUsageInfo{}, nil
			}
			dui, err := loadDataUsageFromBackend(ctx, objLayer)
			if err != nil {
				return metrics.DataUsageInfo{}, err
			}
			return toMetricsDataUsageInfo(dui), nil
		},
		ClusterHealth: func(ctx context.Context) (metrics.HealthResult, error) {
			objLayer := newObjectLayerFn()
			if objLayer == nil {
				return metrics.HealthResult{}, nil
			}
			hr := objLayer.Health(ctx, HealthOptions{})
			out := metrics.HealthResult{
				Healthy:     hr.Healthy,
				WriteQuorum: hr.WriteQuorum,
			}
			for _, es := range hr.ESHealth {
				out.ESHealth = append(out.ESHealth, metrics.ESHealth{
					Healthy:       es.Healthy,
					PoolID:        es.PoolID,
					SetID:         es.SetID,
					ReadQuorum:    es.ReadQuorum,
					WriteQuorum:   es.WriteQuorum,
					HealthyDrives: es.HealthyDrives,
					HealingDrives: es.HealingDrives,
				})
			}
			return out, nil
		},
		LocalStorageInfo: func(ctx context.Context) (si madmin.StorageInfo, online, offline, total int) {
			objLayer := newObjectLayerFn()
			if objLayer == nil {
				return
			}
			si = objLayer.LocalStorageInfo(ctx, true)
			on, off := getOnlineOfflineDisksStats(si.Disks)
			return si, on.Sum(), off.Sum(), on.Merge(off).Sum()
		},
		ClusterStorageInfo: func(ctx context.Context) (si madmin.StorageInfo, online, offline, total int) {
			objLayer := newObjectLayerFn()
			if objLayer == nil {
				return
			}
			si = objLayer.StorageInfo(ctx, true)
			on, off := getOnlineOfflineDisksStats(si.Disks)
			return si, on.Sum(), off.Sum(), on.Merge(off).Sum()
		},
		CollectLocalMetrics: func(types madmin.MetricType, hosts map[string]struct{}) madmin.RealtimeMetrics {
			return collectLocalMetrics(types, collectMetricsOpts{hosts: hosts})
		},

		IAMStats: func() (lastSyncMillis, lastSyncUnixNano, failures, successes uint64) {
			if globalIAMSys == nil {
				return 0, 0, 0, 0
			}
			return atomic.LoadUint64(&globalIAMSys.LastRefreshDurationMilliseconds),
				atomic.LoadUint64(&globalIAMSys.LastRefreshTimeUnixNano),
				atomic.LoadUint64(&globalIAMSys.TotalRefreshFailures),
				atomic.LoadUint64(&globalIAMSys.TotalRefreshSuccesses)
		},
		AuthNPluginMetrics: func() (m idplugin.Metrics) {
			globalAuthPluginMutex.Lock()
			p := globalAuthNPlugin
			globalAuthPluginMutex.Unlock()
			if p == nil {
				return m
			}
			return p.Metrics()
		},
		BucketQuota: func(ctx context.Context, bucket string) (*madmin.BucketQuota, error) {
			return globalBucketQuotaSys.Get(ctx, bucket)
		},
		SiteReplicationEnabled: globalSiteReplicationSys.isEnabled,
		ScannerLifetime: func(operation string) uint64 {
			var m scannerMetric
			switch operation {
			case metrics.ScannerScanBucketDrive:
				m = scannerMetricScanBucketDrive
			case metrics.ScannerScanFolder:
				m = scannerMetricScanFolder
			case metrics.ScannerScanObject:
				m = scannerMetricScanObject
			case metrics.ScannerApplyVersion:
				m = scannerMetricApplyVersion
			case metrics.ScannerILM:
				m = scannerMetricILM
			default:
				return 0
			}
			return globalScannerMetrics.lifetime(m)
		},
		ScannerActiveDrives: globalScannerMetrics.activeDrives,
		ExpiryPendingTasks: func() int {
			if globalExpiryState == nil {
				return 0
			}
			return globalExpiryState.PendingTasks()
		},
		TransitionTasks: func() (active, pending, missed int) {
			if globalTransitionState == nil {
				return 0, 0, 0
			}
			return int(globalTransitionState.ActiveTasks()), globalTransitionState.PendingTasks(), int(globalTransitionState.MissedImmediateTasks())
		},
		LockStats: func() (reads, writes int64) {
			if globalLockServer == nil {
				return 0, 0
			}
			st := globalLockServer.stats()
			return int64(st.Reads), int64(st.Writes)
		},
		EventTargetStats: func() metrics.EventTargetStats {
			if globalEventNotifier == nil {
				return metrics.EventTargetStats{}
			}
			st := globalEventNotifier.targetList.Stats()
			return metrics.EventTargetStats{
				CurrentSendCalls:  st.CurrentSendCalls,
				EventsErrorsTotal: st.EventsErrorsTotal,
				TotalEvents:       st.TotalEvents,
				EventsSkipped:     st.EventsSkipped,
			}
		},
		CPUAvg: func() (idle, iowait float64, ok bool) {
			resourceMetricsMapMu.RLock()
			defer resourceMetricsMapMu.RUnlock()
			cpu, found := resourceMetricsMap[cpuSubsystem]
			if !found {
				return 0, 0, false
			}
			idleM, ok1 := cpu[getResourceKey(cpuIdle, nil)]
			iowaitM, ok2 := cpu[getResourceKey(cpuIOWait, nil)]
			if !ok1 || !ok2 {
				return 0, 0, false
			}
			return idleM.Avg, iowaitM.Avg, true
		},
		ClusterReplicationQueue: func() *metrics.ReplQueueSummary {
			st := globalReplicationStats.Load()
			if st == nil {
				return nil
			}
			qs := st.getNodeQueueStatsSummary()
			out := &metrics.ReplQueueSummary{
				AvgBytes:  uint64(qs.QStats.Avg.Bytes),
				AvgCount:  uint64(qs.QStats.Avg.Count),
				MaxBytes:  uint64(qs.QStats.Max.Bytes),
				MaxCount:  uint64(qs.QStats.Max.Count),
				CurrBytes: uint64(qs.QStats.Curr.Bytes),
				CurrCount: uint64(qs.QStats.Curr.Count),

				AvgActiveWorkers:  float64(qs.ActiveWorkers.Avg),
				CurrActiveWorkers: qs.ActiveWorkers.Curr,
				MaxActiveWorkers:  qs.ActiveWorkers.Max,

				RecentBacklog: qs.MRFStats.LastFailedCount,
			}
			if tots, found := qs.XferStats[Total]; found {
				out.HasTransfer = true
				out.AvgTransfer = tots.Avg
				out.CurrTransfer = tots.Curr
				out.PeakTransfer = tots.Peak
			}
			return out
		},
		BucketReplicationStats: func(usage map[string]metrics.BucketUsage) map[string]metrics.BucketReplicationStats {
			st := globalReplicationStats.Load()
			if st == nil {
				return nil
			}
			cmdUsage := make(map[string]BucketUsageInfo, len(usage))
			for b, u := range usage {
				cmdUsage[b] = BucketUsageInfo{
					Size:                    u.Size,
					ObjectsCount:            u.ObjectsCount,
					VersionsCount:           u.VersionsCount,
					DeleteMarkersCount:      u.DeleteMarkersCount,
					ObjectSizesHistogram:    u.ObjectSizesHistogram,
					ObjectVersionsHistogram: u.ObjectVersionsHistogram,
				}
			}
			out := make(map[string]metrics.BucketReplicationStats)
			for bucket, bs := range st.getAllLatest(cmdUsage) {
				v := metrics.BucketReplicationStats{
					HasUsage: bs.ReplicationStats.hasReplicationUsage(),
					Targets:  make(map[string]metrics.BucketReplTargetStats, len(bs.ReplicationStats.Stats)),
				}
				for arn, stat := range bs.ReplicationStats.Stats {
					v.Targets[arn] = metrics.BucketReplTargetStats{
						FailedLastHrBytes:  uint64(stat.Failed.LastHour.Bytes),
						FailedLastHrCount:  uint64(stat.Failed.LastHour.Count),
						FailedLastMinBytes: uint64(stat.Failed.LastMinute.Bytes),
						FailedLastMinCount: uint64(stat.Failed.LastMinute.Count),
						FailedTotalBytes:   uint64(stat.Failed.Totals.Bytes),
						FailedTotalCount:   uint64(stat.Failed.Totals.Count),
						ReplicatedCount:    uint64(stat.ReplicatedCount),
						ReplicatedSize:     uint64(stat.ReplicatedSize),
						UploadLatencyMs:    stat.Latency.getUploadLatency(),
						Proxy: metrics.ProxyStats{
							GetFailedTotal:    bs.ProxyStats.GetFailedTotal,
							GetTotal:          bs.ProxyStats.GetTotal,
							GetTagFailedTotal: bs.ProxyStats.GetTagFailedTotal,
							GetTagTotal:       bs.ProxyStats.GetTagTotal,
							HeadFailedTotal:   bs.ProxyStats.HeadFailedTotal,
							HeadTotal:         bs.ProxyStats.HeadTotal,
							PutTagFailedTotal: bs.ProxyStats.PutTagFailedTotal,
							PutTagTotal:       bs.ProxyStats.PutTagTotal,
							RmvTagFailedTotal: bs.ProxyStats.RmvTagFailedTotal,
							RmvTagTotal:       bs.ProxyStats.RmvTagTotal,
						},
					}
				}
				out[bucket] = v
			}
			return out
		},
	}
}

// toMetricsHTTPStats converts the cmd HTTP statistics type to the metrics
// package view.
func toMetricsHTTPStats(hs ServerHTTPStats) metrics.ServerHTTPStats {
	return metrics.ServerHTTPStats{
		S3RequestsInQueue:      hs.S3RequestsInQueue,
		S3RequestsIncoming:     hs.S3RequestsIncoming,
		CurrentS3Requests:      hs.CurrentS3Requests.APIStats,
		TotalS3Requests:        hs.TotalS3Requests.APIStats,
		TotalS3Errors:          hs.TotalS3Errors.APIStats,
		TotalS35xxErrors:       hs.TotalS35xxErrors.APIStats,
		TotalS34xxErrors:       hs.TotalS34xxErrors.APIStats,
		TotalS3Canceled:        hs.TotalS3Canceled.APIStats,
		TotalS3RejectedAuth:    hs.TotalS3RejectedAuth,
		TotalS3RejectedTime:    hs.TotalS3RejectedTime,
		TotalS3RejectedHeader:  hs.TotalS3RejectedHeader,
		TotalS3RejectedInvalid: hs.TotalS3RejectedInvalid,
	}
}

// toMetricsDataUsageInfo converts the cmd data usage census to the metrics
// package view.
func toMetricsDataUsageInfo(dui DataUsageInfo) metrics.DataUsageInfo {
	out := metrics.DataUsageInfo{
		LastUpdate:   dui.LastUpdate,
		BucketsUsage: make(map[string]metrics.BucketUsage, len(dui.BucketsUsage)),
	}
	for b, u := range dui.BucketsUsage {
		out.BucketsUsage[b] = metrics.BucketUsage{
			Size:                    u.Size,
			ObjectsCount:            u.ObjectsCount,
			VersionsCount:           u.VersionsCount,
			DeleteMarkersCount:      u.DeleteMarkersCount,
			ObjectSizesHistogram:    u.ObjectSizesHistogram,
			ObjectVersionsHistogram: u.ObjectVersionsHistogram,
		}
	}
	return out
}
