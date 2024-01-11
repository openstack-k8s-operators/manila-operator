# manila-basic kuttl test

The "basic" manila kuttl test is supposed to cover asserts related to the main
manila services deployed in the most common use case scenario.

## Topology

The target topology for this test is:

1. one ManilaAPI object
2. one ManilaScheduler object
3. one ManilaShare object connected with a Ceph backend: the protocol used for
   this use case is native CephFS

The manila-basic steps are supposed to cover scaling up and scaling down the
ManilaAPI service.

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
