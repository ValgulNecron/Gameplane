# Pencil MCP: visitor-based `Get` crashes on ~41 top-level roots

## Environment
- File: `/home/valgul/project/Gameplane/design.pen`
- Access method: `mcp__pencil__execute` (execute API), read-only investigation
- Pencil MCP server, `batch_design_runtime.js` (bundled runtime invoked by `execute`)

## Exact stack / error
```
TypeError: cannot read property of undefined
    at visitNodes (batch_design_runtime.js:8:40)
    ...
    at visitNodes (batch_design_runtime.js:27:17)
```
Raised by both `Get(id, visitFn)` and the document-wide `Get(visitFn)` form. Never raised by
`Get(id, {depth: N})` (non-visitor read), which returns the full raw subtree successfully for
every node tested, including all crashing ones.

## Minimal repro
```js
// Works (non-visitor):
Get("Z6aeH", {depth: 10000})

// Crashes:
Get("Z6aeH", n => true)
```
`Z6aeH` ("Header Row", inside `F9pUrx/ucID1`, the Servers Table) is representative of the
minimal crashing unit. Bisection detail below.

## Bisection (F9pUrx / ucID1 — "Servers Table")
1. `Get("F9pUrx/ucID1", n => true)` → CRASH.
2. Its two children, visited as roots individually:
   - `Get("o6QNN", n=>true)` (Head Row) → CRASH
   - `Get("Oa9hp", n=>true)` (Body) → CRASH
3. Descending into `o6QNN`'s single child `Z6aeH` (Header Row) as root → CRASH.
4. `Z6aeH`'s own 8 children (`m6jlCE, g8chx, JlFKH, WzOVs, fEPGw, qmuv5, UDnA9, QPxOE`,
   all `ref → DYYKO` "Table Column" instances), visited **individually as the root of their
   own `Get` call** → **all OK**.
5. But visiting `Z6aeH` itself with a visitor that recurses into those same children (even
   keeping only 1, or 3, of the 8 via `ctx.skipChildren()` on the rest) → CRASH every time,
   including the single-child case. Visiting `Z6aeH` with `ctx.skipChildren()` called
   immediately (i.e. visiting only the node itself, no descent) → OK.

   ⇒ The crash is triggered specifically by the runtime's handling of `Z6aeH` when it must
   resolve/merge `Z6aeH`'s own override data while descending into its children — not by
   any of the 8 leaf children themselves, and not by "many siblings" per se.

Conclusion of bisection: the smallest crashing node is `Z6aeH` itself (a `ref` node — see next
section for its shape).

## The node JSON shape that triggers it
`Z6aeH` (non-visitor read):
```json
{
  "id": "Z6aeH", "type": "ref", "ref": "tDY4O", "name": "Header Row",
  "width": "fill_container", "layout": "horizontal", "alignItems": "center",
  "children": [ /* 8 literal DYYKO ref instances, e.g. */
    {"id":"m6jlCE","type":"ref","ref":"DYYKO","name":"HC NAME","width":"fill_container",
     "justifyContent":"start","descendants":{"SIu74":{"content":"NAME"}}},
    ...
  ]
}
```
The `.pen` schema's `Ref` interface only defines `ref` and `descendants` (plus an `[key:string]: any`
catch-all) as the override mechanism for a component instance — there is no documented `children`
override on a `ref` node. `Z6aeH` has a **literal top-level `children` array** instead of (or as
well as) `descendants`. `layout`/`alignItems` are likewise foreign properties on a `ref`.

Confirmed the same shape on every other crashing root checked:
- `hxova` (inside `IoDHI` → `B3o9Eq` "Wizard Modal Footer / FooterRight"): `type:"ref", ref:"cb4rt"`
  with a literal `children:[...]` containing brand-new text/icon nodes (`H87Gb`, `ECmNv`) whose ids
  don't correspond at all to the underlying component `cb4rt`'s real children (`Xpn7K`, `U6q13B`).
  `Get("hxova", n=>true)` crashes even as sole root (no siblings involved).
- `tkoqz` ("RBAC Filter Row"): child `j2eDt3` is `type:"ref", ref:"Dx8S8"` with literal
  `children:[fT0iB, vPDGL, ...]`, each of which is *itself* a `ref` with its own literal
  `children` override (`fT0iB`: `ref:"CEWo3"` with children `[vym2o, qv4ZW]`, etc.) — nested
  ref-with-literal-children.
- `DPrYX` ("Screen/Backups — Index"): `KYn2g` is `type:"ref", ref:"ECbbo"` (Table) with literal
  `children:[O8swk, ...]`, where `O8swk` is `type:"ref", ref:"eafUs"` (Head Row) with its own
  literal `children:[...DYYKO refs...]` — same nested pattern as `ucID1`/`Z6aeH`.

Control (non-crashing): `kIxaJ` ("Audit Integrity Banner") visits fine with a visitor
(`Get("kIxaJ", n=>true)` → OK). It is a plain `frame`, and none of its nested `ref` nodes carry a
literal `children` override — everything is either a plain frame/text/icon subtree or a `ref` using
only `descendants`.

## Whether the ref target exists
Checked directly: `DYYKO`'s real component tree does contain a descendant named `SIu74`
(`Get("DYYKO", {depth:5})` → `children:[{"type":"text","id":"SIu74","name":"Label",...}]`), so the
`descendants:{"SIu74":{...}}` overrides used *inside* the literal-children entries are pointing at
real ids. **The crash is not caused by a dangling/missing `descendants` key.** It is caused by the
outer `ref` node (`Z6aeH`, `hxova`, `j2eDt3`, `KYn2g`, `O8swk`, ...) itself carrying a literal
`children` array as if it were a plain frame, which the visitor's `visitNodes` apparently attempts
to reconcile against the referenced component's real children (by id/position) while building each
child's traversal context (bounds, index, etc.), and dereferences something on an undefined match
(e.g. because `hxova`'s override children ids don't exist anywhere in `cb4rt`'s real child list).

## Suspected cause
`visitNodes` in `batch_design_runtime.js`, when descending through a `ref` node, tries to resolve
the effective/overridden child list by correlating the ref's own literal `children` property with
the underlying component's definition (something the non-visitor `Get(id,{depth})` path does not
need to do — it just serializes whatever is stored). Because `children` on a `ref` isn't a
recognized/valid override shape (only `descendants` is per the schema), this correlation step gets
an unexpected shape and ends up indexing into a lookup table/map with an id or index that isn't
present, yielding `undefined`, and then reads a property off that `undefined` value → `TypeError:
cannot read property of undefined`.

## Common pattern across all crashing roots
Every crashing root traced so far contains at least one `ref` node whose customization is stored as
a literal top-level `children` array (a full node-tree, sometimes nested ref-within-ref) instead of
the documented `descendants` override map. This is very likely the sole/common trigger for all ~41
crashing roots (`F9pUrx`/`ucID1`, `jmoi3`, `DPrYX`, `o4LH8W`, `tkoqz`, `IoDHI`/`B3o9Eq`, etc.).

## Workaround
Use the non-visitor form exclusively when reading these subtrees:
```js
Get(id, {depth: 10000})   // or a bounded depth; works on all 41 affected roots
```
Avoid `Get(id, visitFn)` and `Get(visitFn)` (whole-document visitor) on any subtree that contains a
`ref` node with a literal `children` override.

## Proposed fix (NOT applied — read-only task)
This is not a case of a single stale `descendants` key that a trivial property `Update` could clear.
The malformed shape is a full `children` array replacing the normal override mechanism on `ref`
nodes, in some cases nested two levels deep (ref-with-children containing another ref-with-children).
Removing just the `children` property via `Update(id, {children: ...})` would need to be paired with
re-adding the same visual content as a proper `descendants` map (or, where the override truly is a
wholesale replacement of the component's structure, expressed via `descendants: {<realDescendantId>:
{type: ..., ...}}` per the schema's "replacement" mechanism) — otherwise the visible content coming
from that `children` array would simply disappear when the ref falls back to the component's default
children. So: plausible root cause, but the correct fix is a data migration/normalization of every
such `ref+children` node into valid `descendants` overrides, not a one-line property update. This
report only diagnoses the crash; no writes were made to the document.
