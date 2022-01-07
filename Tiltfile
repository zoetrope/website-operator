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

def manifests():
    return './bin/controller-gen crd rbac:roleName=manager-role webhook paths="./..." output:crd:artifacts:config=config/crd/bases;'

def generate():
    return './bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./...";'

def operator_binary():
    return 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/website-operator cmd/website-operator/main.go'

def repochecker_binary():
    return 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/repo-checker cmd/repo-checker/main.go'

def ui_frontend():
    return 'cd ui/frontend && npm install && npm run build && cd ../../;'

def ui_backend():
    return 'CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o bin/website-operator-ui cmd/ui/main.go'

installed = local("which kubebuilder")
print("kubebuilder is present:", installed)

DIRNAME = os.path.basename(os. getcwd())

local(manifests() + generate())

local_resource('CRD', manifests() + 'kustomize build config/crd | kubectl apply -f -', deps=["api"], ignore=['*/*/zz_generated.deepcopy.go'])

watch_settings(ignore=['config/crd/bases/', 'config/rbac/role.yaml', 'config/webhook/manifests.yaml'])
k8s_yaml(kustomize('./config/dev'))

operator_deps = ['api', 'controllers', 'pkg', 'hooks', 'cmd/website-operator', 'version.go', 'constants.go']
local_resource('Watch&Compile website-operator', generate() + operator_binary(), deps=operator_deps, ignore=['*/*/zz_generated.deepcopy.go'])

repochecker_deps = ['checker', 'cmd/repo-checker', 'version.go', 'constants.go']
local_resource('Watch&Compile repo-checker', repochecker_binary(), deps=repochecker_deps)

ui_deps = ['ui', 'cmd/ui', 'version.go', 'constants.go']
local_resource('Watch&Compile ui', ui_frontend() + ui_backend(), deps=ui_deps, ignore=['ui/frontend/node_modules', 'ui/frontend/dist', 'ui/frontend/.parcel-cache', 'ui/frontend/package*'])

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

