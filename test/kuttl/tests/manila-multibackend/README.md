# manila-multibackend kuttl test

The "multibackend" manila kuttl test is supposed to cover asserts related to the
main manila services deployed, as well as scaling up and scaling down multiple
`ManilaShares` instances connected to different backends.
To make things simpler and easy to test in the `openstack-k8s-operators` context,
two `ManilaShares` instances are connected to the same `Ceph` cluster using two
different protocols:

1. `share0` is connected to `Ceph` using the `native-CephFS` protocol
2. `share1` is connected to `Ceph` using the `CephNFS` protocol

The test starts deploying a basic `ManilaShare` where only `share0` is active
(step1).
The second step scales the ManilaShare instances enabling `share1` (`replicas: 1`):
the asserts checks that both shares are available and the `manila-share` containers
match what's defined in the `CSV`.
The third step scales `share0` down, and asserts that only share1 is available.
The use case tested by this scenario ends deleting the `manila` resources (the
main CR is removed) and checks that the environment is clean.

## Topology

The target topology for this test is:

1. one ManilaAPI object
2. one ManilaScheduler object
3. two ManilaShare objects: both are connected with a Ceph clusters with two different protocols:
   a. `share0` is connected to `Ceph` using the `native-CephFS` protocol
   b. `share1` is connected to `Ceph` using the `CephNFS` protocol

The manila-multibackend steps are supposed to cover scaling up and scaling down
ManilaShares (both `share0` and `share1`).

### Prerequisites

As a prerequisite for this test, we assume:

1. a running `Ceph` cluster (or a Ceph Pod deployed via the `install_yamls`
   `make ceph` command)
2. an existing `MariaDB/Galera` entity in the target namespace
3. an existing `Keystone` deployed via keystone-operator
4. an existing `RabbitMQ` cluster in the target namespace
5. a running `manila-operator` deployed via the `install_yamls` `make manila`
   target

These resources can be deployed via `install_yamls` using the `kuttl_common_prep`
target provided by the default Makefile.

### Run the manila-basic tests

Once the kuttl requirements are satisfied, the actual tests can be executed,
against an existing namespace, via the following command:

```
NAMESPACE=openstack
MANILA_OPERATOR=<GIT REPO WHERE MANILA-OPERATOR IS CLONED>
kubectl-kuttl test --config $MANILA_OPERATOR/kuttl-test.yaml \
   $MANILA_OPERATOR/test/kuttl/tests/ --namespace $NAMESPACE | tee -a $HOME/kuttl.log
```
