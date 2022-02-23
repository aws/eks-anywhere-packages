module github.com/aws/eks-anywhere-packages/kubetest-plugins

go 1.16

require (
	github.com/octago/sflags v0.2.0
	github.com/spf13/pflag v1.0.5
	k8s.io/klog v1.0.0
	sigs.k8s.io/kind v0.11.1
	sigs.k8s.io/kubetest2 v0.0.0-20211202193745-acc28b44b0ad
)

replace (
	github.com/containerd/containerd => github.com/containerd/containerd v1.5.9
	github.com/docker/distribution => github.com/docker/distribution v2.8.0-beta.1+incompatible
)
