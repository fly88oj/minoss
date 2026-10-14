// Copyright (c) 2015-2021 MinIO, Inc.
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
	"net/http"

	"github.com/minio/minio/internal/auth"
	"github.com/minio/minio/internal/mcontext"
	"github.com/minio/pkg/v3/policy"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	httpRequestsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "s3_ttfb_seconds",
			Help:    "Time taken by requests served by current MinIO server instance",
			Buckets: []float64{.05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"api"},
	)
	bucketHTTPRequestsDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "s3_ttfb_seconds",
			Help:    "Time taken by requests served by current MinIO server instance per bucket",
			Buckets: []float64{.05, .1, .25, .5, 1, 2.5, 5, 10},
		},
		[]string{"api", "bucket"},
	)
	minioVersionInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Namespace: "minio",
			Name:      "version_info",
			Help:      "Version of current MinIO server instance",
		},
		[]string{
			// current version
			"version",
			// commit-id of the current version
			"commit",
		},
	)
)

const (
	healMetricsNamespace = "self_heal"
	cacheNamespace       = "cache"
	s3Namespace          = "s3"
	bucketNamespace      = "bucket"
	minioNamespace       = "minio"
	diskNamespace        = "disk"
	interNodeNamespace   = "internode"
)

func init() {
	prometheus.MustRegister(httpRequestsDuration)
	prometheus.MustRegister(minioVersionInfo)
}

// NoAuthMiddleware no auth middle ware.
func NoAuthMiddleware(h http.Handler) http.Handler {
	return h
}

// AuthMiddleware checks if the bearer token is valid and authorized.
func AuthMiddleware(h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tc, ok := r.Context().Value(mcontext.ContextTraceKey).(*mcontext.TraceCtxt)

		claims, groups, owner, authErr := metricsRequestAuthenticate(r)
		if authErr != nil || (claims != nil && !claims.VerifyIssuer("prometheus", true)) {
			if ok {
				tc.FuncName = "handler.MetricsAuth"
				tc.ResponseRecorder.LogErrBody = true
			}

			writeErrorResponseJSON(r.Context(), w, toAdminAPIErr(r.Context(), errAuthentication), r.URL)
			return
		}

		cred := auth.Credentials{
			AccessKey: claims.AccessKey,
			Claims:    claims.Map(),
			Groups:    groups,
		}

		// For authenticated users apply IAM policy.
		if !globalIAMSys.IsAllowed(policy.Args{
			AccountName:     cred.AccessKey,
			Groups:          cred.Groups,
			Action:          policy.PrometheusAdminAction,
			ConditionValues: getConditionValues(r, "", cred),
			IsOwner:         owner,
			Claims:          cred.Claims,
		}) {
			if ok {
				tc.FuncName = "handler.MetricsAuth"
				tc.ResponseRecorder.LogErrBody = true
			}

			writeErrorResponseJSON(r.Context(), w, toAdminAPIErr(r.Context(), errAuthentication), r.URL)
			return
		}
		h.ServeHTTP(w, r)
	})
}
