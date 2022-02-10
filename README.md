# Configset

Configset is a tool for managing a set of configs.

It can be used either as a plugin with nearly identical usage to `kubectl apply`, or as a library to power your own tools.

It features a similar release-like concept from Helm, which we call a config set. A set of configs is applied to Kubernetes as a unit, so that you can query existing config sets and delete them as a whole with a config set name. Resources that do not appear in the configs from a later apply can be automatically pruned from the cluster.

To install or upgrade a config set, run `kubectl configset apply` with the name of the config set and the source of configs:

```
kubectl configset apply myapp -f configs/ -n some-namespace
```

To delete it you only need the config set name:

```
kubectl configset delete myapp -n some-namespace
```

How is this superior than `kubectl apply` and Helm? Here is why:

- Configset fully utilizes the [server-side apply feature](https://kubernetes.io/docs/reference/using-api/server-side-apply/) introduced lately by Kubernetes, letting the apiserver do most of the validating and patching, which is more accurate than a purely client-side implementation.

- Because of this, unlike Helm, configset doesn't need to store the full content of the last applied configs somewhere, as they are not needed under server-side apply mode. Instead, configset only stores some metadata like related resources' GVK, namespace, name and uid - all it needs to implement resource pruning.

- Also, thanks to the server-side apply, implementing diff is much simpler, and the result is more accurate. Configset has a similar diff feature like `kubectl diff` to help compare the changes before persisting. Just use the `--diff` flag on `kubectl configset apply` or `kubectl configset delete` command.
