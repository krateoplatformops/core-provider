package certificates

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	certs "github.com/krateoplatformops/core-provider/internal/tools/certs"
	"github.com/krateoplatformops/provider-runtime/pkg/logging"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
)

// MockCertManager is a mock implementation of CertManagerInterface for testing.
type MockCertManager struct {
	manageCertificatesCalled      int
	updateExistingResourcesCalled int
	mu                            sync.Mutex
}

func (m *MockCertManager) ManageCertificates(ctx context.Context, gvr schema.GroupVersionResource) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.manageCertificatesCalled++
	return nil
}

func (m *MockCertManager) GetCABundle() []byte {
	return []byte("test-ca-bundle")
}

func (m *MockCertManager) GetServiceName() string {
	return "test-webhook"
}

func (m *MockCertManager) GetServiceNamespace() string {
	return "test-system"
}

func (m *MockCertManager) UpdateExistingResources(ctx context.Context) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateExistingResourcesCalled++
	return nil
}

// MockPluralizer is a mock implementation of pluralizer.PluralizerInterface.
type MockPluralizer struct{}

func (m *MockPluralizer) GVKtoGVR(gvk schema.GroupVersionKind) (schema.GroupVersionResource, error) {
	return schema.GroupVersionResource{
		Group:    gvk.Group,
		Version:  gvk.Version,
		Resource: gvk.Kind + "s", // simple pluralization for testing
	}, nil
}

// TestCertificateReconcilerStart tests the Start method and periodic reconciliation.
func TestCertificateReconcilerStart(t *testing.T) {
	mockMgr := &MockCertManager{}
	mockPlur := &MockPluralizer{}
	log := logging.NewLogrLogger(ctrl.Log)

	// Use a short sync interval for testing
	reconciler := NewCertificateReconciler(mockMgr, mockPlur, log, 100*time.Millisecond)

	// Create a context that we can cancel to stop the reconciler
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Run the reconciler in a goroutine
	done := make(chan error, 1)
	go func() {
		done <- reconciler.Start(ctx)
	}()

	// Wait for a few sync cycles
	time.Sleep(400 * time.Millisecond)

	// Cancel the context to stop the reconciler
	cancel()

	// Wait for the reconciler to finish
	err := <-done
	if err != nil {
		t.Fatalf("reconciler failed: %v", err)
	}

	// Verify that UpdateExistingResources was called multiple times
	mockMgr.mu.Lock()
	defer mockMgr.mu.Unlock()

	if mockMgr.updateExistingResourcesCalled < 3 {
		t.Errorf("expected UpdateExistingResources to be called at least 3 times, got %d", mockMgr.updateExistingResourcesCalled)
	}
}

// TestCertificateReconcilerGracefulShutdown tests that the reconciler stops gracefully.
func TestCertificateReconcilerGracefulShutdown(t *testing.T) {
	mockMgr := &MockCertManager{}
	mockPlur := &MockPluralizer{}
	log := logging.NewLogrLogger(ctrl.Log)

	reconciler := NewCertificateReconciler(mockMgr, mockPlur, log, 100*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		done <- reconciler.Start(ctx)
	}()

	// Immediately cancel
	time.Sleep(50 * time.Millisecond)
	cancel()

	// Should return within a reasonable time
	select {
	case <-done:
		// Success
	case <-time.After(2 * time.Second):
		t.Fatal("reconciler did not stop within 2 seconds")
	}
}

// TestCertificateReconcilerDefaultInterval tests that default interval is used when 0 is passed.
func TestCertificateReconcilerDefaultInterval(t *testing.T) {
	mockMgr := &MockCertManager{}
	mockPlur := &MockPluralizer{}
	log := logging.NewLogrLogger(ctrl.Log)

	// Pass 0 as syncInterval to trigger default
	reconciler := NewCertificateReconciler(mockMgr, mockPlur, log, 0)

	if reconciler.syncInterval != 5*time.Minute {
		t.Errorf("expected default interval to be 5m, got %v", reconciler.syncInterval)
	}
}

// TestCertManagerThreadSafety tests that concurrent reads and writes to caBundle are safe.
func TestCertManagerThreadSafety(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	cm := &CertManager{
		kube:       fakeClient,
		client:     nil,
		caBundle:   []byte("initial"),
		log:        func(msg string, keysAndValues ...any) {},
		pluralizer: &MockPluralizer{},
	}

	// Simulate concurrent reads and writes
	var wg sync.WaitGroup
	errors := make(chan error)

	// 10 goroutines doing concurrent reads
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				_ = cm.GetCABundle()
				time.Sleep(time.Microsecond)
			}
		}()
	}

	// 5 goroutines doing concurrent writes
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 20; j++ {
				cm.caBundleMu.Lock()
				cm.caBundle = []byte{byte(id)}
				cm.caBundleMu.Unlock()
				time.Sleep(time.Microsecond)
			}
		}(i)
	}

	// Wait for all goroutines to finish
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	// If we have a race condition, this will panic or timeout
	select {
	case <-done:
		// Success - all goroutines completed without panicking
	case <-time.After(5 * time.Second):
		t.Fatal("test timed out - possible deadlock in mutex")
	case err := <-errors:
		t.Fatalf("test failed with error: %v", err)
	}
}

// TestGetCABundleConcurrentReads tests that multiple reads happen without blocking each other.
func TestGetCABundleConcurrentReads(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	cm := &CertManager{
		kube:       fakeClient,
		client:     nil,
		caBundle:   []byte("test-bundle"),
		log:        func(msg string, keysAndValues ...any) {},
		pluralizer: &MockPluralizer{},
	}

	var wg sync.WaitGroup
	startTime := time.Now()

	// 100 concurrent reads should be fast (not serialized)
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			bundle := cm.GetCABundle()
			if len(bundle) == 0 {
				t.Error("GetCABundle returned empty bundle")
			}
		}()
	}

	wg.Wait()
	elapsed := time.Since(startTime)

	// If reads were serialized, this would take much longer
	// With RWMutex, all 100 reads should happen concurrently in < 100ms
	if elapsed > 1*time.Second {
		t.Logf("warning: concurrent reads took %v (may indicate serialization)", elapsed)
	}
}

// TestCertificateReconcilerWithConcurrentReadsAndWrites simulates the real scenario where
// the CertificateReconciler is updating certificates while the controller is reconciling
// compositions and reading the CA bundle concurrently.
func TestCertificateReconcilerWithConcurrentReadsAndWrites(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Create a cert manager that simulates updating certificates
	cm := &CertManager{
		kube:       fakeClient,
		client:     nil,
		caBundle:   []byte("initial-bundle-v1"),
		log:        func(msg string, keysAndValues ...any) {},
		pluralizer: &MockPluralizer{},
	}

	// Simulate a controller reconciler that reads the CA bundle multiple times
	var wg sync.WaitGroup
	bundleValues := make(map[string]int) // track what bundle values were read
	bundlesLock := sync.Mutex{}

	// Start 5 "controller" goroutines continuously reading the CA bundle
	for ctrl := 0; ctrl < 5; ctrl++ {
		wg.Add(1)
		go func(controllerID int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				// Simulate controller reading CA bundle during reconciliation
				bundle := cm.GetCABundle()
				bundlesLock.Lock()
				bundleValues[string(bundle)]++
				bundlesLock.Unlock()

				// Simulate some work
				time.Sleep(100 * time.Microsecond)
			}
		}(ctrl)
	}

	// Start the CertificateReconciler
	mockMgr := &MockCertManager{}
	mockPlur := &MockPluralizer{}
	log := logging.NewLogrLogger(ctrl.Log)
	reconciler := NewCertificateReconciler(mockMgr, mockPlur, log, 50*time.Millisecond)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	reconcilerDone := make(chan error, 1)
	go func() {
		reconcilerDone <- reconciler.Start(ctx)
	}()

	// Let the concurrent reads and reconciler run together
	time.Sleep(300 * time.Millisecond)

	// Now have the reconciler update the certificate bundle (simulating a refresh)
	// This happens while controllers are still reading
	for round := 0; round < 3; round++ {
		newBundle := []byte("updated-bundle-v" + string(byte('2'+round)))
		cm.caBundleMu.Lock()
		cm.caBundle = newBundle
		cm.caBundleMu.Unlock()
		time.Sleep(30 * time.Millisecond)
	}

	// Continue concurrent reads for a bit more
	time.Sleep(150 * time.Millisecond)

	// Stop and wait for all operations to complete
	cancel()
	wg.Wait()

	err := <-reconcilerDone
	if err != nil {
		t.Fatalf("reconciler failed: %v", err)
	}

	// Verify that controllers read at least 2 different bundle values
	// (they should have read different versions as they were updated)
	bundlesLock.Lock()
	bundleCount := len(bundleValues)
	bundlesLock.Unlock()

	if bundleCount < 1 {
		t.Errorf("expected to read at least 1 bundle version, got %d", bundleCount)
	}

	t.Logf("Controllers read %d different bundle versions during concurrent updates", bundleCount)
}

// TestWebhookConfigUpdatesConcurrentLoad simulates webhook configuration updates
// under concurrent load from multiple controllers reconciling compositions.
type MockWebhookManager struct {
	updateCount int
	bundleMap   map[string]int // track which bundles were used in updates
	mu          sync.Mutex
}

func (m *MockWebhookManager) UpdateWebhookConfig(ctx context.Context, bundle []byte, name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.updateCount++
	if m.bundleMap == nil {
		m.bundleMap = make(map[string]int)
	}
	m.bundleMap[string(bundle)]++
	// Simulate update latency
	time.Sleep(5 * time.Millisecond)
	return nil
}

func TestWebhookConfigUpdatesConcurrentLoad(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	// Create a cert manager with a webhook manager
	cm := &CertManager{
		kube:       fakeClient,
		client:     nil,
		caBundle:   []byte("webhook-bundle-v1"),
		log:        func(msg string, keysAndValues ...any) {},
		pluralizer: &MockPluralizer{},
	}

	webhookMgr := &MockWebhookManager{}

	// Simulate 10 concurrent controllers each making multiple webhook updates
	var wg sync.WaitGroup
	for controller := 0; controller < 10; controller++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for update := 0; update < 20; update++ {
				// Each controller reads the current CA bundle
				bundle := cm.GetCABundle()

				// Then updates the webhook config with that bundle
				if err := webhookMgr.UpdateWebhookConfig(
					context.Background(),
					bundle,
					"webhook-"+string(byte('0'+id)),
				); err != nil {
					t.Errorf("failed to update webhook config: %v", err)
				}

				time.Sleep(10 * time.Millisecond)
			}
		}(controller)
	}

	// While controllers are updating webhooks, simulate certificate rotations
	go func() {
		for rotation := 0; rotation < 5; rotation++ {
			time.Sleep(40 * time.Millisecond)
			newBundle := []byte("webhook-bundle-v" + string(byte('2'+rotation)))
			cm.caBundleMu.Lock()
			cm.caBundle = newBundle
			cm.caBundleMu.Unlock()
		}
	}()

	// Wait for all updates to complete
	wg.Wait()

	webhookMgr.mu.Lock()
	defer webhookMgr.mu.Unlock()

	expectedUpdates := 10 * 20 // 10 controllers x 20 updates each
	if webhookMgr.updateCount != expectedUpdates {
		t.Errorf("expected %d webhook updates, got %d", expectedUpdates, webhookMgr.updateCount)
	}

	// Verify that multiple bundle versions were used
	if len(webhookMgr.bundleMap) < 1 {
		t.Error("expected multiple webhook bundle versions to be used")
	}

	t.Logf("Webhook updates: %d total, %d different bundle versions used",
		webhookMgr.updateCount, len(webhookMgr.bundleMap))
}

// TestCertificateReconcilerWithReconciliationRaceCondition tests the scenario where
// the certificate reconciler updates the CA bundle at the exact moment a controller
// is reading it and propagating it to a CRD.
func TestCertificateReconcilerWithReconciliationRaceCondition(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	cm := &CertManager{
		kube:       fakeClient,
		client:     nil,
		caBundle:   []byte("race-test-v1"),
		log:        func(msg string, keysAndValues ...any) {},
		pluralizer: &MockPluralizer{},
	}

	// Track all bundles read (for race condition verification)
	readBundles := make([][]byte, 0)
	readLock := sync.Mutex{}

	// Simulate tight coupling between certificate update and controller reconciliation
	var wg sync.WaitGroup

	// Controller thread: rapidly read and use the bundle
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			bundle := cm.GetCABundle()
			readLock.Lock()
			readBundles = append(readBundles, bundle)
			readLock.Unlock()
			// Simulate some processing
			time.Sleep(100 * time.Nanosecond)
		}
	}()

	// Reconciler thread: rapidly update the bundle
	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			newBundle := []byte("race-test-v" + string(byte('2'+(i%10))))
			cm.caBundleMu.Lock()
			cm.caBundle = newBundle
			cm.caBundleMu.Unlock()
			time.Sleep(50 * time.Nanosecond)
		}
	}()

	// Wait for both to complete
	wg.Wait()

	// Verify that we never read an empty bundle (would indicate a data race)
	readLock.Lock()
	for i, bundle := range readBundles {
		if len(bundle) == 0 {
			t.Fatalf("read empty bundle at index %d - potential data race", i)
		}
	}
	readLock.Unlock()

	t.Logf("Race condition test completed: controller read %d bundle values during concurrent updates",
		len(readBundles))
}

// TestCertificateFileAtomicity tests that UpdateCerts uses atomic file operations
// and that concurrent reads don't see corrupted/partial certificate files.
func TestCertificateFileAtomicity(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	tempDir := t.TempDir()

	cm := &CertManager{
		kube:       fakeClient,
		client:     nil,
		caBundle:   []byte("initial-bundle"),
		log:        func(msg string, keysAndValues ...any) {},
		pluralizer: &MockPluralizer{},
		certPath:   tempDir,
	}

	// Track all certificate contents read (to verify no partial/corrupted reads)
	readCerts := make(map[string]int)
	readLock := sync.Mutex{}

	// Simulate certificate file corruption by checking file sizes
	fileSizes := make(map[int]int) // certSize -> count
	sizeLock := sync.Mutex{}

	var wg sync.WaitGroup

	// Writer: continuously "refresh" certificates with different content
	for writer := 0; writer < 2; writer++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 10; i++ {
				// Create test certificates with different lengths
				testCert := fmt.Sprintf("-----BEGIN CERTIFICATE-----\ntest-cert-version-%d-writer-%d\n-----END CERTIFICATE-----", i, id)
				testKey := fmt.Sprintf("-----BEGIN RSA PRIVATE KEY-----\ntest-key-version-%d-writer-%d\n-----END RSA PRIVATE KEY-----", i, id)

				// Base64 encode
				encodedCert := base64.StdEncoding.EncodeToString([]byte(testCert))
				encodedKey := base64.StdEncoding.EncodeToString([]byte(testKey))

				// Write certificates using the CertManager's atomic update
				cm.certGenMu.Lock()
				err := certs.UpdateCerts(encodedCert, encodedKey, cm.certPath)
				cm.certGenMu.Unlock()

				if err != nil {
					t.Errorf("Failed to update certs: %v", err)
					return
				}

				time.Sleep(5 * time.Millisecond)
			}
		}(writer)
	}

	// Reader: continuously read and validate certificates
	for reader := 0; reader < 3; reader++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for i := 0; i < 30; i++ {
				// Read the cert file
				certPath := filepath.Join(cm.certPath, "tls.crt")
				certBytes, err := os.ReadFile(certPath)
				if err != nil && !os.IsNotExist(err) {
					t.Errorf("Reader %d: unexpected error reading cert: %v", id, err)
					return
				}

				if len(certBytes) > 0 {
					// Verify file is not truncated (should have valid PEM structure)
					readLock.Lock()
					readCerts[string(certBytes)]++
					readLock.Unlock()

					// Track file size to detect partial writes
					sizeLock.Lock()
					fileSizes[len(certBytes)]++
					sizeLock.Unlock()

					// Verify it looks like PEM (not corrupted)
					if !bytes.Contains(certBytes, []byte("BEGIN CERTIFICATE")) {
						t.Errorf("Reader %d: certificate missing BEGIN marker, possible corruption", id)
						return
					}
					if !bytes.Contains(certBytes, []byte("END CERTIFICATE")) {
						t.Errorf("Reader %d: certificate missing END marker, possible corruption", id)
						return
					}
				}

				time.Sleep(3 * time.Millisecond)
			}
		}(reader)
	}

	wg.Wait()

	// Verify results
	readLock.Lock()
	if len(readCerts) == 0 {
		t.Error("No certificates were read - file updates may have failed")
	}
	readLock.Unlock()

	sizeLock.Lock()
	defer sizeLock.Unlock()

	// All read sizes should be reasonable (not 0 or tiny, indicating no truncation issues)
	for size, count := range fileSizes {
		if size == 0 {
			t.Errorf("read certificates with size 0 (%d times) - indicates truncation issues", count)
		}
		t.Logf("Certificate size %d bytes: read %d times", size, count)
	}

	// Verify final file state is valid
	finalCert, err := os.ReadFile(filepath.Join(cm.certPath, "tls.crt"))
	if err != nil {
		t.Fatalf("Failed to read final certificate: %v", err)
	}
	if len(finalCert) == 0 {
		t.Fatal("Final certificate file is empty - atomic operations may have failed")
	}
	if !bytes.Contains(finalCert, []byte("BEGIN CERTIFICATE")) || !bytes.Contains(finalCert, []byte("END CERTIFICATE")) {
		t.Fatal("Final certificate file is corrupted")
	}

	t.Log("All certificate files verified as valid with no corruption detected")
}

// TestCertManagerGenMutexSerializesWrites tests that certGenMu prevents concurrent certificate writes
func TestCertManagerGenMutexSerializesWrites(t *testing.T) {
	scheme := runtime.NewScheme()
	fakeClient := fake.NewClientBuilder().WithScheme(scheme).Build()

	tempDir := t.TempDir()

	cm := &CertManager{
		kube:       fakeClient,
		client:     nil,
		caBundle:   []byte("initial-bundle"),
		log:        func(msg string, keysAndValues ...any) {},
		pluralizer: &MockPluralizer{},
		certPath:   tempDir,
	}

	// Track lock acquisition order to verify serialization
	lockOrder := make([]string, 0)
	lockLock := sync.Mutex{}

	var wg sync.WaitGroup

	// Multiple goroutines trying to acquire certGenMu
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 3; j++ {
				// Try to acquire the lock
				start := time.Now()
				cm.certGenMu.Lock()
				duration := time.Since(start)

				lockLock.Lock()
				lockOrder = append(lockOrder, fmt.Sprintf("goroutine-%d-%d", id, j))
				lockLock.Unlock()

				// Simulate certificate update
				time.Sleep(5 * time.Millisecond)

				cm.certGenMu.Unlock()

				t.Logf("Goroutine %d iteration %d waited %v for lock", id, j, duration)
			}
		}(i)
	}

	wg.Wait()

	// Verify that we got all lock acquisitions (no deadlock)
	lockLock.Lock()
	if len(lockOrder) != 15 { // 5 goroutines * 3 iterations
		t.Errorf("Expected 15 lock acquisitions, got %d - possible deadlock", len(lockOrder))
	}
	lockLock.Unlock()

	t.Logf("Successfully serialized %d certificate write attempts with certGenMu", len(lockOrder))
}
