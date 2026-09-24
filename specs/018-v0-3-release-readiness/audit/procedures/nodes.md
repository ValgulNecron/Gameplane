# Procedures: NODE

Shared conventions: [conventions.md](../conventions.md).

## scheduling

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
   
   kubectl get pod -n gameplane-games -l gameplane.io/audit=018 -o custom-columns=NAME:.metadata.name,NODE:.spec.nodeName,PHASE:.status.phase > ~/gameplane-audit-018/scheduling/pods.txt
   
   # Create a mapping: for each audit018- pod, record its node
   for pod in $(kubectl get pod -n gameplane-games -l gameplane.io/audit=018 -o jsonpath='{.items[*].metadata.name}'); do
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
   cp ~/gameplane-audit-018/scheduling/*.txt ~/Gameplane/audit/evidence/INV-NODE-001/
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

## drain

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
   TARGET_NODE=$(kubectl get pod -n gameplane-games -l gameplane.io/audit=018 -o jsonpath='{.items[0].spec.nodeName}')
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
   # Get audit018- pods on the target node
   PODS=$(kubectl get pod -n gameplane-games -l gameplane.io/audit=018 --field-selector spec.nodeName="$TARGET_NODE" -o jsonpath='{.items[*].metadata.name}')
   
   for pod in $PODS; do
     echo "Evicting pod: $pod from node $TARGET_NODE"
     kubectl evict "$pod" -n gameplane-games --ignore-errors --delete-empty-dir-data
   done
   ```

5. **Wait for eviction to complete**:
   ```sh
   # Verify old pods are gone or Terminating
   kubectl get pod -n gameplane-games -l gameplane.io/audit=018 --field-selector spec.nodeName="$TARGET_NODE" -w &
   WATCH_PID=$!
   sleep 20
   kill $WATCH_PID || true
   
   # Pods should either be gone or Terminating
   kubectl get pod -n gameplane-games -l gameplane.io/audit=018 --field-selector spec.nodeName="$TARGET_NODE" -o jsonpath='{.items[*].status.phase}'
   ```

6. **Verify new pods are created on other nodes**:
   ```sh
   kubectl get pod -n gameplane-games -l gameplane.io/audit=018 -o wide > ~/gameplane-audit-018/scheduling/post-eviction.txt
   
   # Check that the audit018- pods are now on different nodes
   cat ~/gameplane-audit-018/scheduling/post-eviction.txt | grep -v "$TARGET_NODE" | head -5
   ```

7. **Verify pre-existing pods on target node are untouched**:
   ```sh
   # List all pods on the target node (not just audit018-)
   kubectl get pod -n gameplane-games -A --field-selector spec.nodeName="$TARGET_NODE" -o wide > ~/gameplane-audit-018/scheduling/target-node-pods-after.txt
   
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
   cp ~/gameplane-audit-018/scheduling/*.txt ~/Gameplane/audit/evidence/INV-NODE-002/
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

## node-loss

Tests real node failure by stopping the k3s-agent on a worker node for a few minutes, then restarting it. Verifies that:
- Pods on that node transition to a lost state
- The operator reschedules them on surviving nodes after the node rejoins
- Pre-existing stateful game servers are unaffected (node is selected to hold none)

**Preconditions**

- Multiple nodes are in Ready state and running audit018- pods
- A worker node has been identified that holds **no pre-existing stateful game servers** (per OD-006, OD-017)
- k3s control plane access or SSH is available to stop/restart k3s-agent on the target worker

**Resources created**

None.

**Steps**

1. **Identify target worker** (must hold no pre-existing stateful GameServers):
   ```sh
   # Per OD-017 as of 2026-09-23, no worker is currently free of pre-existing servers.
   # Wait for maintainer guidance on which node and time window to use.
   # Blocked: see OD-017 for available options.
   ```

2. **[If node is approved] Record pre-loss state**:
   ```sh
   TARGET_WORKER=kubelab-worker-N  # to be determined
   
   kubectl describe node "$TARGET_WORKER" > ~/gameplane-audit-018/node-loss/pre-loss-state.txt
   kubectl get pod -n gameplane-games -l gameplane.io/audit=018 --field-selector spec.nodeName="$TARGET_WORKER" -o wide > ~/gameplane-audit-018/node-loss/pods-on-target.txt
   ```

3. **[If node is approved] Stop k3s-agent on the worker**:
   ```sh
   # This requires SSH or kubectl debug access to the target worker
   # k3s-agent runs as a systemd service on worker nodes
   kubectl debug node/"$TARGET_WORKER" -it --image=busybox -- \
     nsenter -t 1 -m -u -i -n systemctl stop k3s-agent
   
   # Verify the node is NotReady
   kubectl get nodes | grep "$TARGET_WORKER"
   # Should show NotReady,SchedulingDisabled
   ```

4. **[If node is approved] Wait for pod eviction**:
   ```sh
   # Pods on a NotReady node transition to Unknown after ~5 minutes (graceful termination period)
   sleep 60
   kubectl get pod -n gameplane-games -l gameplane.io/audit=018 -o wide > ~/gameplane-audit-018/node-loss/pods-during-loss.txt
   
   # Verify pods are Unknown or being evicted
   cat ~/gameplane-audit-018/node-loss/pods-during-loss.txt | grep "$TARGET_WORKER"
   ```

5. **[If node is approved] Restart k3s-agent on the worker**:
   ```sh
   kubectl debug node/"$TARGET_WORKER" -it --image=busybox -- \
     nsenter -t 1 -m -u -i -n systemctl start k3s-agent
   
   # Wait for node to return to Ready
   kubectl wait --for=condition=Ready node/"$TARGET_WORKER" --timeout=300s
   kubectl get nodes | grep "$TARGET_WORKER"
   # Should show Ready,SchedulingEnabled
   ```

6. **[If node is approved] Verify pod recovery**:
   ```sh
   # Pods should be rescheduled on other nodes or recreated
   kubectl get pod -n gameplane-games -l gameplane.io/audit=018 -o wide > ~/gameplane-audit-018/node-loss/pods-after-recovery.txt
   
   # Compare with pre-loss mapping: pods should have different node assignments
   diff <(cat ~/gameplane-audit-018/node-loss/pods-on-target.txt | awk '{print $1}') \
        <(cat ~/gameplane-audit-018/node-loss/pods-after-recovery.txt | grep -v "$TARGET_WORKER" | awk '{print $1}')
   ```

7. **[If node is approved] Verify pre-existing pods on other nodes**:
   ```sh
   # Confirm no pre-existing stateful servers were lost (e.g., mc-fabric, soak-pool-west)
   kubectl get pod -n gameplane-games -o wide | grep -E "mc-fabric|soak-pool-west|soak-bogus-pool|squad|soak-no-preference"
   # All should still be Running (not on target worker)
   ```

8. **[If node is approved] Save evidence**:
   ```sh
   cp ~/gameplane-audit-018/node-loss/*.txt ~/Gameplane/audit/evidence/INV-NODE-003/
   ```

**Expected**

- Target worker is identified per OD-017 maintainer input
- k3s-agent stop/start cycle completes successfully
- Node transitions to NotReady and back to Ready
- audit018- pods on the lost node are rescheduled on surviving nodes
- Pre-existing stateful game servers on other nodes remain unaffected

**Cleanup**

- Node is back to Ready and SchedulingEnabled
- Evicted audit018- pods are recovered
- Pre-existing GameServers continue running

**Automatable?**

Blocked pending OD-017. Alternative (automatable): use the drain test (INV-NODE-002) instead, which tests cordon + eviction without full node loss.

