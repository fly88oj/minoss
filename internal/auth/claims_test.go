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

package auth

import "testing"

// The claim names and service-account policy values are wire data: they are
// what appears inside tokens and JSON documents that are already stored or in
// flight. Renaming a Go constant is safe; changing its value silently breaks
// every credential issued before the change, and nothing else fails. So the
// values are pinned here, in the one place they are allowed to be spelled.
func TestClaimWireValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		got   string
		value string
	}{
		{"ClaimExpiry", ClaimExpiry, "exp"},
		{"ClaimSubject", ClaimSubject, "sub"},
		{"ClaimAudience", ClaimAudience, "aud"},
		{"ClaimIssuer", ClaimIssuer, "iss"},
		{"ClaimParent", ClaimParent, "parent"},
		{"ClaimRoleARN", ClaimRoleARN, "roleArn"},
		{"ClaimTokenRevokeType", ClaimTokenRevokeType, "tokenRevokeType"},
		{"ClaimServiceAccountPolicy", ClaimServiceAccountPolicy, "sa-policy"},
		{"EmbeddedPolicy", EmbeddedPolicy, "embedded-policy"},
		{"InheritedPolicy", InheritedPolicy, "inherited-policy"},
		{"ClaimLDAPUserDN", ClaimLDAPUserDN, "ldapUser"},
		{"ClaimLDAPActualUserDN", ClaimLDAPActualUserDN, "ldapActualUser"},
		{"ClaimLDAPUsername", ClaimLDAPUsername, "ldapUsername"},
		{"ClaimLDAPAttributePrefix", ClaimLDAPAttributePrefix, "ldapAttrib_"},
		{"ClaimAccessKey", ClaimAccessKey, "accessKey"},
	}

	seen := make(map[string]string, len(tests))
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if tc.got != tc.value {
				t.Errorf("%s wire value changed: have %q, want %q", tc.name, tc.got, tc.value)
			}
			if other, dup := seen[tc.got]; dup {
				t.Errorf("%s and %s share the wire value %q", other, tc.name, tc.got)
			}
			seen[tc.got] = tc.name
		})
	}
}
