# Cass Operator
[![License: Apache License 2.0](https://img.shields.io/github/license/k8ssandra/cass-operator)](https://github.com/k8ssandra/cass-operator/blob/master/LICENSE.txt)

The DataStax Kubernetes Operator for Apache Cassandra&reg;. This repository replaces the old [datastax/cass-operator](https://github.com/datastax/cass-operator) for use-cases in the k8ssandra project. Some documentation is still out of date and will be modified in the future. Check [k8ssandra/k8ssandra](https://github.com/k8ssandra/k8ssandra) for more up to date information.

## Getting Started

To create a full featured cluster, the recommend approach is to use the Helm charts from k8ssandra. Check the [Getting started](https://k8ssandra.io/docs/getting-started/) documentation at (k8ssandra.io)[https://k8ssandra.io/docs].

Quick start:
```console
# *** This is for GKE Regular Channel - k8s 1.16 -> Adjust based on your cloud or storage options
kubectl create -f https://raw.githubusercontent.com/k8ssandra/cass-operator/v1.7.1/docs/user/cass-operator-manifests.yaml
kubectl create -f https://raw.githubusercontent.com/k8ssandra/cass-operator/v1.7.1/operator/k8s-flavors/gke/storage.yaml
kubectl -n cass-operator create -f https://raw.githubusercontent.com/k8ssandra/cass-operator/v1.7.1/operator/example-cassdc-yaml/cassandra-3.11.x/example-cassdc-minimal.yaml
```

### Loading the operator

Installing the Cass Operator itself is straightforward. Apply the relevant manifest to your cluster as follows:

```console
kubectl apply -f https://raw.githubusercontent.com/k8ssandra/cass-operator/v1.7.1/docs/user/cass-operator-manifests.yaml
```

Note that since the manifest will install a [Custom Resource Definition](https://kubernetes.io/docs/concepts/extend-kubernetes/api-extension/custom-resources/), the user running the above command will need cluster-admin privileges.

This will deploy the operator, along with any requisite resources such as Role, RoleBinding, etc., to the `cass-operator` namespace. You can check to see if the operator is ready as follows:

```console
$ kubectl -n cass-operator get pods --selector name=cass-operator
NAME                             READY   STATUS    RESTARTS   AGE
cass-operator-555577b9f8-zgx6j   1/1     Running   0          25h
```

### Creating a storage class

You will need to create an appropriate storage class which will define the type of storage to use for Cassandra nodes in a cluster. For example, here is a storage class for using SSDs in GKE, which you can also find at [operator/deploy/k8s-flavors/gke/storage.yaml](operator/k8s-flavors/gke/storage.yaml):

```yaml
apiVersion: storage.k8s.io/v1
kind: StorageClass
metadata:
  name: server-storage
provisioner: kubernetes.io/gce-pd
parameters:
  type: pd-ssd
  replication-type: none
volumeBindingMode: WaitForFirstConsumer
reclaimPolicy: Delete
```

Apply the above as follows:

```
kubectl apply -f https://raw.githubusercontent.com/k8ssandra/cass-operator/v1.7.1/operator/k8s-flavors/gke/storage.yaml
```

### Creating a CassandraDatacenter

The following resource defines a Cassandra 3.11.7 datacenter with 3 nodes on one rack, which you can also find at [operator/example-cassdc-yaml/cassandra-3.11.x/example-cassdc-minimal.yaml](operator/example-cassdc-yaml/cassandra-3.11.x/example-cassdc-minimal.yaml):

```yaml
apiVersion: cassandra.datastax.com/v1beta1
kind: CassandraDatacenter
metadata:
  name: dc1
spec:
  clusterName: cluster1
  serverType: cassandra
  serverVersion: 3.11.7
  managementApiAuth:
    insecure: {}
  size: 3
  storageConfig:
    cassandraDataVolumeClaimSpec:
      storageClassName: server-storage
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 5Gi
  config:
    cassandra-yaml:
      authenticator: org.apache.cassandra.auth.PasswordAuthenticator
      authorizer: org.apache.cassandra.auth.CassandraAuthorizer
      role_manager: org.apache.cassandra.auth.CassandraRoleManager
    jvm-options:
      initial_heap_size: 800M
      max_heap_size: 800M
```

Apply the above as follows:

```console
kubectl -n cass-operator apply -f https://raw.githubusercontent.com/k8ssandra/cass-operator/v1.7.1/operator/example-cassdc-yaml/cassandra-3.11.x/example-cassdc-minimal.yaml
```

You can check the status of pods in the Cassandra cluster as follows:

```console
$ kubectl -n cass-operator get pods --selector cassandra.datastax.com/cluster=cluster1
NAME                         READY   STATUS    RESTARTS   AGE
cluster1-dc1-default-sts-0   2/2     Running   0          26h
cluster1-dc1-default-sts-1   2/2     Running   0          26h
cluster1-dc1-default-sts-2   2/2     Running   0          26h
```

You can check to see the current progress of bringing the Cassandra datacenter online by checking the `cassandraOperatorProgress` field of the `CassandraDatacenter`'s `status` sub-resource as follows:

```console
$ kubectl -n cass-operator get cassdc/dc1 -o "jsonpath={.status.cassandraOperatorProgress}"
Ready
```

(`cassdc` and `cassdcs` are supported short forms of `CassandraDatacenter`.)

A value of "Ready", as above, means the operator has finished setting up the Cassandra datacenter.

You can also check the Cassandra cluster status using `nodetool` by invoking it on one of the pods in the Cluster as follows:

```console
$ kubectl -n cass-operator exec -it -c cassandra cluster1-dc1-default-sts-0 -- nodetool status
Datacenter: dc1
===============
Status=Up/Down
|/ State=Normal/Leaving/Joining/Moving/Stopped
--  Address         Load       Tokens       Owns (effective)  Host ID                               Rack
UN  10.233.105.125  224.82 KiB  1            65.4%             5e29b4c9-aa69-4d53-97f9-a3e26115e625  r1
UN  10.233.92.96    186.48 KiB  1            61.6%             b119eae5-2ff4-4b06-b20b-c492474e59a6  r1
UN  10.233.90.54    205.1 KiB   1            73.1%             0a96e814-dcf6-48b9-a2ca-663686c8a495  r1
```

The operator creates a secure Cassandra cluster by default, with a new superuser (not the traditional `cassandra` user) and a random password. You can get those out of a Kubernetes secret and use them to log into your Cassandra cluster for the first time. For example:

```console
$ # get CASS_USER and CASS_PASS variables into the current shell
$ CASS_USER=$(kubectl -n cass-operator get secret cluster1-superuser -o json | jq -r '.data.username' | base64 --decode)
$ CASS_PASS=$(kubectl -n cass-operator get secret cluster1-superuser -o json | jq -r '.data.password' | base64 --decode)
$ kubectl -n cass-operator exec -ti cluster1-dc1-default-sts-0 -c cassandra -- sh -c "cqlsh -u '$CASS_USER' -p '$CASS_PASS'"

Connected to cluster1 at 127.0.0.1:9042.
[cqlsh 5.0.1 | Cassandra 3.11.6 | CQL spec 3.4.4 | Native protocol v4]
Use HELP for help.

cluster1-superuser@cqlsh> select * from system.peers;

 peer      | data_center | host_id                              | preferred_ip | rack    | release_version | rpc_address | schema_version                       | tokens
-----------+-------------+--------------------------------------+--------------+---------+-----------------+-------------+--------------------------------------+--------------------------
 10.28.0.4 |         dc1 | 4bf5e110-6c19-440e-9d97-c013948f007c |         null | default |          3.11.6 |   10.28.0.4 | e84b6a60-24cf-30ca-9b58-452d92911703 | {'-7957039572378599263'}
 10.28.5.5 |         dc1 | 3e84b0f1-9c1e-4deb-b6f8-043731eaead4 |         null | default |          3.11.6 |   10.28.5.5 | e84b6a60-24cf-30ca-9b58-452d92911703 | {'-3984092431318102676'}

(2 rows)
```

### Installing cluster via Helm

To install a cluster with optional integrated backup/restore and repair utilities, check the [k8ssandra/k8ssandra](https://github.com/k8ssandra/k8ssandra) helm charts project. 

If you wish to install only the cass-operator, you can run the following command:

```
helm repo add k8ssandra https://helm.k8ssandra.io/stable
helm install k8ssandra k8ssandra/k8ssandra --set cassandra.enabled=false --set reaper.enabled=false --set reaper-operator.enabled=false --set stargate.enabled=false --set kube-prometheus-stack.enabled=false
```

You can then apply your CassandraDatacenter.

##### Custom Docker registry example: Github packages

Github Packages may be used as a custom Docker registry.

First, a Github personal access token must be created.

See:

https://docs.github.com/en/github/authenticating-to-github/creating-a-personal-access-token

Second, the access token will be used to create the Secret:

```console
kubectl create secret docker-registry github-docker-registry --docker-username=USERNAME --docker-password=ACCESSTOKEN --docker-server docker.pkg.github.com
```

Replace USERNAME with the github username and ACCESSTOKEN with the personal access token.

## Features

- Proper token ring initialization, with only one node bootstrapping at a time
- Seed node management - one per rack, or three per datacenter, whichever is more
- Server configuration integrated into the CassandraDatacenter CRD
- Rolling reboot nodes by changing the CRD
- Store data in a rack-safe way - one replica per cloud AZ
- Scale up racks evenly with new nodes
- Scale down racks evenly by decommissioning existing nodes
- Replace dead/unrecoverable nodes
- Multi DC clusters (limited to one Kubernetes namespace)

All features are documented in the [User Documentation](docs/user/README.md).

### Containers

The operator is comprised of the following container images working in concert:
* The operator, built from sources in the [operator](operator/) directory.
* The config builder init container, built from sources in [datastax/cass-config-builder](https://github.com/datastax/cass-config-builder).
* Cassandra, built from
  [datastax/management-api-for-apache-cassandra](https://github.com/datastax/management-api-for-apache-cassandra),
  with Cassandra 3.11.7 support, and experimental support for Cassandra
  4.0-beta1.
* ... or DSE, built from [datastax/docker-images](https://github.com/datastax/docker-images).

### Overriding properties of cass-operator created Containers

If the CassandraDatacenter specifies a podTemplateSpec field, then containers with specific names can be used to override default settings in containers that will be created by cass-operator.

Currently cass-operator will create an InitContainer with the name of "server-config-init". Normal Containers that will be created have the names "cassandra", "server-system-logger", and optionally "reaper". 

In general, the values specified in this way by the user will override anything generated by cass-operator.

Of special note is that user-specified environment variables, ports, and volumes in the corresponding containers will be added to the values that cass-operator automatically generates for those containers.

```yaml
apiVersion: cassandra.datastax.com/v1beta1
kind: CassandraDatacenter
metadata:
  name: dc1
spec:
  clusterName: cluster1
  serverType: cassandra
  serverVersion: 3.11.7
  managementApiAuth:
    insecure: {}
  size: 3
  podTemplateSpec:
    spec:
      initContainers:
        - name: "server-config-init"
          env:
          - name: "EXTRA_PARAM"
            value: "123"
      containers:
        - name: "cassandra"
          terminationMessagePath: "/dev/other-termination-log"
          terminationMessagePolicy: "File"
  storageConfig:
    cassandraDataVolumeClaimSpec:
      storageClassName: server-storage
      accessModes:
      - ReadWriteOnce
      resources:
        requests:
          storage: 5Gi
  config:
    cassandra-yaml:
      authenticator: org.apache.cassandra.auth.PasswordAuthenticator
      authorizer: org.apache.cassandra.auth.CassandraAuthorizer
      role_manager: org.apache.cassandra.auth.CassandraRoleManager
    jvm-options:
      initial_heap_size: 800M
      max_heap_size: 800M
```

## Requirements

- Kubernetes cluster, 1.16 or newer.

## Contributing

If you wish to file a bug, enhancement proposal or have other questions, use the issues in repository [k8ssandra/k8ssandra](https://github.com/k8ssandra/k8ssandra). PRs should target this repository and you can link the PR to issue repository with ``k8ssandra/k8ssandra#ticketNumber`` syntax.

For other means of contacting, check [k8ssandra community](https://k8ssandra.io/community/) resources.

### Developer setup

Almost every build, test, or development task requires the following
pre-requisites...

* Golang 1.15 or newer
* Docker, either the docker.io packages on Ubuntu, Docker Desktop for Mac,
  or your preferred docker distribution.
* [mage](https://magefile.org/): There are some tips for using mage in
  [docs/developer/mage.md](docs/developer/mage.md)

### Building

The operator uses [mage](https://magefile.org/) for its build process.

#### Build the Operator Container Image
This build task will create the operator container image, building or rebuilding
the binary from golang sources if necessary:

``` bash
mage operator:buildDocker
```

#### Build the Operator Binary
If you wish to perform ONLY to the golang build or rebuild, without creating
a container image:

``` bash
mage operator:buildGo
```

### Testing

``` bash
mage operator:testGo
```

#### End-to-end Automated Testing

Run fully automated end-to-end tests...

```bash
mage integ:run
```

Docs about testing are [here](tests/README.md). These work against any k8s
cluster with six or more worker nodes.

#### Manual Local Testing
There are a number of ways to run the operator, see the following docs for
more information:
* [k8s targets](docs/developer/k8s_targets.md): A set of mage targets for
  automating a variety of tasks for several different supported k8s flavors.
  At the moment, we support KIND, k3d, and gke. These targets can setup and
  manage a local cluster in either KIND or k3d, and also a remote cluster
  in gke. Both KIND and k3d can simulate a k8s cluster with multiple worker 
  nodes on a single physical machine, though it's necessary to dial down 
  the database memory requests.

The [user documentation](docs/user/README.md) also contains information on
spinning up your first operator instance that is useful regardless of what
Kubernetes distribution you're using to do so.

## Uninstall

*This will destroy all of your data!*

Delete your CassandraDatacenters first, otherwise Kubernetes will block deletion because we use a finalizer.
```
kubectl delete cassdcs --all-namespaces --all
```

Remove the operator Deployment, CRD, etc.
```
kubectl delete -f https://raw.githubusercontent.com/k8ssandra/cass-operator/v1.7.1/docs/user/cass-operator-manifests.yaml
```

## Contacts

For development questions, please reach out on [Development mailing list](https://groups.google.com/g/k8ssandra-developers), or by opening an issue on [k8ssandra/k8ssandra](https://github.com/k8ssandra/k8ssandra) GitHub repository.

For usage questions, please visit our [User mailing list](https://groups.google.com/g/k8ssandra-users).

## License

Copyright DataStax, Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the License. You may obtain a copy of the License at

http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific language governing permissions and limitations under the License.
