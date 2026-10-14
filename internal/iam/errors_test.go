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

package iam

import (
	"testing"
)

// sentinels is every error this package declares. Callers compare these by
// identity, so a copy-paste that points two names at one message would merge
// two distinct S3 or admin responses with no compile error.
//
// The mapping half of that hazard is pinned where the mapping lives, not here:
// cmd/api-errors_test.go for toAPIErrorCode and
// cmd/admin-handler-utils_test.go for toAdminAPIErr. Keeping a sentinel out of
// those tables is still possible.
var sentinels = []error{
	ErrNoSuchUser,
	ErrNoSuchUserLDAPWarn,
	ErrNoSuchServiceAccount,
	ErrNoSuchTempAccount,
	ErrNoSuchAccessKey,
	ErrNoSuchAccount,
	ErrNoSuchGroup,
	ErrGroupNotEmpty,
	ErrGroupDisabled,
	ErrGroupNameContainsReservedChars,
	ErrNoSuchPolicy,
	ErrPolicyInUse,
	ErrNoPolicyToAttachOrDetach,
	ErrTooManyPolicies,
	ErrIAMActionNotAllowed,
	ErrIAMServiceAccountNotAllowed,
	ErrIAMNotInitialized,
	ErrSessionPolicyTooLarge,
}

func TestSentinelsAreDistinct(t *testing.T) {
	t.Parallel()

	seen := make(map[string]int, len(sentinels))
	for _, err := range sentinels {
		seen[err.Error()]++
	}
	for msg, n := range seen {
		if n > 1 {
			t.Errorf("%d sentinels share the message %q", n, msg)
		}
	}
}
