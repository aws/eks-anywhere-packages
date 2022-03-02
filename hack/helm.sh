#!/bin/bash -e

chart() {
    (
        cd charts
        helm lint eks-anywhere-packages
        helm package eks-anywhere-packages
        helm-docs
    )
}

chart