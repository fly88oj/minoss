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
	"fmt"
	"io"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/minio/madmin-go/v3"
	idplugin "github.com/minio/minio/internal/config/identity/plugin"
	"github.com/minio/mux"
	"github.com/prometheus/client_golang/prometheus"
)

// mountV3 mounts the server behind its real mux route so that
// mux.Vars()["pathComps"] is populated like in production.
func mountV3(srv *Server) http.Handler {
	r := mux.NewRouter()
	r.Methods(http.MethodGet).Path("/minio/metrics/v3{pathComps:.*}").Handler(srv)
	return r
}

// testDeps returns a fully stubbed Deps: every hook returns deterministic
// values, so loaders can run without any server state.
func testDeps() *Deps {
	return &Deps{
		Ctx:           context.Background(),
		LocalNodeName: "test-node",
		LocalPoolIdx:  0,
		IsDistErasure: true,
		BootTime:      time.Now().Add(-time.Hour),

		LogIf: func(ctx context.Context, err error, tags ...any) {},

		HTTPStats: func() ServerHTTPStats {
			return ServerHTTPStats{
				S3RequestsInQueue:   2,
				S3RequestsIncoming:  10,
				CurrentS3Requests:   map[string]int{"GetObject": 1},
				TotalS3Requests:     map[string]int{"GetObject": 42},
				TotalS3Errors:       map[string]int{"GetObject": 3},
				TotalS34xxErrors:    map[string]int{"GetObject": 2},
				TotalS35xxErrors:    map[string]int{"GetObject": 1},
				TotalS3Canceled:     map[string]int{"GetObject": 0},
				TotalS3RejectedAuth: 4,
			}
		},
		ConnStats: func() ServerConnStats {
			return ServerConnStats{S3InputBytes: 100, S3OutputBytes: 200, InternodeInputBytes: 1, InternodeOutputBytes: 2}
		},
		BucketConnStats: func(buckets []string) map[string]InOutBytes {
			return map[string]InOutBytes{"b1": {In: 5, Out: 6}}
		},
		BucketHTTPStats: func(bucket string) BucketHTTPStats {
			return BucketHTTPStats{TotalS3Requests: map[string]int{"GetObject": 7}}
		},
		TTFBHistogram: func() *prometheus.HistogramVec {
			h := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "ttfb"}, []string{"api"})
			h.WithLabelValues("GetObject").Observe(0.1)
			return h
		},
		BucketTTFBHistogram: func() *prometheus.HistogramVec {
			return prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "bttfb"}, []string{"api", "bucket"})
		},

		PeerOnlineCount: func() (int, int) { return 3, 1 },
		LoadDataUsage: func(ctx context.Context) (DataUsageInfo, error) {
			return DataUsageInfo{
				LastUpdate: time.Now(),
				BucketsUsage: map[string]BucketUsage{
					"b1": {
						Size: 123, ObjectsCount: 4, VersionsCount: 6, DeleteMarkersCount: 1,
						ObjectSizesHistogram: map[string]uint64{"0-1KB": 4},
					},
				},
			}, nil
		},
		ClusterHealth: func(ctx context.Context) (HealthResult, error) {
			return HealthResult{
				Healthy:     true,
				WriteQuorum: 4,
				ESHealth: []ESHealth{{
					Healthy: true, PoolID: 0, SetID: 1, ReadQuorum: 4, WriteQuorum: 4, HealthyDrives: 8, HealingDrives: 1,
				}},
			}, nil
		},
		LocalStorageInfo: func(ctx context.Context) (madmin.StorageInfo, int, int, int) {
			return madmin.StorageInfo{}, 7, 1, 8
		},
		ClusterStorageInfo: func(ctx context.Context) (madmin.StorageInfo, int, int, int) {
			return madmin.StorageInfo{Backend: madmin.BackendInfo{StandardSCParity: 4}}, 15, 1, 16
		},
		CollectLocalMetrics: func(types madmin.MetricType, hosts map[string]struct{}) madmin.RealtimeMetrics {
			return madmin.RealtimeMetrics{}
		},

		IAMStats:           func() (uint64, uint64, uint64, uint64) { return 11, 22, 33, 44 },
		AuthNPluginMetrics: func() idplugin.Metrics { return idplugin.Metrics{} },
		BucketQuota: func(ctx context.Context, bucket string) (*madmin.BucketQuota, error) {
			return &madmin.BucketQuota{Quota: 1024}, nil
		},
		SiteReplicationEnabled: func() bool { return false },
		ScannerLifetime:        func(op string) uint64 { return 9 },
		ScannerActiveDrives:    func() int { return 4 },
		ExpiryPendingTasks:     func() int { return 2 },
		TransitionTasks:        func() (int, int, int) { return 1, 5, 0 },
		LockStats:              func() (int64, int64) { return 100, 200 },
		EventTargetStats: func() EventTargetStats {
			return EventTargetStats{CurrentSendCalls: 1, TotalEvents: 10, EventsErrorsTotal: 2, EventsSkipped: 3}
		},
		CPUAvg: func() (float64, float64, bool) { return 12.345, 1.5, true },
		ClusterReplicationQueue: func() *ReplQueueSummary {
			return &ReplQueueSummary{
				AvgBytes: 10, AvgCount: 20, MaxBytes: 30, MaxCount: 40, CurrBytes: 50, CurrCount: 60,
				AvgActiveWorkers: 2.5, CurrActiveWorkers: 2, MaxActiveWorkers: 3,
				HasTransfer: true, AvgTransfer: 1, CurrTransfer: 2, PeakTransfer: 3,
				RecentBacklog: 7,
			}
		},
		BucketReplicationStats: func(usage map[string]BucketUsage) map[string]BucketReplicationStats {
			return map[string]BucketReplicationStats{
				"b1": {HasUsage: true, Targets: map[string]BucketReplTargetStats{
					"arn:1": {
						FailedTotalCount: 1, ReplicatedCount: 9, ReplicatedSize: 99,
						UploadLatencyMs: map[string]uint64{"upload": 12},
					},
				}},
			}
		},
	}
}

func TestV3ServerListMetrics(t *testing.T) {
	t.Parallel()
	srv := NewV3Server(func(h http.Handler) http.Handler { return h }, testDeps())

	// The production middleware chain parses the form before the metrics
	// server sees the request; mirror that here.
	req := httptest.NewRequest(http.MethodGet, "/minio/metrics/v3?list", nil)
	req.ParseForm()
	rec := httptest.NewRecorder()
	mountV3(srv).ServeHTTP(rec, req)
	resp := &http.Response{StatusCode: rec.Code, Body: io.NopCloser(rec.Body)}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	// The listing is rendered as a table of metric names.
	for _, want := range []string{
		"minio_cluster_iam_last_sync_duration_millis",
		"minio_system_memory_used",
		"minio_replication_recent_backlog_count",
		"minio_api_requests_total",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("list output missing %q", want)
		}
	}
}

func TestV3ServerCollectIAM(t *testing.T) {
	t.Parallel()
	srv := NewV3Server(func(h http.Handler) http.Handler { return h }, testDeps())
	ts := httptest.NewServer(mountV3(srv))
	defer ts.Close()

	resp, err := http.Get(ts.URL + "/minio/metrics/v3/cluster/iam")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	for _, want := range []string{
		"minio_cluster_iam_last_sync_duration_millis 11",
		"minio_cluster_iam_sync_successes 44",
	} {
		if !strings.Contains(string(body), want) {
			t.Errorf("exposition missing %q", want)
		}
	}
}

func TestV3ServerCollectReplicationAndILM(t *testing.T) {
	t.Parallel()
	srv := NewV3Server(func(h http.Handler) http.Handler { return h }, testDeps())
	ts := httptest.NewServer(mountV3(srv))
	defer ts.Close()

	for path, want := range map[string]string{
		"/minio/metrics/v3/replication": "minio_replication_last_minute_queued_count{server=\"test-node\"} 60",
		"/minio/metrics/v3/ilm":         "minio_ilm_transition_active_tasks{server=\"test-node\"} 1",
	} {
		resp, err := http.Get(ts.URL + path)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(resp.Body)
		resp.Body.Close()
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("%s: status = %d, want 200", path, resp.StatusCode)
		}
		if !strings.Contains(string(body), want) {
			t.Errorf("%s: exposition missing %q", path, want)
		}
	}
}

func TestV3ServerBucketReplication(t *testing.T) {
	t.Parallel()
	d := testDeps()
	srv := NewV3Server(func(h http.Handler) http.Handler { return h }, d)

	// Bucket collectors take the bucket name as the last path component.
	req := httptest.NewRequest(http.MethodGet, "/minio/metrics/v3/bucket/replication/b1", nil)
	rec := httptest.NewRecorder()
	mountV3(srv).ServeHTTP(rec, req)
	body := rec.Body.String()
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if !strings.Contains(body, "minio_bucket_replication_sent_count{") {
		t.Errorf("bucket replication metrics missing, got:\n%s", body)
	}
}

func TestMetricValuesSetAndPanic(t *testing.T) {
	t.Parallel()
	descs := map[MetricName]MetricDescriptor{
		"some_total": NewCounterMD("some_total", "help", "bucket"),
	}
	mv := newMetricValues(descs)
	mv.Set("some_total", 3.5, "bucket", "b")
	if mv.ToPromMetrics("prefix_", nil) == nil {
		t.Error("expected prom metrics")
	}

	defer func() {
		if recover() == nil {
			t.Error("Set with unknown name should panic")
		}
	}()
	mv.Set("missing", 1)
}

func TestHistogramMetrics(t *testing.T) {
	t.Parallel()
	h := prometheus.NewHistogramVec(prometheus.HistogramOpts{Name: "test_hist"}, []string{"api"})
	h.WithLabelValues("GetObject").Observe(0.05)

	ms := histogramMetrics(h, false)
	if len(ms) == 0 {
		t.Fatal("expected histogram bucket metrics")
	}
	found := false
	for _, m := range ms {
		if m.VariableLabels["api"] == "GetObject" && m.VariableLabels["le"] == fmtInf() {
			found = true
		}
	}
	if !found {
		t.Error("missing +Inf bucket")
	}
}

func fmtInf() string { return fmt.Sprintf("%.3f", math.Inf(+1)) }

func TestCapacityFuncs(t *testing.T) {
	t.Parallel()
	disks := []madmin.Disk{
		{PoolIndex: 0, DiskIndex: 0, TotalSpace: 100, AvailableSpace: 40},
		{PoolIndex: 0, DiskIndex: 1, TotalSpace: 100, AvailableSpace: 60}, // parity drive
		{PoolIndex: 0, DiskIndex: 2, TotalSpace: 50, AvailableSpace: 50},
	}
	si := madmin.StorageInfo{Backend: madmin.BackendInfo{
		StandardSCData:   []int{2},
		StandardSCParity: 1,
	}}
	if got := GetTotalCapacity(disks); got != 250 {
		t.Errorf("GetTotalCapacity = %d, want 250", got)
	}
	if got := GetTotalCapacityFree(disks); got != 150 {
		t.Errorf("GetTotalCapacityFree = %d, want 150", got)
	}
	// data drives are indexes < StandardSCData[0]=2 -> usable total 200, free 100
	if got := GetTotalUsableCapacity(disks, si); got != 200 {
		t.Errorf("GetTotalUsableCapacity = %d, want 200", got)
	}
	if got := GetTotalUsableCapacityFree(disks, si); got != 100 {
		t.Errorf("GetTotalUsableCapacityFree = %d, want 100", got)
	}
}

func TestCollectorPaths(t *testing.T) {
	t.Parallel()
	srv := NewV3Server(func(h http.Handler) http.Handler { return h }, testDeps())
	if len(srv.CollectorPaths()) == 0 {
		t.Fatal("expected collector paths")
	}
}
