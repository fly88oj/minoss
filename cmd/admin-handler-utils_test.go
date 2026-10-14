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

package cmd

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/minio/minio/internal/iam"
)

// toAdminAPIErr maps the IAM sentinels that live in internal/iam by identity.
// Dropping one of those cases, or redeclaring a value somewhere else, turns a
// 4xx/403/503 into the generic internal error with no compile error - so the
// mapping is pinned here the same way cmd/api-errors_test.go pins the S3 one.
func TestToAdminAPIErrIAMSentinels(t *testing.T) {
	t.Parallel()

	ctx := t.Context()
	tests := []struct {
		err     error
		code    string
		status  int
		wrapped bool
	}{
		{iam.ErrTooManyPolicies, "XMinioAdminInvalidRequest", http.StatusBadRequest, false},
		{iam.ErrIAMActionNotAllowed, "XMinioIAMActionNotAllowed", http.StatusForbidden, false},
		{iam.ErrIAMServiceAccountNotAllowed, "XMinioIAMServiceAccountNotAllowed", http.StatusBadRequest, false},
		{iam.ErrIAMNotInitialized, "XMinioIAMNotInitialized", http.StatusServiceUnavailable, false},
		{iam.ErrPolicyInUse, "XMinioIAMPolicyInUse", http.StatusBadRequest, false},
		{iam.ErrSessionPolicyTooLarge, "XMinioIAMServiceAccountSessionPolicyTooLarge", http.StatusBadRequest, false},
		// These two had an API error code defined or implied but no mapping, so
		// they reached clients as a generic 500.
		{iam.ErrGroupDisabled, "XMinioAdminGroupDisabled", http.StatusBadRequest, false},
		{iam.ErrNoSuchTempAccount, "XMinioAdminNoSuchAccessKey", http.StatusNotFound, false},
		// Admin handlers wrap these with context; the mapping unwraps.
		{fmt.Errorf("loading policy: %w", iam.ErrIAMActionNotAllowed), "XMinioIAMActionNotAllowed", http.StatusForbidden, true},
	}

	for _, tc := range tests {
		name := tc.code
		if tc.wrapped {
			name += " (wrapped)"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			apiErr := toAdminAPIErr(ctx, tc.err)
			if apiErr.Code != tc.code {
				t.Errorf("expected code %q, got %q", tc.code, apiErr.Code)
			}
			if apiErr.HTTPStatusCode != tc.status {
				t.Errorf("expected status %d, got %d", tc.status, apiErr.HTTPStatusCode)
			}
		})
	}
}
