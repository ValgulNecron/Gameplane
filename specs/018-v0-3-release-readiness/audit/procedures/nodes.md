# Procedures: NODE

Shared conventions: [conventions.md](../conventions.md).

### scheduling

Records the current scheduling state of audit018- GameServers: which pod is assigned to which node. This baseline is used to verify that node eviction and node loss recover pods correctly.

**Preconditions**

- `audit018-` GameServers are running on kubelab
- All nodes are in Ready state: `kubectl get nodes`
- Multiple audit018- servers exist (from prior test rounds)

**Resources created**

None.

**Steps**

1. **List all audit018- GameServers and their pod assignments**:
   ```sh
   mkdir -p ~/gameplane-audit-018/scheduling
   
   kubectl get gameserver -n gameplane-games -l gameplane.io/audit=018 -o wide > ~/gameplane-audit-018/scheduling/gameservers.txt
   
   # Pods are matched by the audit018- name prefix: operator/internal/controller/gameserver_controller.go
   # (buildStatefulSet) sets the pod template's labels to a fixed set
   # (app.kubernetes.io/name, app.kubernetes.io/instance, gameplane.local/template)
   # and never copies a GameServer's own labels onto its pod, so
   # -l gameplane.io/audit=018 never matches a pod.
   kubectl get pod -n gameplane-games -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName,PHASE:.status.phase | awk 'NR==1 || $1 ~ /^audit018-/' > ~/gameplane-audit-018/scheduling/pods.txt
   
   # Create a mapping: for each audit018- pod, record its node
   for pod in $(kubectl get pod -n gameplane-games -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n' | grep '^audit018-'); do
     node=$(kubectl get pod "$pod" -n gameplane-games -o jsonpath='{.spec.nodeName}')
     echo "$pod -> $node" >> ~/gameplane-audit-018/scheduling/pod-node-map.txt
   done
   
   cat ~/gameplane-audit-018/scheduling/pod-node-map.txt
   ```

2. **Record current node state**:
   ```sh
   kubectl get nodes -o wide > ~/gameplane-audit-018/scheduling/nodes.txt
   
   for node in $(kubectl get nodes -o jsonpath='{.items[*].metadata.name}'); do
     echo "Node: $node"
     kubectl describe node "$node" | grep -A 5 "Allocatable\|Allocated resources" >> ~/gameplane-audit-018/scheduling/node-state.txt
   done
   ```

3. **Save evidence**:
   ```sh
   cp ~/gameplane-audit-018/scheduling/*.txt ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-NODE-001/
   ```

**Expected**

- At least 2 audit018- GameServers are running across the cluster
- Each pod has a scheduled node assignment
- Node state files capture resource allocation
- Mapping file is human-readable and complete

**Cleanup**

- Mapping files are kept for the drain and node-loss tests
- No cluster state is modified

**Automatable?**

Yes. Bucket: `operator`.

---

### drain

Tests node drain behavior by cordoning a node, evicting only the audit018- pod(s) on that node via the Kubernetes eviction API, and verifying that:
- The pod is evicted and goes into Terminating state
- The operator recreates the pod on a different node
- The pre-existing pods on that node are **not** touched (no kubectl drain of the full node)

**Preconditions**

- audit018- pods are running on multiple nodes (from scheduling step)
- At least one node holds at least one audit018- pod
- Pod-to-node mapping is recorded in `~/gameplane-audit-018/scheduling/pod-node-map.txt`

**Resources created**

None (audit018- pods already exist).

**Steps**

1. **Select a target node** (one with at least one audit018- pod):
   ```sh
   TARGET_NODE=$(kubectl get pod -n gameplane-games -o jsonpath='{range .items[*]}{.metadata.name}{" "}{.spec.nodeName}{"\n"}{end}' | awk '$1 ~ /^audit018-/ {print $2; exit}')
   echo "Target node: $TARGET_NODE"
   ```

2. **Record pre-drain state**:
   ```sh
   kubectl get pod -n gameplane-games -o wide > ~/gameplane-audit-018/scheduling/pre-drain.txt
   ```

3. **Cordon the node**:
   ```sh
   kubectl cordon "$TARGET_NODE"
   kubectl describe node "$TARGET_NODE" | grep -A 1 "SchedulingDisabled"
   ```

4. **Evict each audit018- pod on the target node** (via eviction API, not force delete):
   ```sh
   # audit018- pods on the target node, matched by name prefix (the
   # operator does not propagate a GameServer's own labels onto its pod,
   # so pods cannot be selected with -l gameplane.io/audit=018)
   PODS=$(kubectl get pod -n gameplane-games --field-selector spec.nodeName="$TARGET_NODE" -o jsonpath='{.items[*].metadata.name}' | tr ' ' '\n' | grep '^audit018-')
   
   # kubectl has no "evict" subcommand; POST to the real Eviction API instead
   for pod in $PODS; do
     echo "Evicting pod: $pod from node $TARGET_NODE"
     printf '{"apiVersion":"policy/v1","kind":"Eviction","metadata":{"name":"%s","namespace":"gameplane-games"}}' "$pod" \
       | kubectl create --raw "/api/v1/namespaces/gameplane-games/pods/$pod/eviction" -f -
   done
   ```

5. **Wait for eviction to complete**:
   ```sh
   # Verify old pods are gone or Terminating
   kubectl get pod -n gameplane-games --field-selector spec.nodeName="$TARGET_NODE" -w &
   WATCH_PID=$!
   sleep 20
   kill $WATCH_PID || true
   
   # Pods should either be gone or Terminating
   for pod in $PODS; do
     kubectl get pod "$pod" -n gameplane-games -o jsonpath='{.metadata.name}={.status.phase}{"\n"}' 2>/dev/null || echo "$pod=Gone"
   done
   ```

6. **Verify new pods are created on other nodes**:
   ```sh
   kubectl get pod -n gameplane-games -o wide | awk 'NR==1 || $1 ~ /^audit018-/' > ~/gameplane-audit-018/scheduling/post-eviction.txt
   
   # Check that the audit018- pods are now on different nodes
   cat ~/gameplane-audit-018/scheduling/post-eviction.txt | grep -v "$TARGET_NODE" | head -5
   ```

7. **Verify pre-existing pods on target node are untouched**:
   ```sh
   # List all pods on the target node (not just audit018-)
   kubectl get pod -n gameplane-games --field-selector spec.nodeName="$TARGET_NODE" -o wide > ~/gameplane-audit-018/scheduling/target-node-pods-after.txt
   
   # Compare with pre-drain (pre-existing pods should still be there)
   cat ~/gameplane-audit-018/scheduling/target-node-pods-after.txt
   ```

8. **Uncordon the node**:
   ```sh
   kubectl uncordon "$TARGET_NODE"
   kubectl describe node "$TARGET_NODE" | grep -A 1 "SchedulingDisabled"
   ```

9. **Save evidence**:
   ```sh
   cp ~/gameplane-audit-018/scheduling/*.txt ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-NODE-002/
   ```

**Expected**

- audit018- pods on the target node are evicted and recreated on other nodes
- Pod nodes change in the post-eviction snapshot compared to pre-drain
- Pre-existing pods (non-audit018-) on the target node remain Running (not evicted)
- Cordon/uncordon operations complete without errors
- No full kubectl drain is executed

**Cleanup**

- Node is uncordoned and back to SchedulingEnabled
- Evicted pods are recreated by the operator
- New pod assignments are recorded for node-loss test

**Automatable?**

Yes. Bucket: `operator`.

---

### node-loss

Tests real node failure by stopping the k3s-agent on a worker node for a few minutes, then restarting it. Verifies that:
- Pods on that node transition to a lost state
- The operator reschedules them on surviving nodes after the node rejoins
- Pre-existing stateful game servers are unaffected (node is selected to hold none)

**Preconditions**

- Multiple nodes are in Ready state and running audit018- pods
- Target worker is `kubelab-worker-2`, per OD-017 (RESOLVED 2026-09-24): it holds the pre-existing `soak-pool-west-0`, which is accepted to go down with the node and must come back `Running` with the same UID and PVC once the node returns
- k3s control plane access or SSH is available to stop/restart k3s-agent on the target worker

**Resources created**

None.

**Steps**

1. **Identify target worker** (per OD-017, RESOLVED 2026-09-24):
   ```sh
   TARGET_WORKER=kubelab-worker-2
   echo "Target worker: $TARGET_WORKER (OD-017)"
   ```

2. **Record pre-loss state**:
   ```sh
   kubectl describe node "$TARGET_WORKER" > ~/gameplane-audit-018/node-loss/pre-loss-state.txt
   kubectl get pod -n gameplane-games --field-selector spec.nodeName="$TARGET_WORKER" -o wide | awk 'NR==1 || $1 ~ /^audit018-/' > ~/gameplane-audit-018/node-loss/pods-on-target.txt

   # soak-pool-west is pre-existing and stateful; per OD-017 it is expected
   # to go down with this node and must return with the same pod UID and PVC
   kubectl get pod soak-pool-west-0 -n gameplane-games -o jsonpath='{.metadata.uid}' > ~/gameplane-audit-018/node-loss/soak-pool-west-pod-uid-before.txt
   kubectl get pvc soak-pool-west-data -n gameplane-games -o jsonpath='{.metadata.uid}' > ~/gameplane-audit-018/node-loss/soak-pool-west-pvc-uid-before.txt
   ```

3. **Stop k3s-agent on the worker**:
   ```sh
   # This requires SSH or kubectl debug access to the target worker
   # k3s-agent runs as a systemd service on worker nodes
   kubectl debug node/"$TARGET_WORKER" -it --image=busybox -- \
     nsenter -t 1 -m -u -i -n systemctl stop k3s-agent
   
   # Verify the node is NotReady
   kubectl get nodes | grep "$TARGET_WORKER"
   # Should show NotReady,SchedulingDisabled
   ```

4. **Wait for pod eviction**:
   ```sh
   # Pods on a NotReady node transition to Unknown after ~5 minutes (graceful termination period)
   sleep 60
   kubectl get pod -n gameplane-games -o wide | awk 'NR==1 || $1 ~ /^audit018-/' > ~/gameplane-audit-018/node-loss/pods-during-loss.txt
   
   # Verify pods are Unknown or being evicted
   cat ~/gameplane-audit-018/node-loss/pods-during-loss.txt | grep "$TARGET_WORKER"
   ```

5. **Restart k3s-agent on the worker**:
   ```sh
   kubectl debug node/"$TARGET_WORKER" -it --image=busybox -- \
     nsenter -t 1 -m -u -i -n systemctl start k3s-agent
   
   # Wait for node to return to Ready
   kubectl wait --for=condition=Ready node/"$TARGET_WORKER" --timeout=300s
   kubectl get nodes | grep "$TARGET_WORKER"
   # Should show Ready,SchedulingEnabled
   ```

6. **Verify pod recovery**:
   ```sh
   # Pods should be rescheduled on other nodes or recreated
   kubectl get pod -n gameplane-games -o wide | awk 'NR==1 || $1 ~ /^audit018-/' > ~/gameplane-audit-018/node-loss/pods-after-recovery.txt
   
   # Compare with pre-loss mapping: pods should have different node assignments
   diff <(cat ~/gameplane-audit-018/node-loss/pods-on-target.txt | awk '{print $1}') \
        <(cat ~/gameplane-audit-018/node-loss/pods-after-recovery.txt | grep -v "$TARGET_WORKER" | awk '{print $1}')
   ```

7. **Verify pre-existing pods**:
   ```sh
   # soak-pool-west-0 was on the target worker and is expected to recover
   # Running with the same pod UID and PVC (OD-017); the rest never left
   # their nodes and must show unchanged
   kubectl get pod -n gameplane-games -o wide | grep -E "mc-fabric|soak-bogus-pool|squad|soak-no-preference"

   kubectl get pod soak-pool-west-0 -n gameplane-games -o jsonpath='{.status.phase}{"\n"}{.metadata.uid}{"\n"}'
   diff ~/gameplane-audit-018/node-loss/soak-pool-west-pod-uid-before.txt \
     <(kubectl get pod soak-pool-west-0 -n gameplane-games -o jsonpath='{.metadata.uid}')
   diff ~/gameplane-audit-018/node-loss/soak-pool-west-pvc-uid-before.txt \
     <(kubectl get pvc soak-pool-west-data -n gameplane-games -o jsonpath='{.metadata.uid}')
   ```

8. **Save evidence**:
   ```sh
   cp ~/gameplane-audit-018/node-loss/*.txt ~/Gameplane/specs/018-v0-3-release-readiness/audit/evidence/INV-NODE-003/
   ```

**Expected**

- k3s-agent stop/start cycle completes successfully on `kubelab-worker-2`
- Node transitions to NotReady and back to Ready
- audit018- pods on the lost node are rescheduled on surviving nodes
- `soak-pool-west` returns `Running` with the same pod UID and the same PVC (OD-017)
- `mc-fabric`, `soak-bogus-pool`, `soak-no-preference` and `squad` are unaffected throughout

**Cleanup**

- Node is back to Ready and SchedulingEnabled
- Evicted audit018- pods are recovered
- Pre-existing GameServers continue running
- Delete the `node-debugger-<node>-<random>` pods left by the two `kubectl debug node/"$TARGET_WORKER"` calls in Steps 3 and 5 (each invocation creates its own pod, in the current kubectl context's namespace, since neither call passed `-n`):
   ```sh
   DEBUG_NS=$(kubectl config view --minify -o jsonpath='{..namespace}')
   DEBUG_NS=${DEBUG_NS:-default}
   kubectl get pod -n "$DEBUG_NS" -o name | grep "^pod/node-debugger-${TARGET_WORKER}-" | xargs -r kubectl delete -n "$DEBUG_NS"
   ```

**Automatable?**

No — requires SSH/`kubectl debug` access to stop `k3s-agent` on a real node, which a Kind-based E2E cluster cannot do. Alternative (automatable): use the drain test (INV-NODE-002) instead, which tests cordon + eviction without full node loss.

