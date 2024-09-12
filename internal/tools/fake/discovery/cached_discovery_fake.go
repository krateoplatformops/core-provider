package fake

import (
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"bytes"
	"crypto/sha256"
	"fmt"

	openapi_v2 "github.com/google/gnostic-models/openapiv2"
	"github.com/gregjones/httpcache"
	"github.com/peterbourgon/diskv"
	"k8s.io/klog/v2"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/openapi"
	cachedopenapi "k8s.io/client-go/openapi/cached"
	restclient "k8s.io/client-go/rest"
)

// CachedDiscoveryClient implements the functions that discovery server-supported API groups,
// versions and resources.
type CachedDiscoveryClient struct {
	delegate discovery.DiscoveryInterface

	// cacheDirectory is the directory where discovery docs are held.  It must be unique per host:port combination to work well.
	cacheDirectory string

	// ttl is how long the cache should be considered valid
	ttl time.Duration

	// mutex protects the variables below
	mutex sync.Mutex

	// ourFiles are all filenames of cache files created by this process
	ourFiles map[string]struct{}
	// invalidated is true if all cache files should be ignored that are not ours (e.g. after Invalidate() was called)
	invalidated bool
	// fresh is true if all used cache files were ours
	fresh bool

	// caching openapi v3 client which wraps the delegate's client
	openapiClient openapi.Client
}

var _ discovery.CachedDiscoveryInterface = &CachedDiscoveryClient{}

// ServerResourcesForGroupVersion returns the supported resources for a group and version.
func (d *CachedDiscoveryClient) ServerResourcesForGroupVersion(groupVersion string) (*metav1.APIResourceList, error) {
	filename := filepath.Join(d.cacheDirectory, groupVersion, "serverresources.json")
	cachedBytes, err := d.getCachedFile(filename)
	// don't fail on errors, we either don't have a file or won't be able to run the cached check. Either way we can fallback.
	if err == nil {
		cachedResources := &metav1.APIResourceList{}
		if err := runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), cachedBytes, cachedResources); err == nil {
			klog.V(10).Infof("returning cached discovery info from %v", filename)
			return cachedResources, nil
		}
	}

	liveResources, err := d.delegate.ServerResourcesForGroupVersion(groupVersion)
	if err != nil {
		klog.V(3).Infof("skipped caching discovery info due to %v", err)
		return liveResources, err
	}
	if liveResources == nil || len(liveResources.APIResources) == 0 {
		klog.V(3).Infof("skipped caching discovery info, no resources found")
		return liveResources, err
	}

	if err := d.writeCachedFile(filename, liveResources); err != nil {
		klog.V(1).Infof("failed to write cache to %v due to %v", filename, err)
	}

	return liveResources, nil
}

// ServerGroupsAndResources returns the supported groups and resources for all groups and versions.
func (d *CachedDiscoveryClient) ServerGroupsAndResources() ([]*metav1.APIGroup, []*metav1.APIResourceList, error) {
	return discovery.ServerGroupsAndResources(d)
}

// ServerGroups returns the supported groups, with information like supported versions and the
// preferred version.
func (d *CachedDiscoveryClient) ServerGroups() (*metav1.APIGroupList, error) {
	filename := filepath.Join(d.cacheDirectory, "servergroups.json")
	cachedBytes, err := d.getCachedFile(filename)
	// don't fail on errors, we either don't have a file or won't be able to run the cached check. Either way we can fallback.
	if err == nil {
		cachedGroups := &metav1.APIGroupList{}
		if err := runtime.DecodeInto(scheme.Codecs.UniversalDecoder(), cachedBytes, cachedGroups); err == nil {
			klog.V(10).Infof("returning cached discovery info from %v", filename)
			return cachedGroups, nil
		}
	}

	liveGroups, err := d.delegate.ServerGroups()
	if err != nil {
		klog.V(3).Infof("skipped caching discovery info due to %v", err)
		return liveGroups, err
	}
	if liveGroups == nil || len(liveGroups.Groups) == 0 {
		klog.V(3).Infof("skipped caching discovery info, no groups found")
		return liveGroups, err
	}

	if err := d.writeCachedFile(filename, liveGroups); err != nil {
		klog.V(1).Infof("failed to write cache to %v due to %v", filename, err)
	}

	return liveGroups, nil
}

func (d *CachedDiscoveryClient) getCachedFile(filename string) ([]byte, error) {
	// after invalidation ignore cache files not created by this process
	d.mutex.Lock()
	_, ourFile := d.ourFiles[filename]
	if d.invalidated && !ourFile {
		d.mutex.Unlock()
		return nil, errors.New("cache invalidated")
	}
	d.mutex.Unlock()

	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return nil, err
	}

	if time.Now().After(fileInfo.ModTime().Add(d.ttl)) {
		return nil, errors.New("cache expired")
	}

	// the cache is present and its valid.  Try to read and use it.
	cachedBytes, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}

	d.mutex.Lock()
	defer d.mutex.Unlock()
	d.fresh = d.fresh && ourFile

	return cachedBytes, nil
}

func (d *CachedDiscoveryClient) writeCachedFile(filename string, obj runtime.Object) error {
	if err := os.MkdirAll(filepath.Dir(filename), 0750); err != nil {
		return err
	}

	bytes, err := runtime.Encode(scheme.Codecs.LegacyCodec(), obj)
	if err != nil {
		return err
	}

	f, err := os.CreateTemp(filepath.Dir(filename), filepath.Base(filename)+".")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	_, err = f.Write(bytes)
	if err != nil {
		return err
	}

	err = os.Chmod(f.Name(), 0660)
	if err != nil {
		return err
	}

	name := f.Name()
	err = f.Close()
	if err != nil {
		return err
	}

	// atomic rename
	d.mutex.Lock()
	defer d.mutex.Unlock()
	err = os.Rename(name, filename)
	if err == nil {
		d.ourFiles[filename] = struct{}{}
	}
	return err
}

// RESTClient returns a RESTClient that is used to communicate with API server
// by this client implementation.
func (d *CachedDiscoveryClient) RESTClient() restclient.Interface {
	return d.delegate.RESTClient()
}

// ServerPreferredResources returns the supported resources with the version preferred by the
// server.
func (d *CachedDiscoveryClient) ServerPreferredResources() ([]*metav1.APIResourceList, error) {
	return discovery.ServerPreferredResources(d)
}

// ServerPreferredNamespacedResources returns the supported namespaced resources with the
// version preferred by the server.
func (d *CachedDiscoveryClient) ServerPreferredNamespacedResources() ([]*metav1.APIResourceList, error) {
	return discovery.ServerPreferredNamespacedResources(d)
}

// ServerVersion retrieves and parses the server's version (git version).
func (d *CachedDiscoveryClient) ServerVersion() (*version.Info, error) {
	return d.delegate.ServerVersion()
}

// OpenAPISchema retrieves and parses the swagger API schema the server supports.
func (d *CachedDiscoveryClient) OpenAPISchema() (*openapi_v2.Document, error) {
	return d.delegate.OpenAPISchema()
}

// OpenAPIV3 retrieves and parses the OpenAPIV3 specs exposed by the server
func (d *CachedDiscoveryClient) OpenAPIV3() openapi.Client {
	// Must take lock since Invalidate call may modify openapiClient
	d.mutex.Lock()
	defer d.mutex.Unlock()

	if d.openapiClient == nil {
		// Delegate is discovery client created with special HTTP client which
		// respects E-Tag cache responses to serve cache from disk.
		d.openapiClient = cachedopenapi.NewClient(d.delegate.OpenAPIV3())
	}

	return d.openapiClient
}

// Fresh is supposed to tell the caller whether or not to retry if the cache
// fails to find something (false = retry, true = no need to retry).
func (d *CachedDiscoveryClient) Fresh() bool {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	return d.fresh
}

// Invalidate enforces that no cached data is used in the future that is older than the current time.
func (d *CachedDiscoveryClient) Invalidate() {
	d.mutex.Lock()
	defer d.mutex.Unlock()

	d.ourFiles = map[string]struct{}{}
	d.fresh = true
	d.invalidated = true
	d.openapiClient = nil
	if ad, ok := d.delegate.(discovery.CachedDiscoveryInterface); ok {
		ad.Invalidate()
	}
}

// WithLegacy returns current cached discovery client;
// current client does not support legacy-only discovery.
func (d *CachedDiscoveryClient) WithLegacy() discovery.DiscoveryInterface {
	return d
}

// NewCachedDiscoveryClientForConfig creates a new DiscoveryClient for the given config, and wraps
// the created client in a CachedDiscoveryClient. The provided configuration is updated with a
// custom transport that understands cache responses.
// We receive two distinct cache directories for now, in order to preserve old behavior
// which makes use of the --cache-dir flag value for storing cache data from the CacheRoundTripper,
// and makes use of the hardcoded destination (~/.kube/cache/discovery/...) for storing
// CachedDiscoveryClient cache data. If httpCacheDir is empty, the restconfig's transport will not
// be updated with a roundtripper that understands cache responses.
// If discoveryCacheDir is empty, cached server resource data will be looked up in the current directory.
func NewCachedDiscoveryClientForConfig(config *restclient.Config, discoveryCacheDir, httpCacheDir string, ttl time.Duration) (*CachedDiscoveryClient, error) {
	if len(httpCacheDir) > 0 {
		// update the given restconfig with a custom roundtripper that
		// understands how to handle cache responses.
		config = restclient.CopyConfig(config)
		config.Wrap(func(rt http.RoundTripper) http.RoundTripper {
			return newCacheRoundTripper(httpCacheDir, rt)
		})
	}

	discoveryClient, err := discovery.NewDiscoveryClientForConfig(config)
	if err != nil {
		return nil, err
	}

	// The delegate caches the discovery groups and resources (memcache). "ServerGroups",
	// which usually only returns (and caches) the groups, can now store the resources as
	// well if the server supports the newer aggregated discovery format.
	return NewCachedDiscoveryClient(memory.NewMemCacheClient(discoveryClient), discoveryCacheDir, ttl), nil
}

// NewCachedDiscoveryClient creates a new DiscoveryClient.  cacheDirectory is the directory where discovery docs are held.  It must be unique per host:port combination to work well.
func NewCachedDiscoveryClient(delegate discovery.DiscoveryInterface, cacheDirectory string, ttl time.Duration) *CachedDiscoveryClient {
	return &CachedDiscoveryClient{
		delegate:       delegate,
		cacheDirectory: cacheDirectory,
		ttl:            ttl,
		ourFiles:       map[string]struct{}{},
		fresh:          true,
	}
}

type cacheRoundTripper struct {
	rt *httpcache.Transport
}

// newCacheRoundTripper creates a roundtripper that reads the ETag on
// response headers and send the If-None-Match header on subsequent
// corresponding requests.
func newCacheRoundTripper(cacheDir string, rt http.RoundTripper) http.RoundTripper {
	d := diskv.New(diskv.Options{
		PathPerm: os.FileMode(0750),
		FilePerm: os.FileMode(0660),
		BasePath: cacheDir,
		TempDir:  filepath.Join(cacheDir, ".diskv-temp"),
	})
	t := httpcache.NewTransport(&sumDiskCache{disk: d})
	t.Transport = rt

	return &cacheRoundTripper{rt: t}
}

func (rt *cacheRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return rt.rt.RoundTrip(req)
}

func (rt *cacheRoundTripper) CancelRequest(req *http.Request) {
	type canceler interface {
		CancelRequest(*http.Request)
	}
	if cr, ok := rt.rt.Transport.(canceler); ok {
		cr.CancelRequest(req)
	} else {
		klog.Errorf("CancelRequest not implemented by %T", rt.rt.Transport)
	}
}

func (rt *cacheRoundTripper) WrappedRoundTripper() http.RoundTripper { return rt.rt.Transport }

// A sumDiskCache is a cache backend for github.com/gregjones/httpcache. It is
// similar to httpcache's diskcache package, but uses SHA256 sums to ensure
// cache integrity at read time rather than fsyncing each cache entry to
// increase the likelihood they will be persisted at write time. This avoids
// significant performance degradation on MacOS.
//
// See https://github.com/kubernetes/kubernetes/issues/110753 for more.
type sumDiskCache struct {
	disk *diskv.Diskv
}

// Get the requested key from the cache on disk. If Get encounters an error, or
// the returned value is not a SHA256 sum followed by bytes with a matching
// checksum it will return false to indicate a cache miss.
func (c *sumDiskCache) Get(key string) ([]byte, bool) {
	b, err := c.disk.Read(sanitize(key))
	if err != nil || len(b) < sha256.Size {
		return []byte{}, false
	}

	response := b[sha256.Size:]
	want := b[:sha256.Size] // The first 32 bytes of the file should be the SHA256 sum.
	got := sha256.Sum256(response)
	if !bytes.Equal(want, got[:]) {
		return []byte{}, false
	}

	return response, true
}

// Set writes the response to a file on disk. The filename will be the SHA256
// sum of the key. The file will contain a SHA256 sum of the response bytes,
// followed by said response bytes.
func (c *sumDiskCache) Set(key string, response []byte) {
	s := sha256.Sum256(response)
	_ = c.disk.Write(sanitize(key), append(s[:], response...)) // Nothing we can do with this error.
}

func (c *sumDiskCache) Delete(key string) {
	_ = c.disk.Erase(sanitize(key)) // Nothing we can do with this error.
}

// Sanitize an httpcache key such that it can be used as a diskv key, which must
// be a valid filename. The httpcache key will either be the requested URL (if
// the request method was GET) or "<method> <url>" for other methods, per the
// httpcache.cacheKey function.
func sanitize(key string) string {
	// These keys are not sensitive. We use sha256 to avoid a (potentially
	// malicious) collision causing the wrong cache data to be written or
	// accessed.
	return fmt.Sprintf("%x", sha256.Sum256([]byte(key)))
}
