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

// Package iam holds the parts of the IAM subsystem that have left package cmd.
//
// It is deliberately small today: only the error sentinels the admin, STS,
// site-replication and S3 error-mapping paths compare by identity. The
// implementation - IAMSys, the store manager and the object/etcd stores - is
// still in cmd/iam*.go, and docs/fork/backlog.md item 2 records what blocks
// moving it (cmd reads IAM through the unexported IAMSys.store field from 29
// sites, and ObjectLayer cannot be implemented outside cmd). Treat this package
// as the destination, not as the whole subsystem.
package iam

import "errors"

// Errors of the IAM subsystem. Callers - the admin API handlers, the STS
// handlers, the S3 policy evaluation and site replication - compare these by
// identity, and cmd/api-errors.go maps them to S3 error codes, so each one is
// declared exactly once.
var (
	// ErrNoSuchUser is returned when a user does not exist.
	ErrNoSuchUser = errors.New("Specified user does not exist")

	// ErrNoSuchUserLDAPWarn is returned when a built-in IDP command is used
	// where the LDAP one was probably meant.
	ErrNoSuchUserLDAPWarn = errors.New("Specified user does not exist. If you meant a user in LDAP please use command under `mc idp ldap`")

	// ErrNoSuchServiceAccount is returned when a service account does not
	// exist.
	ErrNoSuchServiceAccount = errors.New("Specified service account does not exist")

	// ErrNoSuchTempAccount is returned when a temporary account does not
	// exist.
	ErrNoSuchTempAccount = errors.New("Specified temporary account does not exist")

	// ErrNoSuchAccessKey is returned when an access key does not exist.
	ErrNoSuchAccessKey = errors.New("Specified access key does not exist")

	// ErrNoSuchAccount is returned when an account (user or service account)
	// does not exist.
	ErrNoSuchAccount = errors.New("Specified account does not exist")

	// ErrNoSuchGroup is returned when a group does not exist.
	ErrNoSuchGroup = errors.New("Specified group does not exist")

	// ErrGroupNotEmpty is returned when a non-empty group is being deleted.
	ErrGroupNotEmpty = errors.New("Specified group is not empty - cannot remove it")

	// ErrGroupDisabled is returned when a group is disabled.
	ErrGroupDisabled = errors.New("Specified group is disabled")

	// ErrGroupNameContainsReservedChars is returned when a group name holds
	// '=' or ','.
	ErrGroupNameContainsReservedChars = errors.New("Group name contains reserved characters '=' or ','")

	// ErrNoSuchPolicy is returned when a canned policy does not exist.
	ErrNoSuchPolicy = errors.New("Specified canned policy does not exist")

	// ErrPolicyInUse is returned when a policy to be deleted is still
	// attached.
	ErrPolicyInUse = errors.New("Specified policy is in use and cannot be deleted.")

	// ErrNoPolicyToAttachOrDetach is returned when a policy update has no net
	// effect, i.e. it is already applied.
	ErrNoPolicyToAttachOrDetach = errors.New("Specified policy update has no net effect")

	// ErrTooManyPolicies is returned when more than one policy is specified
	// where only one is expected.
	ErrTooManyPolicies = errors.New("Only a single policy may be specified here.")

	// ErrIAMActionNotAllowed is returned for an IAM action that the
	// configuration forbids.
	ErrIAMActionNotAllowed = errors.New("Specified IAM action is not allowed")

	// ErrIAMServiceAccountNotAllowed is returned for a service-account action
	// the configuration forbids.
	ErrIAMServiceAccountNotAllowed = errors.New("Specified service account action is not allowed")

	// ErrIAMNotInitialized is returned while the IAM subsystem is still
	// loading its configuration.
	ErrIAMNotInitialized = errors.New("IAM sub-system is being initialized, please try again")

	// ErrSessionPolicyTooLarge is returned for a session policy over the
	// supported size.
	ErrSessionPolicyTooLarge = errors.New("Session policy should not exceed 2048 characters")
)
