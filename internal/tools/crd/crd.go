package crd

// import (
// 	"context"
// 	"log"

// 	"github.com/avast/retry-go"
// 	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
// 	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
// 	apierrors "k8s.io/apimachinery/pkg/api/errors"
// 	"k8s.io/apimachinery/pkg/runtime/schema"
// 	"k8s.io/apimachinery/pkg/runtime/serializer/json"
// 	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
// 	"sigs.k8s.io/controller-runtime/pkg/client"
// )

// func UninstallCRD(ctx context.Context, kube client.Client, gr schema.GroupResource) error {
// 	return retry.Do(
// 		func() error {
// 			obj := apiextensionsv1.CustomResourceDefinition{}
// 			err := kube.Get(ctx, client.ObjectKey{Name: gr.String()}, &obj, &client.GetOptions{})
// 			if err != nil {
// 				if apierrors.IsNotFound(err) {
// 					return nil
// 				}

// 				return err
// 			}

// 			err = kube.Delete(ctx, &obj, &client.DeleteOptions{})
// 			if err != nil {
// 				if apierrors.IsNotFound(err) {
// 					return nil
// 				}

// 				return err
// 			}

// 			return nil
// 		},
// 	)
// }

// func InstallCRD(ctx context.Context, kube client.Client, obj *apiextensionsv1.CustomResourceDefinition) error {
// 	return retry.Do(
// 		func() error {
// 			tmp := apiextensionsv1.CustomResourceDefinition{}
// 			err := kube.Get(ctx, client.ObjectKeyFromObject(obj), &tmp)
// 			if err != nil {
// 				if apierrors.IsNotFound(err) {
// 					return kube.Create(ctx, obj)
// 				}

// 				return err
// 			}

// 			gracePeriod := int64(0)
// 			_ = kube.Delete(ctx, &tmp, &client.DeleteOptions{GracePeriodSeconds: &gracePeriod})

// 			return kube.Create(ctx, obj)
// 		},
// 	)
// }

// func LookupCRD(ctx context.Context, kube client.Client, gvr schema.GroupVersionResource) (bool, error) {
// 	res := apiextensionsv1.CustomResourceDefinition{}
// 	err := kube.Get(ctx, client.ObjectKey{Name: gvr.GroupResource().String()}, &res, &client.GetOptions{})
// 	if err != nil {
// 		if apierrors.IsNotFound(err) {
// 			log.Printf("[WRN] CRD NOT found (gvr: %s)\n", gvr.String())
// 			return false, nil
// 		} else {
// 			return false, err
// 		}
// 	}

// 	log.Printf("[DBG] Looking for matching version (%s)\n", gvr.Version)
// 	for _, el := range res.Spec.Versions {
// 		if el.Name == gvr.Version {
// 			return true, nil
// 		}
// 	}

// 	return false, nil
// }

// func UnmarshalCRD(dat []byte) (*apiextensionsv1.CustomResourceDefinition, error) {
// 	if !clientsetscheme.Scheme.IsGroupRegistered("apiextensions.k8s.io") {
// 		_ = apiextensionsscheme.AddToScheme(clientsetscheme.Scheme)
// 	}

// 	s := json.NewYAMLSerializer(json.DefaultMetaFactory,
// 		clientsetscheme.Scheme,
// 		clientsetscheme.Scheme)

// 	res := &apiextensionsv1.CustomResourceDefinition{}
// 	_, _, err := s.Decode(dat, nil, res)
// 	return res, err
// }

import (
	"context"
	"fmt"
	"strings"

	gojson "encoding/json"

	"github.com/avast/retry-go"
	"github.com/krateoplatformops/core-provider/internal/tools/crd/jsonequality"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	apiextensionsscheme "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset/scheme"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	clientsetscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/gengo/namer"
	"k8s.io/gengo/types"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func InferGroupResource(gk schema.GroupKind) schema.GroupResource {
	kind := types.Type{Name: types.Name{Name: gk.Kind}}
	namer := namer.NewPrivatePluralNamer(nil)
	return schema.GroupResource{
		Group:    gk.Group,
		Resource: strings.ToLower(namer.Name(&kind)),
	}
}

func Update(ctx context.Context, kube client.Client, name string, newObj *apiextensionsv1.CustomResourceDefinition) error {
	if err := registerEventually(); err != nil {
		return err
	}

	return retry.Do(
		func() error {
			obj := apiextensionsv1.CustomResourceDefinition{}
			err := kube.Get(ctx, client.ObjectKey{Name: name}, &obj, &client.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					fmt.Println("CRD not found")
					return nil
				}

				return err
			}
			newObj.SetResourceVersion(obj.GetResourceVersion())

			err = kube.Update(ctx, newObj, &client.UpdateOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					fmt.Print("CRD not found")
					return nil
				}

				return err
			}

			return nil
		},
	)
}

func Uninstall(ctx context.Context, kube client.Client, gr schema.GroupResource) error {
	if err := registerEventually(); err != nil {
		return err
	}

	return retry.Do(
		func() error {
			obj := apiextensionsv1.CustomResourceDefinition{}
			err := kube.Get(ctx, client.ObjectKey{Name: gr.String()}, &obj, &client.GetOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}

				return err
			}

			err = kube.Delete(ctx, &obj, &client.DeleteOptions{})
			if err != nil {
				if apierrors.IsNotFound(err) {
					return nil
				}

				return err
			}

			return nil
		},
	)
}

func Install(ctx context.Context, kube client.Client, obj *apiextensionsv1.CustomResourceDefinition) error {
	if err := registerEventually(); err != nil {
		return err
	}

	return retry.Do(
		func() error {
			tmp := apiextensionsv1.CustomResourceDefinition{}
			err := kube.Get(ctx, client.ObjectKeyFromObject(obj), &tmp)
			if err != nil {
				if apierrors.IsNotFound(err) {
					return kube.Create(ctx, obj)
				}

				return err
			}

			gracePeriod := int64(0)
			_ = kube.Delete(ctx, &tmp, &client.DeleteOptions{GracePeriodSeconds: &gracePeriod})

			return kube.Create(ctx, obj)
		},
	)
}

func Get(ctx context.Context, kube client.Client, gvr schema.GroupVersionResource) (*apiextensionsv1.CustomResourceDefinition, error) {
	if err := registerEventually(); err != nil {
		return nil, err
	}

	res := apiextensionsv1.CustomResourceDefinition{}
	err := kube.Get(ctx, client.ObjectKey{Name: gvr.GroupResource().String()}, &res, &client.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	return &res, nil
}

func Lookup(ctx context.Context, kube client.Client, gvr schema.GroupVersionResource) (bool, error) {
	if err := registerEventually(); err != nil {
		return false, err
	}

	res := apiextensionsv1.CustomResourceDefinition{}
	err := kube.Get(ctx, client.ObjectKey{Name: gvr.GroupResource().String()}, &res, &client.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return false, nil
		} else {
			return false, err
		}
	}

	for _, el := range res.Spec.Versions {
		if el.Name == gvr.Version {
			return true, nil
		}
	}

	return false, nil
}

func Unmarshal(dat []byte) (*apiextensionsv1.CustomResourceDefinition, error) {
	if err := registerEventually(); err != nil {
		return nil, err
	}

	s := json.NewYAMLSerializer(json.DefaultMetaFactory,
		clientsetscheme.Scheme,
		clientsetscheme.Scheme)

	res := &apiextensionsv1.CustomResourceDefinition{}
	_, _, err := s.Decode(dat, nil, res)
	return res, err
}

// AppendVersion appends the version of the toadd CRD to the crd CRD and sets the Storage and Served fields in the last version of the crd CRD.
func AppendVersion(crd apiextensionsv1.CustomResourceDefinition, toadd apiextensionsv1.CustomResourceDefinition) (*apiextensionsv1.CustomResourceDefinition, error) {
	for _, el2 := range toadd.Spec.Versions {
		exist := false
		for _, el1 := range crd.Spec.Versions {
			if el1.Name == el2.Name {
				exist = true
				break
			}

		}

		if !exist {
			existingschema, err := gojson.Marshal(crd.Spec.Versions[0].Schema.OpenAPIV3Schema.Properties)
			if err != nil {
				fmt.Println("error: ", err)
			}
			newschema, err := gojson.Marshal(el2.Schema.OpenAPIV3Schema.Properties)
			if err != nil {
				fmt.Println("error: ", err)
			}

			if jsonequality.JSONEq(string(existingschema), string(newschema)) {
				crd.Spec.Versions = append(crd.Spec.Versions, el2)
			} else {
				return nil, fmt.Errorf("schema not equal")
			}
		}
	}
	if len(crd.Spec.Versions) > 1 {
		// Latest version is the last version - not ideal
		// var versions []string
		// for i := range crd.Spec.Versions {
		// 	versions = append(versions, crd.Spec.Versions[i].Name)
		// }
		// versions = versionsort.SortVersions(versions)
		// for i := range crd.Spec.Versions {
		// 	if crd.Spec.Versions[i].Name == versions[0] {
		// 		crd.Spec.Versions[i].Storage = true
		// 		crd.Spec.Versions[i].Served = true
		// 	} else {
		// 		crd.Spec.Versions[i].Storage = false
		// 		crd.Spec.Versions[i].Served = false
		// 	}
		// }

		// Latest version has to be the newly added version
		SetServedStorage(&crd, toadd.Spec.Versions[0].Name)
	}

	return &crd, nil
}

func SetServedStorage(crd *apiextensionsv1.CustomResourceDefinition, version string) {
	for i := range crd.Spec.Versions {
		if crd.Spec.Versions[i].Name == version {
			crd.Spec.Versions[i].Served = true
			crd.Spec.Versions[i].Storage = true
		} else {
			crd.Spec.Versions[i].Served = false
			crd.Spec.Versions[i].Storage = false
		}
	}
}

func registerEventually() error {
	if clientsetscheme.Scheme.IsGroupRegistered("apiextensions.k8s.io") {
		return nil
	}

	return apiextensionsscheme.AddToScheme(clientsetscheme.Scheme)
}
