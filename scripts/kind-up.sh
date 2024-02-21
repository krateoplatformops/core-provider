#!/bin/bash

kind get kubeconfig >/dev/null 2>&1 || kind create cluster
