package kube

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

func TestCreateCertificateSigningRequests(t *testing.T) {
	username := "pippo"
	groups := []string{"devs"}

	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatal(err)
	}

	restCfg, err := clientcmd.BuildConfigFromFlags("", filepath.Join(home, ".kube", "config"))
	if err != nil {
		t.Fatal(err)
	}

	client, err := kubernetes.NewForConfig(restCfg)
	if err != nil {
		t.Fatal(err)
	}

	exists, err := CertificateExistsFunc(client, username)(context.TODO())
	if err != nil {
		if !errors.IsNotFound(err) {
			t.Fatal(err)
		}
	}

	if exists {
		fmt.Println("exists: ", exists)
		return
	}

	key, err := NewPrivateKey()
	if err != nil {
		t.Fatal(err)
	}

	req, err := NewCertificateRequest(key, username, groups)
	if err != nil {
		t.Fatal(err)
	}

	// csr object from csr bytes
	csr := NewCertificateSigningRequest(req, time.Hour*1, "12345", username)

	// create kubernetes csr object
	err = CreateCertificateSigningRequests(client, csr)
	if err != nil {
		if !errors.IsAlreadyExists(err) {
			t.Fatal(err)
		}

		t.Logf("csr already exists")
		if err := DeleteCertificateSigningRequest(client, csr.Name); err != nil {
			t.Fatal(err)
		}
	}

	err = ApproveCertificateSigningRequest(client, csr)
	if err != nil {
		t.Fatal(err)
	}

}
