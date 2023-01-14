load('ext://restart_process', 'docker_build_with_restart')

OPERATOR_DOCKERFILE = '''FROM golang:alpine
WORKDIR /
COPY ./bin/website-operator /
CMD ["/website-operator"]
'''

REPOCHECKER_DOCKERFILE = '''FROM golang:alpine
WORKDIR /
COPY ./bin/repo-checker /
CMD ["/repo-checker"]
'''

UI_DOCKERFILE = '''FROM golang:alpine
WORKDIR /
COPY ./ui/frontend/dist/* /dist/
COPY ./bin/website-operator-ui /
CMD ["/website-operator-ui"]
'''

# Generate manifests and go files
local_resource('make manifests', "make manifests", deps=["api", "controllers"], ignore=['*/*/zz_generated.deepcopy.go'])
local_resource('make generate', "make generate", deps=["api", "controllers"], ignore=['*/*/zz_generated.deepcopy.go'])

# Deploy CRD
local_resource(
    'CRD', 'make install', deps=["api"],
    ignore=['*/*/zz_generated.deepcopy.go'])

installed = local("which kubebuilder")
print("kubebuilder is present:", installed)

DIRNAME = os.path.basename(os. getcwd())

watch_settings(ignore=['config/crd/bases/', 'config/rbac/role.yaml', 'config/webhook/manifests.yaml'])
k8s_yaml(kustomize('./config/dev'))

operator_deps = ['api', 'controllers', 'cmd/website-operator', 'version.go', 'constants.go']
local_resource('Watch&Compile website-operator', "make bin/website-operator", deps=operator_deps, ignore=['*/*/zz_generated.deepcopy.go'])

repochecker_deps = ['checker', 'cmd/repo-checker', 'version.go', 'constants.go']
local_resource('Watch&Compile repo-checker', "make bin/repo-checker", deps=repochecker_deps)

ui_deps = ['ui', 'cmd/website-operator-ui', 'version.go', 'constants.go']
local_resource('Watch&Compile website-operator-ui', "make frontend; make bin/website-operator-ui", deps=ui_deps, ignore=['ui/frontend/node_modules', 'ui/frontend/dist', 'ui/frontend/.parcel-cache', 'ui/frontend/package*'])

local_resource('Sample YAML', 'kubectl apply -f ./config/samples', deps=["./config/samples"], resource_deps=[DIRNAME + "-controller-manager"])

docker_build_with_restart('website-operator:dev', '.',
 dockerfile_contents=OPERATOR_DOCKERFILE,
 entrypoint=['/website-operator'],
 only=['./bin/website-operator'],
 live_update=[
       sync('./bin/website-operator', '/website-operator'),
   ]
)

docker_build('repo-checker:dev', '.',
 dockerfile_contents=REPOCHECKER_DOCKERFILE,
 only=['./bin/repo-checker'],
 match_in_env_vars=True
)

docker_build_with_restart('website-operator-ui:dev', '.',
 dockerfile_contents=UI_DOCKERFILE,
 entrypoint=['/website-operator-ui'],
 only=['./bin/website-operator-ui', './ui/frontend/dist'],
 live_update=[
       sync('./bin/website-operator-ui', '/website-operator-ui'),
       sync('./ui/frontend/dist', '/dist'),
   ]
)

k8s_resource(workload='website-operator-ui', port_forwards='8080:8080')

