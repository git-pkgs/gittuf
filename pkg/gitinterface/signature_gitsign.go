// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

//go:build gitsign

package gitinterface

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/sigstore/cosign/v3/pkg/cosign"
	gitsignVerifier "github.com/sigstore/gitsign/pkg/git"
	gitsignRekor "github.com/sigstore/gitsign/pkg/rekor"
	"github.com/sigstore/sigstore/pkg/fulcioroots"
)

const rekorPublicGoodInstance = "https://rekor.sigstore.dev"

// verifyGitsignSignature handles the Sigstore-specific workflow involved in
// verifying commit or tag signatures issued by gitsign.
func verifyGitsignSignature(ctx context.Context, repo *Repository, key *signerverifier.SSLibKey, data, signature []byte) error {
	checkOpts := &cosign.CheckOpts{
		Identities: []cosign.Identity{{
			Issuer:  key.KeyVal.Issuer,
			Subject: key.KeyVal.Identity,
		}},
	}

	var verifier *gitsignVerifier.CertVerifier
	sigstoreRootFilePath := os.Getenv(sigstore.EnvSigstoreRootFile)
	if sigstoreRootFilePath == "" {
		root, err := fulcioroots.Get()
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
		intermediate, err := fulcioroots.GetIntermediates()
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}

		checkOpts.RootCerts = root
		checkOpts.IntermediateCerts = intermediate

		verifier, err = gitsignVerifier.NewCertVerifier(
			gitsignVerifier.WithRootPool(root),
			gitsignVerifier.WithIntermediatePool(intermediate),
		)
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
	} else {
		slog.Debug("Using environment variables to establish trust for Sigstore instance...")
		rootCerts, err := common.LoadCertsFromPath(sigstoreRootFilePath)
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
		root := x509.NewCertPool()
		for _, cert := range rootCerts {
			root.AddCert(cert)
		}

		checkOpts.RootCerts = root

		verifier, err = gitsignVerifier.NewCertVerifier(
			gitsignVerifier.WithRootPool(root),
		)
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
	}

	verifiedCert, err := verifier.Verify(ctx, data, signature, true)
	if err != nil {
		return ErrIncorrectVerificationKey
	}

	rekorURL := rekorPublicGoodInstance
	// Check git config to see if rekor server must be overridden
	config, err := repo.GetGitConfig()
	if err != nil {
		return errors.Join(ErrVerifyingSigstoreSignature, err)
	}
	if configValue, has := config[sigstore.GitConfigRekor]; has {
		slog.Debug(fmt.Sprintf("Using '%s' as Rekor instance...", configValue))
		rekorURL = configValue
	}

	// gitsignRekor.NewWithOptions invokes cosign.GetRekorPubs which looks at
	// the env var, so we don't have to do anything here
	rekor, err := gitsignRekor.NewWithOptions(ctx, rekorURL)
	if err != nil {
		return errors.Join(ErrVerifyingSigstoreSignature, err)
	}

	checkOpts.RekorClient = rekor.Rekor
	checkOpts.RekorPubKeys = rekor.PublicKeys()

	// cosign.GetCTLogPubs already looks at the env var, so we don't have to do
	// anything here
	ctPub, err := cosign.GetCTLogPubs(ctx)
	if err != nil {
		return errors.Join(ErrVerifyingSigstoreSignature, err)
	}

	checkOpts.CTLogPubKeys = ctPub

	if _, err := cosign.ValidateAndUnpackCert(verifiedCert, checkOpts); err != nil {
		return errors.Join(ErrIncorrectVerificationKey, err)
	}

	return nil
}
