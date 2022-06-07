module github.com/aws/eks-anywhere-packages/kubetest-plugins

go 1.17

require (
	github.com/octago/sflags v0.2.0
	github.com/spf13/pflag v1.0.5
	k8s.io/klog v1.0.0
	sigs.k8s.io/kind v0.11.1
	sigs.k8s.io/kubetest2 v0.0.0-20211202193745-acc28b44b0ad
)

require (
	github.com/alessio/shellescape v1.4.1 // indirect
	github.com/google/uuid v1.2.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/kballard/go-shellquote v0.0.0-20180428030007-95032a82bc51 // indirect
	github.com/pkg/errors v0.9.1 // indirect
	github.com/spf13/cobra v1.1.3 // indirect
	gopkg.in/yaml.v3 v3.0.0 // indirect
	k8s.io/apimachinery v0.22.5 // indirect
)

replace (
	github.com/containerd/containerd => github.com/containerd/containerd v1.6.6
	github.com/docker/distribution => github.com/docker/distribution v2.8.0-beta.1+incompatible
)
