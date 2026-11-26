# Spike #523: Crossplane Package Retrieval Through K3s Spegel

**Question:** can Crossplane fetch packages stored in K3s containerd through
the embedded registry mirror (Spegel), so the separate local registry could be
removed (#524)?

**Answer: no, not directly; and a gateway (#524) would not fully close the
gap.** Spegel works well for what it is built for, distributing images between
containerd instances, which is relevant to #522.

Environment: K3s v1.36.2+k3s1 in Docker, one server and one agent, server
started with `--embedded-registry`, both with `registries.yaml` mirroring
`registry.overlock.svc.cluster.local` and `xpkg.crossplane.io`. Crossplane
2.5.0. Packages: `provider-nop:v0.5.0`, `function-auto-ready:v0.7.4`, imported
into the server node only. Reproduce with `hack/spikes/spegel-crossplane.sh`.

---

## Results

| Validation | Result |
|---|---|
| Direct OCI manifest retrieval | Works only with `?ns=<original registry>`: `/v2/acme/nop/manifests/v0.5.0?ns=registry.overlock.svc.cluster.local` → 200; without `ns` → 404, also with the original registry in the path |
| Tag resolution (`HEAD` manifest) | Works with `ns` (returns `Docker-Content-Digest`) |
| Blob retrieval | Works with `ns`, by digest |
| Tag listing (`/tags/list`) | Not supported (404) |
| Client certificate authentication | The server requests a client certificate but does not require one: manifests and blobs are served without a certificate, from the host and from any pod |
| `ns` handling | Required on every request; the request path is the repository without registry |
| Crossplane `ImageConfig` | Crossplane fails: `cannot resolve 172.20.0.2:6443/acme/nop:v0.5.0 to digest ... HEAD .../v2/acme/nop/manifests/v0.5.0` → 404. An `ImageConfig` can rewrite the path but cannot add `ns`; TLS trust through `registryCaBundleConfig` works |
| Function installation | Same failure as providers; nothing is unpacked |
| Dependency resolution | Not reachable (the package itself cannot be resolved). Version ranges would additionally need tag listing, which Spegel does not provide |
| Remote node (agent) | Any node serves the registry API; tags resolve through peers |
| Disconnected operation (upstream does not exist) | Pulled images are distributed between nodes without upstream. **Imported images are not**: the tag is advertised, the content digests are not (404 from other nodes), so pods on other nodes fail to pull |

### Imported images and Spegel

Content pulled by containerd is labelled with its origin
(`containerd.io/distribution.source.<registry>=<repository>`); imported content
(`ctr import`, including the format of #521) is not. Without the label, Spegel
advertises the tag but not the digests. After adding the label to the image's
manifest, config and layer blobs and restarting K3s, other nodes fetched the
image and a pod on the agent started from it. Spegel noticed live imports
immediately (`OCI event ... CREATE`), but did not pick up labels added later
without a restart.

---

## Conclusions

1. **Crossplane cannot use Spegel as a registry.** Crossplane's OCI client
   (go-containerregistry) sends standard registry requests; Spegel only answers
   requests carrying the non-standard `ns` parameter. Confirmed with `crane`
   (same library) and with Crossplane itself.
2. **A gateway (#524) could add `ns`, but would still be incomplete:** Spegel
   has no tag listing, so dependencies with version ranges could not be
   resolved, and package versions not yet present on any node cannot be
   fetched at all. The local registry would still be needed as the source and
   for these cases, so removing it is not achievable.
3. **Security:** Spegel on port 6443 serves every mirrored image in the
   cluster's containerd without client authentication, to anything that
   reaches the port, contrary to the K3s documentation. This includes images
   pulled with private registry credentials. Any setup enabling it must keep
   6443 unreachable from untrusted networks.
4. **For #522 (image distribution):** Spegel distributes pulled images between
   nodes without upstream, but imported images (what `load` produces) need the
   `distribution.source` labels on their content to be shared, ideally set at
   import time so no restart is needed.

## Recommendation

- **#524: do not implement.** The condition "only if the prototype
  demonstrates reliable behavior" is not met; keep the local registry.
- **#522: proceed with configuration only**, and add the
  `containerd.io/distribution.source.*` labels to imported content, verified
  with live imports, late-joining nodes and restarts.
