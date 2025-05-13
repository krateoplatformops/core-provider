package certs

import (
	"crypto/x509"
	"crypto/x509/pkix"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"encoding/asn1"
	"encoding/base64"
	"encoding/pem"

	kube "github.com/krateoplatformops/plumbing/certs"
	certv1 "k8s.io/api/certificates/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
)

type GenerateClientCertAndKeyOpts struct {
	Duration              time.Duration
	LeaseExpirationMargin time.Duration
	Username              string
	Approver              string
}

func GenerateClientCertAndKey(client kubernetes.Interface, log func(msg string, keysAndValues ...any), o GenerateClientCertAndKeyOpts) (string, string, error) {
	key, err := kube.NewPrivateKey()
	if err != nil {
		return "", "", err
	}

	// Define the SAN extension
	sanExtension := pkix.Extension{
		Id:       []int{2, 5, 29, 17}, // OID for Subject Alternative Name
		Critical: false,
		Value:    []byte{},
	}

	// Add DNS names to the SAN extension
	dnsNames := []string{o.Username}
	rawValues := []asn1.RawValue{}
	for _, dnsName := range dnsNames {
		rawValues = append(rawValues, asn1.RawValue{
			Tag:   2, // DNSName
			Class: asn1.ClassContextSpecific,
			Bytes: []byte(dnsName),
		})
	}
	sanExtension.Value, _ = asn1.Marshal(rawValues)

	opts := kube.CertificateRequestOptions{
		Key:      key,
		Username: "system:node:" + o.Username,
		Groups:   []string{"system:nodes"},
		ExtraExtensions: []pkix.Extension{
			sanExtension,
		},
	}
	req, err := kube.NewCertificateRequest(opts)
	if err != nil {
		return "", "", err
	}

	reqOpts := kube.CertificateSigningRequestOptions{
		Username:   o.Username,
		Duration:   o.Duration,
		CSR:        req,
		SignerName: certv1.KubeletServingSignerName,
		Usages:     []string{string(certv1.UsageServerAuth), string(certv1.UsageKeyEncipherment), string(certv1.UsageDigitalSignature)},
	}

	// csr object from csr bytes
	csr := kube.NewCertificateSigningRequest(reqOpts)

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
	err = kube.ApproveCertificateSigningRequest(client, csr, o.Approver)
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

func CheckOrRegenerateClientCertAndKey(client kubernetes.Interface, log func(msg string, keysAndValues ...any), o GenerateClientCertAndKeyOpts) (bool, string, string, error) {
	csr, err := kube.GetCertificateSigningRequest(client, o.Username)
	if err != nil {
		if errors.IsNotFound(err) {
			log("Certificate signing request not found, creating", "crs.name", o.Username)
			crtStr, keyStr, err := GenerateClientCertAndKey(client, log, o)
			return false, crtStr, keyStr, err
		}
		return false, "", "", fmt.Errorf("getting CSR kubernetes object: %w", err)
	}
	if ok := Expired(csr, o.LeaseExpirationMargin); ok {
		log("Certificate signing request is expired, recreating", "crs.name", csr.Name)
		err = kube.DeleteCertificateSigningRequest(client, csr.Name)
		if err != nil {
			return false, "", "", fmt.Errorf("deleting CSR kubernetes object: %w", err)
		}
		crtStr, keyStr, err := GenerateClientCertAndKey(client, log, o)
		return false, crtStr, keyStr, err
	}
	log("Certificate signing request is not expired", "crs.name", csr.Name)

	return true, "", "", nil
}

func Expired(csr *certv1.CertificateSigningRequest, leaseExpirationMargin time.Duration) bool {
	expiration := leaseExpirationMargin
	// check creationTimestamp of the csr
	creationTimestamp := csr.CreationTimestamp.Time
	// check if the certificate is expired
	expiration = expiration - time.Since(creationTimestamp)
	return expiration < 0
}

func UpdateCerts(crt string, key string, certsPath string) error {
	decCert, err := base64.StdEncoding.DecodeString(crt)
	if err != nil {
		return fmt.Errorf("cannot decode certificate: %v", err)
	}
	decKey, err := base64.StdEncoding.DecodeString(key)
	if err != nil {
		return fmt.Errorf("cannot decode key: %v", err)
	}

	// Create the directory if it doesn't exist
	err = os.MkdirAll(certsPath, 0755)
	if err != nil {
		return fmt.Errorf("cannot create directory for certificate and key: %v", err)
	}
	// Write the certificate and key to a files
	f, err := os.Create(filepath.Join(certsPath, "tls.crt"))
	if err != nil {
		return fmt.Errorf("cannot create certificate file: %v", err)
	}
	defer f.Close()
	_, err = f.Write(decCert)
	if err != nil {
		return fmt.Errorf("cannot write certificate file: %v", err)
	}
	f, err = os.Create(filepath.Join(certsPath, "tls.key"))
	if err != nil {
		return fmt.Errorf("cannot create key file: %v", err)
	}
	defer f.Close()
	_, err = f.Write(decKey)
	if err != nil {
		return fmt.Errorf("cannot write key file: %v", err)
	}
	return nil
}
