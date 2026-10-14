// Copyright (c) 2024 MinIO, Inc.
//
// # This file is part of MinIO Object Storage stack
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
	expiryPendingTasks             = "expiry_pending_tasks"
	transitionActiveTasks          = "transition_active_tasks"
	transitionPendingTasks         = "transition_pending_tasks"
	transitionMissedImmediateTasks = "transition_missed_immediate_tasks"
	versionsScanned                = "versions_scanned"
)

var (
	ilmExpiryPendingTasksMD             = NewGaugeMD(expiryPendingTasks, "Number of pending ILM expiry tasks in the queue")
	ilmTransitionActiveTasksMD          = NewGaugeMD(transitionActiveTasks, "Number of active ILM transition tasks")
	ilmTransitionPendingTasksMD         = NewGaugeMD(transitionPendingTasks, "Number of pending ILM transition tasks in the queue")
	ilmTransitionMissedImmediateTasksMD = NewCounterMD(transitionMissedImmediateTasks, "Number of missed immediate ILM transition tasks")
	ilmVersionsScannedMD                = NewCounterMD(versionsScanned, "Total number of object versions checked for ILM actions since server start")
)

// loadILMMetrics - `MetricsLoaderFn` for ILM metrics.
func (d *Deps) loadILMMetrics(_ context.Context, m MetricValues, _ *metricsCache) error {
	m.Set(expiryPendingTasks, float64(d.ExpiryPendingTasks()))
	active, pending, missed := d.TransitionTasks()
	m.Set(transitionActiveTasks, float64(active))
	m.Set(transitionPendingTasks, float64(pending))
	m.Set(transitionMissedImmediateTasks, float64(missed))
	m.Set(versionsScanned, float64(d.ScannerLifetime(ScannerILM)))

	return nil
}
