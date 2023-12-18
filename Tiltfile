# -*- mode: Python -*-
# derived from https://github.com/tilt-dev/tilt-example-go/tree/master/3-recommended

registry = os.getenv('REGISTRY')
k8s_contexts = os.getenv('KUBERNETES_CONTEXTS')

allow_k8s_contexts(k8s_contexts)
default_registry(registry)

# ToDo:
# - scope down this build so that it only responds to appropriate code changes
# - build executable outside container and configure tilt to inject into and restart container
docker_build(
  'packages-controller',
  '.',
)

k8s_yaml('tilt/deployment.yaml')