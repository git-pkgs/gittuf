// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

//go:build !gitsign

package gitinterface

import (
	"context"
	"errors"

	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

// verifyGitsignSignature is unavailable in builds without the gitsign tag.
// The gitsign dependency transitively pulls in go-git/v5; embedders that want
// a single go-git in the binary can build without the tag and accept that
// sigstore-signed commits won't verify.
func verifyGitsignSignature(_ context.Context, _ *Repository, _ *signerverifier.SSLibKey, _, _ []byte) error {
	return errors.Join(ErrVerifyingSigstoreSignature,
		errors.New("gitsign verification not available: rebuild with -tags gitsign"))
}
