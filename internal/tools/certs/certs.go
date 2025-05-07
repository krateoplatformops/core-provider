package certs

import (
	"crypto/x509"
	"fmt"
	"time"

	"encoding/base64"
	"encoding/pem"

	kube "github.com/krateoplatformops/core-provider/internal/tools/kube/certs"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

type GenerateClientCertAndKeyOpts struct {
	Duration time.Duration
	UserID   string
	Username string
	Groups   []string
}

func GenerateClientCertAndKey(client kubernetes.Interface, log func(msg string, keysAndValues ...any), o GenerateClientCertAndKeyOpts) (string, string, error) {
	key, err := kube.NewPrivateKey()
	if err != nil {
		return "", "", err
	}

	req, err := kube.NewCertificateRequest(key, o.Username, o.Groups)
	if err != nil {
		return "", "", err
	}

	// csr object from csr bytes
	csr := kube.NewCertificateSigningRequest(req, o.Duration, o.UserID, o.Username)

	// create kubernetes csr object
	err = kube.CreateCertificateSigningRequests(client, csr)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			return "", "", fmt.Errorf("creating CSR kubernetes object: %w", err)
		}

		// if the csr already exists, we need to delete it and create a new one
		err = kube.DeleteCertificateSigningRequest(client, csr.Name)
		if err != nil {
			return "", "", fmt.Errorf("deleting CSR kubernetes object: %w", err)
		}
		err = kube.CreateCertificateSigningRequests(client, csr)
		if err != nil {
			return "", "", fmt.Errorf("creating CSR kubernetes object: %w", err)
		}

		log("Certificate signing request already exists, recreating", "crs.name", csr.Name)
	}
	log("Certificate signing request created", "crs.name", csr.Name)

	// approve the csr
	err = kube.ApproveCertificateSigningRequest(client, csr)
	if err != nil {
		return "", "", err
	}
	log("Certificate signing request approved", "crs.name", csr.Name)

	// wait for certificate
	log("Waiting for certificate", "crs.name", csr.Name)
	err = kube.WaitForCertificate(client, csr.Name)
	if err != nil {
		return "", "", err
	}

	crt, err := kube.Certificate(client, csr.Name)
	if err != nil {
		return "", "", err
	}
	log("Certificate acquired", "crs.name", csr.Name)

	crtStr := base64.StdEncoding.EncodeToString(crt)
	keyStr := base64.StdEncoding.EncodeToString(pem.EncodeToMemory(&pem.Block{
		Type:  "RSA PRIVATE KEY",
		Bytes: x509.MarshalPKCS1PrivateKey(key),
	}))
	return crtStr, keyStr, nil
}
