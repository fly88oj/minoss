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

// Claim keys of the tokens this server issues and accepts. They are a wire
// contract: the STS handlers write them, IAM reads them when it resolves a
// policy for an assumed identity, the S3 policy evaluation reads them, and the
// FTP/SFTP adapters pass them through to the object layer. A name that drifts
// between the writer and the reader silently drops the policy claim, which
// denies (or, on a collision, mis-resolves) access - so the names the server
// itself defines live here, once.
//
// The JWT library in internal/jwt reads the registered claims ("exp",
// "iat", "nbf") and "accessKey" through its own struct tags and MapClaims
// lookups; it cannot import this package (internal/auth depends on
// internal/jwt), and that is the library's format handling, not a second
// definition of these names.
const (
	// ClaimExpiry is the standard JWT expiration time claim.
	ClaimExpiry = "exp"
	// ClaimSubject is the standard JWT subject claim.
	ClaimSubject = "sub"
	// ClaimAudience is the standard JWT audience claim.
	ClaimAudience = "aud"
	// ClaimIssuer is the standard JWT issuer claim.
	ClaimIssuer = "iss"

	// ClaimAccessKey carries the access key of the credential a token was
	// issued for. The JWT layer in internal/jwt reads it by name; it cannot
	// import this package (internal/auth depends on internal/jwt).
	ClaimAccessKey = "accessKey"

	// ClaimParent is the parent user of a service account, an assumed-role
	// session or an LDAP/OIDC-derived credential.
	ClaimParent = "parent"

	// ClaimRoleARN is the role an AssumeRole request resolved to.
	ClaimRoleARN = "roleArn"

	// ClaimTokenRevokeType is the revoke scope requested for a token.
	ClaimTokenRevokeType = "tokenRevokeType"

	// ClaimServiceAccountPolicy selects how a service account gets its
	// policy; its value is one of EmbeddedPolicy or InheritedPolicy.
	ClaimServiceAccountPolicy = "sa-policy"

	// EmbeddedPolicy is a service-account policy supplied with the request.
	EmbeddedPolicy = "embedded-policy"

	// InheritedPolicy is a service-account policy inherited from its parent
	// user.
	InheritedPolicy = "inherited-policy"

	// ClaimLDAPUserDN is the normalized LDAP distinguished name of the user.
	ClaimLDAPUserDN = "ldapUser"

	// ClaimLDAPActualUserDN is the LDAP distinguished name as entered.
	ClaimLDAPActualUserDN = "ldapActualUser"

	// ClaimLDAPUsername is the short/login LDAP username.
	ClaimLDAPUsername = "ldapUsername"

	// ClaimLDAPAttributePrefix prefixes claims carrying LDAP user attributes.
	ClaimLDAPAttributePrefix = "ldapAttrib_"
)
