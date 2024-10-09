## Troubleshoot

### Freezing during Environment Creation with Overlock CLI

#### Symptom
The process freezes for a few minutes during the "Joining worker nodes" step when creating multiple environments with Overlock CLI. Eventually, it fails with the following error:
`ERROR: failed to create cluster: failed to join node with kubeadm: command "docker exec --privileged dest-worker kubeadm join --config /kind/kubeadm.conf --skip-phases=preflight --v=6" failed with error: exit status 1`


#### Cause
When the Overlock CLI creates environments, it also installs resources, likely increasing the number of file system watches (inotify instances) that Kubernetes and its components need to manage. This increased usage, combined with existing watches from previous Overlock environments, could exceed the default system limits, leading to the kubelet.service on the newly created worker node failing to start due to the error: `Failed to allocate directory watch: Too many open files.`

#### Steps to Resolve
1. Run the following command to adjust the `fs.inotify.max_user_instances` setting on your host:
`sysctl fs.inotify.max_user_instances=512`
2. Retry the `overlock env create` command.


#### Explanation
- **Why did the error occur?**  
The error indicates that the kubelet.service failed to start due to the system reaching its limit for the number of file system watches (`inotify` instances) allowed per user.

- **How did adjusting `fs.inotify.max_user_instances` solve the error?**  
Increasing the `fs.inotify.max_user_instances` setting allows more `inotify` instances to be allocated per user, resolving the resource limitation that caused the kubelet.service to fail.
