package objects

import (
	"encoding/json"
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"

	"github.com/stretchr/testify/assert"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

const schemaJSON = `{
  "$schema": "http://json-schema.org/draft-07/schema#",
  "type": "object",
  "title": "Helm Chart Values Schema",
  "description": "Schema for validating Helm chart values",
  "properties": {
    "replicaCount": {
      "type": "integer",
      "minimum": 1,
      "default": 1,
      "description": "Number of replicas for the deployment"
    },
    "image": {
      "type": "object",
      "properties": {
        "repository": {
          "type": "string",
          "default": "nginx",
          "description": "Container image repository"
        },
        "tag": {
          "type": "string",
          "default": "latest",
          "description": "Container image tag"
        },
        "pullPolicy": {
          "type": "string",
          "enum": ["Always", "IfNotPresent", "Never"],
          "default": "IfNotPresent",
          "description": "Image pull policy"
        }
      },
      "required": ["repository"],
      "additionalProperties": false
    },
    "service": {
      "type": "object",
      "properties": {
        "type": {
          "type": "string",
          "enum": ["ClusterIP", "NodePort", "LoadBalancer", "ExternalName"],
          "default": "ClusterIP",
          "description": "Kubernetes service type"
        },
        "port": {
          "type": "integer",
          "minimum": 1,
          "maximum": 65535,
          "default": 80,
          "description": "Service port number"
        },
        "targetPort": {
          "type": "integer",
          "minimum": 1,
          "maximum": 65535,
          "default": 8080,
          "description": "Target port for the service"
        }
      },
      "additionalProperties": false
    },
    "ingress": {
      "type": "object",
      "properties": {
        "enabled": {
          "type": "boolean",
          "default": false,
          "description": "Enable ingress controller resource"
        },
        "className": {
          "type": "string",
          "description": "Ingress class name"
        },
        "annotations": {
          "type": "object",
          "additionalProperties": {
            "type": "string"
          },
          "description": "Ingress annotations"
        },
        "hosts": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "host": {
                "type": "string",
                "description": "Hostname for the ingress rule"
              },
              "paths": {
                "type": "array",
                "items": {
                  "type": "object",
                  "properties": {
                    "path": {
                      "type": "string",
                      "default": "/",
                      "description": "Path for the ingress rule"
                    },
                    "pathType": {
                      "type": "string",
                      "enum": ["Exact", "Prefix", "ImplementationSpecific"],
                      "default": "Prefix",
                      "description": "Path type for the ingress rule"
                    }
                  },
                  "required": ["path", "pathType"]
                }
              }
            },
            "required": ["host", "paths"]
          }
        },
        "tls": {
          "type": "array",
          "items": {
            "type": "object",
            "properties": {
              "secretName": {
                "type": "string",
                "description": "Name of the secret containing TLS certificates"
              },
              "hosts": {
                "type": "array",
                "items": {
                  "type": "string"
                },
                "description": "List of hosts covered by the TLS certificate"
              }
            }
          }
        }
      },
      "additionalProperties": false
    },
    "resources": {
      "type": "object",
      "properties": {
        "limits": {
          "type": "object",
          "properties": {
            "cpu": {
              "type": "string",
              "pattern": "^[0-9]+m?$",
              "description": "CPU limit"
            },
            "memory": {
              "type": "string",
              "pattern": "^[0-9]+(Mi|Gi)$",
              "description": "Memory limit"
            }
          },
          "additionalProperties": false
        },
        "requests": {
          "type": "object",
          "properties": {
            "cpu": {
              "type": "string",
              "pattern": "^[0-9]+m?$",
              "description": "CPU request"
            },
            "memory": {
              "type": "string",
              "pattern": "^[0-9]+(Mi|Gi)$",
              "description": "Memory request"
            }
          },
          "additionalProperties": false
        }
      },
      "additionalProperties": false
    },
    "nodeSelector": {
      "type": "object",
      "additionalProperties": {
        "type": "string"
      },
      "description": "Node selector for pod assignment"
    },
    "tolerations": {
      "type": "array",
      "items": {
        "type": "object",
        "properties": {
          "key": {
            "type": "string"
          },
          "operator": {
            "type": "string",
            "enum": ["Equal", "Exists"]
          },
          "value": {
            "type": "string"
          },
          "effect": {
            "type": "string",
            "enum": ["NoSchedule", "PreferNoSchedule", "NoExecute"]
          }
        }
      },
      "description": "Tolerations for pod assignment"
    },
    "affinity": {
      "type": "object",
      "description": "Affinity settings for pod assignment"
    }
  },
  "required": ["image"],
  "additionalProperties": false
}`

func TestObject(t *testing.T) {
	gvr := schema.GroupVersionResource{
		Group:    "rbac.authorization.k8s.io",
		Version:  "v1",
		Resource: "roles",
	}
	nn := types.NamespacedName{
		Name:      "test-role",
		Namespace: "test-namespace",
	}
	path := "testdata/role_template.yaml"

	role := rbacv1.Role{}

	err := CreateK8sObject(&role, gvr, nn, path, "secretName", "test-value")
	assert.NoError(t, err)

	expectedRules := []rbacv1.PolicyRule{
		{
			APIGroups: []string{"apiextensions.k8s.io"},
			Resources: []string{"customresourcedefinitions"},
			Verbs:     []string{"get", "list"},
		},
		{
			APIGroups: []string{""},
			Resources: []string{"events"},
			Verbs:     []string{"create", "patch", "update"},
		},
		{
			APIGroups:     []string{""},
			Resources:     []string{"secrets"},
			Verbs:         []string{"get", "list", "watch"},
			ResourceNames: []string{"test-value"},
		},
	}

	assert.Equal(t, "rbac.authorization.k8s.io/v1", role.APIVersion)
	assert.Equal(t, "Role", role.Kind)
	assert.Equal(t, gvr.Resource+"-"+gvr.Version, role.Name)
	assert.Equal(t, expectedRules, role.Rules)

	gvr = schema.GroupVersionResource{
		Group:    "composition.krateo.io",
		Version:  "v1alpha2",
		Resource: "fireworksapp",
	}

	nn = types.NamespacedName{
		Name:      "test-fireworksapp",
		Namespace: "test-namespace",
	}

	configmap := corev1.ConfigMap{}
	err = CreateK8sObject(&configmap, gvr, nn, "testdata/configmap_template.yaml", "schema", schemaJSON)
	assert.NoError(t, err)

	fmt.Println("ConfigMap created successfully:")
	b, _ := json.MarshalIndent(configmap, "", "  ")
	fmt.Println(string(b))

}
