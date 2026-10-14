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
	"context"
)

const (
	replicationAverageActiveWorkers    = "average_active_workers"
	replicationAverageQueuedBytes      = "average_queued_bytes"
	replicationAverageQueuedCount      = "average_queued_count"
	replicationAverageDataTransferRate = "average_data_transfer_rate"
	replicationCurrentActiveWorkers    = "current_active_workers"
	replicationCurrentDataTransferRate = "current_data_transfer_rate"
	replicationLastMinuteQueuedBytes   = "last_minute_queued_bytes"
	replicationLastMinuteQueuedCount   = "last_minute_queued_count"
	replicationMaxActiveWorkers        = "max_active_workers"
	replicationMaxQueuedBytes          = "max_queued_bytes"
	replicationMaxQueuedCount          = "max_queued_count"
	replicationMaxDataTransferRate     = "max_data_transfer_rate"
	replicationRecentBacklogCount      = "recent_backlog_count"
)

var (
	replicationAverageActiveWorkersMD = NewGaugeMD(replicationAverageActiveWorkers,
		"Average number of active replication workers")
	replicationAverageQueuedBytesMD = NewGaugeMD(replicationAverageQueuedBytes,
		"Average number of bytes queued for replication since server start")
	replicationAverageQueuedCountMD = NewGaugeMD(replicationAverageQueuedCount,
		"Average number of objects queued for replication since server start")
	replicationAverageDataTransferRateMD = NewGaugeMD(replicationAverageDataTransferRate,
		"Average replication data transfer rate in bytes/sec")
	replicationCurrentActiveWorkersMD = NewGaugeMD(replicationCurrentActiveWorkers,
		"Total number of active replication workers")
	replicationCurrentDataTransferRateMD = NewGaugeMD(replicationCurrentDataTransferRate,
		"Current replication data transfer rate in bytes/sec")
	replicationLastMinuteQueuedBytesMD = NewGaugeMD(replicationLastMinuteQueuedBytes,
		"Number of bytes queued for replication in the last full minute")
	replicationLastMinuteQueuedCountMD = NewGaugeMD(replicationLastMinuteQueuedCount,
		"Number of objects queued for replication in the last full minute")
	replicationMaxActiveWorkersMD = NewGaugeMD(replicationMaxActiveWorkers,
		"Maximum number of active replication workers seen since server start")
	replicationMaxQueuedBytesMD = NewGaugeMD(replicationMaxQueuedBytes,
		"Maximum number of bytes queued for replication since server start")
	replicationMaxQueuedCountMD = NewGaugeMD(replicationMaxQueuedCount,
		"Maximum number of objects queued for replication since server start")
	replicationMaxDataTransferRateMD = NewGaugeMD(replicationMaxDataTransferRate,
		"Maximum replication data transfer rate in bytes/sec seen since server start")
	replicationRecentBacklogCountMD = NewGaugeMD(replicationRecentBacklogCount,
		"Total number of objects seen in replication backlog in the last 5 minutes")
)

// loadClusterReplicationMetrics - `MetricsLoaderFn` for cluster replication metrics
// such as transfer rate and objects queued.
func (d *Deps) loadClusterReplicationMetrics(ctx context.Context, m MetricValues, c *metricsCache) error {
	qs := d.ClusterReplicationQueue()
	if qs == nil {
		return nil
	}

	m.Set(replicationAverageQueuedBytes, float64(qs.AvgBytes))
	m.Set(replicationAverageQueuedCount, float64(qs.AvgCount))
	m.Set(replicationMaxQueuedBytes, float64(qs.MaxBytes))
	m.Set(replicationMaxQueuedCount, float64(qs.MaxCount))
	m.Set(replicationLastMinuteQueuedBytes, float64(qs.CurrBytes))
	m.Set(replicationLastMinuteQueuedCount, float64(qs.CurrCount))

	m.Set(replicationAverageActiveWorkers, qs.AvgActiveWorkers)
	m.Set(replicationCurrentActiveWorkers, float64(qs.CurrActiveWorkers))
	m.Set(replicationMaxActiveWorkers, float64(qs.MaxActiveWorkers))

	if qs.HasTransfer {
		m.Set(replicationAverageDataTransferRate, qs.AvgTransfer)
		m.Set(replicationCurrentDataTransferRate, qs.CurrTransfer)
		m.Set(replicationMaxDataTransferRate, qs.PeakTransfer)
	}

	m.Set(replicationRecentBacklogCount, float64(qs.RecentBacklog))
	return nil
}
