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

package metrics

import (
	"context"
	"time"

	"github.com/minio/madmin-go/v3"
	idplugin "github.com/minio/minio/internal/config/identity/plugin"
	"github.com/prometheus/client_golang/prometheus"
)

// Deps carries the server state the v3 metrics subsystem reads. It is wired
// once at bootstrap (see cmd/metrics-v3.go) so this package never reaches
// into package-level globals; every hook is nil-tolerant where the
// corresponding subsystem may be disabled.
type Deps struct {
	// Ctx is the server lifetime context.
	Ctx context.Context

	// Server identity and bootstrap values.
	LocalNodeName string
	LocalPoolIdx  int
	IsDistErasure bool
	BootTime      time.Time

	// LogIf mirrors cmd.metricsLogIf.
	LogIf func(ctx context.Context, err error, tags ...any)

	// --- API / HTTP / connection statistics ---

	// HTTPStats returns server-wide S3 HTTP statistics.
	HTTPStats func() ServerHTTPStats
	// ConnStats returns server-wide connection statistics.
	ConnStats func() ServerConnStats
	// BucketConnStats returns per-bucket in/out byte counters for the
	// requested buckets.
	BucketConnStats func(buckets []string) map[string]InOutBytes
	// BucketHTTPStats returns per-bucket HTTP statistics for one bucket.
	BucketHTTPStats func(bucket string) BucketHTTPStats
	// TTFBHistogram returns the server-wide S3 time-to-first-byte histogram.
	TTFBHistogram func() *prometheus.HistogramVec
	// BucketTTFBHistogram returns the per-bucket S3 TTFB histogram.
	BucketTTFBHistogram func() *prometheus.HistogramVec

	// --- cached cluster sources (network/storage backed) ---

	// PeerOnlineCount returns the number of online and offline peers.
	PeerOnlineCount func() (online, offline int)
	// LoadDataUsage returns the latest data usage census.
	LoadDataUsage func(ctx context.Context) (DataUsageInfo, error)
	// ClusterHealth returns cluster health information.
	ClusterHealth func(ctx context.Context) (HealthResult, error)
	// LocalStorageInfo returns local storage info plus drive counts.
	LocalStorageInfo func(ctx context.Context) (si madmin.StorageInfo, online, offline, total int)
	// ClusterStorageInfo returns cluster storage info plus drive counts.
	ClusterStorageInfo func(ctx context.Context) (si madmin.StorageInfo, online, offline, total int)
	// CollectLocalMetrics collects realtime metrics from the given hosts
	// (used for CPU, memory and per-drive IO stats).
	CollectLocalMetrics func(types madmin.MetricType, hosts map[string]struct{}) madmin.RealtimeMetrics

	// --- domain hooks ---

	// IAMStats returns IAM import counters: last refresh duration in
	// milliseconds, last refresh unix-nano, total failures and successes.
	IAMStats func() (lastSyncMillis, lastSyncUnixNano, failures, successes uint64)
	// AuthNPluginMetrics returns the authn plugin request statistics.
	AuthNPluginMetrics func() idplugin.Metrics
	// BucketQuota returns the quota (bytes) configured for a bucket.
	BucketQuota func(ctx context.Context, bucket string) (*madmin.BucketQuota, error)
	// SiteReplicationEnabled reports whether site replication is active
	// (bucket replication metrics are suppressed in that case).
	SiteReplicationEnabled func() bool
	// ScannerLifetime returns lifetime scanner counters per operation
	// (see the scannerMetric* constants).
	ScannerLifetime func(operation string) uint64
	// ScannerActiveDrives returns the number of drives being scanned.
	ScannerActiveDrives func() int
	// ExpiryPendingTasks returns pending ILM expiry tasks.
	ExpiryPendingTasks func() int
	// TransitionTasks returns active, pending and missed-immediate ILM
	// transition tasks.
	TransitionTasks func() (active, pending, missed int)
	// LockStats returns distributed lock read/write counts.
	LockStats func() (reads, writes int64)
	// EventTargetStats returns bucket-event target queue statistics.
	EventTargetStats func() EventTargetStats
	// CPUAvg returns rolling average CPU idle and iowait percentages,
	// collected by the resource metrics job.
	CPUAvg func() (idle, iowait float64, ok bool)
	// ClusterReplicationQueue returns cluster replication queue summary;
	// nil when no replication stats have been recorded yet.
	ClusterReplicationQueue func() *ReplQueueSummary
	// BucketReplicationStats returns per-bucket replication statistics
	// computed against the given usage census.
	BucketReplicationStats func(usage map[string]BucketUsage) map[string]BucketReplicationStats
}

// ServerHTTPStats is the server HTTP statistics view used by the api
// metrics.
type ServerHTTPStats struct {
	S3RequestsInQueue      int32
	S3RequestsIncoming     uint64
	CurrentS3Requests      map[string]int
	TotalS3Requests        map[string]int
	TotalS3Errors          map[string]int
	TotalS35xxErrors       map[string]int
	TotalS34xxErrors       map[string]int
	TotalS3Canceled        map[string]int
	TotalS3RejectedAuth    uint64
	TotalS3RejectedTime    uint64
	TotalS3RejectedHeader  uint64
	TotalS3RejectedInvalid uint64
}

// ServerConnStats is the server connection statistics view.
type ServerConnStats struct {
	S3InputBytes         uint64
	S3OutputBytes        uint64
	InternodeInputBytes  uint64
	InternodeOutputBytes uint64
}

// InOutBytes is a directional byte counter pair.
type InOutBytes struct {
	In, Out uint64
}

// BucketHTTPStats is the per-bucket HTTP statistics view used by the
// bucket api metrics.
type BucketHTTPStats struct {
	CurrentS3Requests map[string]int
	TotalS3Requests   map[string]int
	TotalS3Canceled   map[string]int
	TotalS34xxErrors  map[string]int
	TotalS35xxErrors  map[string]int
}

// DataUsageInfo is the data usage census view consumed by usage, scanner
// and bucket replication metrics.
type DataUsageInfo struct {
	LastUpdate   time.Time
	BucketsUsage map[string]BucketUsage
}

// BucketUsage is the per-bucket usage view.
type BucketUsage struct {
	Size                    uint64
	ObjectsCount            uint64
	VersionsCount           uint64
	DeleteMarkersCount      uint64
	ObjectSizesHistogram    map[string]uint64
	ObjectVersionsHistogram map[string]uint64
}

// HealthResult is the cluster health view consumed by health and erasure
// set metrics.
type HealthResult struct {
	Healthy     bool
	WriteQuorum int
	ESHealth    []ESHealth
}

// ESHealth is the per-set health view.
type ESHealth struct {
	Healthy       bool
	PoolID        int
	SetID         int
	ReadQuorum    int
	WriteQuorum   int
	HealthyDrives int
	HealingDrives int
}

// EventTargetStats is the bucket-event target statistics view.
type EventTargetStats struct {
	CurrentSendCalls  int64
	EventsErrorsTotal int64
	TotalEvents       int64
	EventsSkipped     int64
}

// ReplQueueSummary is the cluster replication queue view.
type ReplQueueSummary struct {
	// Queue byte/count statistics.
	AvgBytes, AvgCount   uint64
	MaxBytes, MaxCount   uint64
	CurrBytes, CurrCount uint64
	// Active workers.
	AvgActiveWorkers  float64
	CurrActiveWorkers int
	MaxActiveWorkers  int
	// Total ("Total" key) transfer rates; HasTransfer reports whether
	// any transfer statistics were recorded.
	HasTransfer                             bool
	AvgTransfer, CurrTransfer, PeakTransfer float64
	// Recent backlog.
	RecentBacklog uint64
}

// BucketReplicationStats is the per-bucket replication view.
type BucketReplicationStats struct {
	HasUsage bool
	Targets  map[string]BucketReplTargetStats
}

// BucketReplTargetStats is the per-target (ARN) replication view.
type BucketReplTargetStats struct {
	FailedLastHrBytes  uint64
	FailedLastHrCount  uint64
	FailedLastMinBytes uint64
	FailedLastMinCount uint64
	FailedTotalBytes   uint64
	FailedTotalCount   uint64
	ReplicatedCount    uint64
	ReplicatedSize     uint64
	UploadLatencyMs    map[string]uint64 // operation -> milliseconds

	Proxy ProxyStats
}

// ProxyStats is the replication proxy counters view.
type ProxyStats struct {
	GetFailedTotal    uint64
	GetTotal          uint64
	GetTagFailedTotal uint64
	GetTagTotal       uint64
	HeadFailedTotal   uint64
	HeadTotal         uint64
	PutTagFailedTotal uint64
	PutTagTotal       uint64
	RmvTagFailedTotal uint64
	RmvTagTotal       uint64
}

// GetTotalCapacity gets the total capacity in the cluster.
func GetTotalCapacity(diskInfo []madmin.Disk) (capacity uint64) {
	for _, disk := range diskInfo {
		capacity += disk.TotalSpace
	}
	return capacity
}

// GetTotalUsableCapacity gets the total usable capacity in the cluster.
func GetTotalUsableCapacity(diskInfo []madmin.Disk, s madmin.StorageInfo) (capacity uint64) {
	for _, disk := range diskInfo {
		// Ignore invalid.
		if disk.PoolIndex < 0 || len(s.Backend.StandardSCData) <= disk.PoolIndex {
			// https://github.com/minio/minio/issues/16500
			continue
		}
		// Ignore parity disks
		if disk.DiskIndex < s.Backend.StandardSCData[disk.PoolIndex] {
			capacity += disk.TotalSpace
		}
	}
	return capacity
}

// GetTotalCapacityFree gets the total capacity free in the cluster.
func GetTotalCapacityFree(diskInfo []madmin.Disk) (capacity uint64) {
	for _, d := range diskInfo {
		capacity += d.AvailableSpace
	}
	return capacity
}

// GetTotalUsableCapacityFree gets the total usable capacity free in the cluster.
func GetTotalUsableCapacityFree(diskInfo []madmin.Disk, s madmin.StorageInfo) (capacity uint64) {
	for _, disk := range diskInfo {
		// Ignore invalid.
		if disk.PoolIndex < 0 || len(s.Backend.StandardSCData) <= disk.PoolIndex {
			// https://github.com/minio/minio/issues/16500
			continue
		}
		// Ignore parity disks
		if disk.DiskIndex < s.Backend.StandardSCData[disk.PoolIndex] {
			capacity += disk.AvailableSpace
		}
	}
	return capacity
}

// Scanner operations reported via Deps.ScannerLifetime.
const (
	ScannerScanBucketDrive = "scan_bucket_drive"
	ScannerScanFolder      = "scan_folder"
	ScannerScanObject      = "scan_object"
	ScannerApplyVersion    = "apply_version"
	ScannerILM             = "ilm"
)
