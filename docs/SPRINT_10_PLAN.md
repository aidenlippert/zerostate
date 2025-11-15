# Sprint 10: Milestone Escrow Activation

**Sprint Goal**: Launch milestone-based escrow with conditional payouts, oracle hooks, and developer-facing controls.

**Status**: Planning → Ready
**Start Window**: Week of 17 Nov 2025
**Duration**: 1 week
**Previous Sprint**: [Sprint 9 – ARI-first execution + runtime registry](./SPRINT_9_COMPLETE.md)

---

## Objectives

### 1. Milestone Escrow Engine (P0 🔴)
Build the plumbing that turns pallet support into orchestrator behavior.

**Tasks**
- [ ] Extend `TaskMilestone` with `paid`, `paid_at`, `tx_hash`.
- [ ] Implement `releaseMilestonePayments` / `refundUncompletedMilestones` (blockchain + payment manager fallback).
- [ ] Persist per-milestone events in `PaymentLifecycleManager`.
- [ ] Add helper to reconcile pallet events -> local state (temporary poller via `escrowClient.GetEscrow`).

**Acceptance**
- Milestone marked `approved` automatically transitions to `paid` (with tx hash) once pallet releases funds.
- Payment metrics show partial releases.
- Failed/canceled tasks refund only the outstanding milestones.

---

### 2. API & CLI Surfaces (P0 🔴)
Expose milestone workflows to product + partners.

**Deliverables**
- [ ] `POST /api/v1/tasks/milestone` – create milestone task (body: base task + milestones + budget + conditions).
- [ ] `POST /api/v1/tasks/:task_id/milestones/:idx/complete` – runtime reports completion (optional evidence payload).
- [ ] `POST /api/v1/tasks/:task_id/milestones/:idx/approve` – payer/committee approval with DID + signature.
- [ ] `GET /api/v1/tasks/:task_id/milestones` – list milestone state, approvals, payouts, on-chain hashes.
- [ ] CLI helper (`tools/milestone-demo.go`) to script the flow end-to-end.

**Acceptance**
- Routes protected via auth middleware; RBAC: payer, agent, committee.
- Integration tests cover success + error paths (missing DID, double approval, invalid index).

---

### 3. Conditional & Oracle Hooks (P1 🟡)
Allow milestone release only when real-world conditions are satisfied.

**Tasks**
- [ ] Define `Condition` struct (type, operator, target, oracle URL).
- [ ] Implement oracle adapter stub that signs verdicts (HTTP + DID signature).
- [ ] Support timeout auto-refund + manual override.

**Acceptance**
- Demo: milestone released only when oracle returns `accuracy >= 0.9`.
- Timeout path refunds milestone automatically if condition unmet by deadline.

---

### 4. Observability & Docs (P1 🟡)
Make milestone activity transparent.

**Deliverables**
- [ ] Prometheus metrics: `ainur_milestones_total`, `ainur_milestones_paid`, `ainur_milestone_value_ainu`, `ainur_milestone_refunds_total`.
- [ ] Grafana panel “Milestone Escrow” showing per-status counts + recent payouts.
- [ ] Docs:
  - `developer/milestone-escrow-guide.md` (API walkthrough, JSON schema, curl snippets).
  - `core-technical/05.5_warden_verification.md` (TEE+ZK proofs for milestone validation).
- [ ] Update `GETTING_STARTED.md` with “Run a milestone workflow”.

**Acceptance**
- Metrics visible locally (`/metrics`) and on devnet dashboards.
- Docs reviewed (prestige tone, no casual phrasing).

---

### 5. Testing & Rollout (P1 🟡)

**Test Matrix**
- SQLite + mock escrow (unit + integration).
- `chain-v2` devnet with pallet-escrow (milestones add → approve → release).
- Failure modes: partial approvals, oracle failure, dispute escalation.

**Rollout**
- Feature flag `ENABLE_MILESTONE_ESCROW`.
- Demo runbook + recorded CLI session.
- Sprint notes + release post summarizing the flow.

---

## Technical Approach

1. **Data Model**
   - Add `Paid bool`, `PaidAt *time.Time`, `PayoutTxHash string` to `TaskMilestone`.
   - store milestone snapshots in DB (task queue + analytics) for auditability.

2. **Escrow Client**
   - Reuse pallet calls: `create_escrow`, `add_milestone`, `complete_milestone`, `approve_milestone`.
   - If pallet v1.1 exposes `release_milestone_payment`, wrap it; otherwise lean on automatic release triggered by approvals.

3. **Orchestrator Flow**
   - Execution path marks milestone `completed`.
   - Approval endpoint updates DID list, calls pallet, then marks `Paid` once release confirmed.
   - Payment manager emits `PaymentEvent` per milestone (type `milestone_release`).

4. **API Design**
   - Request/response schemas versioned (`milestone_schema_version: 1`).
   - Evidence payload stored as JSON (hash large blobs to R2/S3).
   - All endpoints logged with correlation IDs for dispute traceability.

5. **Docs & DX**
   - Provide sample JSON + CLI commands.
   - Section in Docs home describing milestone use cases (enterprise deployments, phased delivery).

---

## Milestone Schedule

| Day | Track | Deliverables |
| --- | --- | --- |
| 1 | Core plumbing | TaskMilestone struct update, release/refund functions, unit tests |
| 2 | API layer | REST handlers + router + auth checks |
| 3 | Oracle hooks | Condition model, oracle adapter stub, timeout paths |
| 4 | Metrics & docs | Prometheus, Grafana, developer guide |
| 5 | Testing & rollout | Integration suite, demo script, release notes |

---

## Risks & Mitigation

| Risk | Impact | Mitigation |
| --- | --- | --- |
| Substrate milestone release events not exposed | Delays payout confirmation | Poll `GetEscrow` + parse `milestones` as interim solution |
| Partial release unsupported by PaymentManager | Fallback path blocked | Extend PaymentManager with `RecordMilestonePayout` that deducts portion from outstanding balance |
| Oracle integration slows sprint | Conditional releases slip | Ship deterministic mock oracle first, wire external feeds in Sprint 11 |

---

## Definition of Done
- ✅ Milestone creation, completion, approval, and payout flows operate against devnet + local mocks.
- ✅ REST + CLI entry points documented and tested.
- ✅ Metrics live; dashboards refreshed.
- ✅ Sprint 10 completion report ready for publish.

---

**Sprint 10 Status**: Ready to begin 🚀
