const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");
const protocol = @import("../protocol/root.zig");

pub const Error = error{
    MissingDynamicModuleSize,
    InvalidModuleSize,
    InvalidClaimCount,
    QuotientIdentityMismatch,
    LagrangeSelectorInDomain,
    LagrangeSelectorPositionOutOfRange,
    CellRefOutOfRange,
};

pub const ModuleSize = union(enum) {
    static: usize,
    dynamic: usize,
};

pub const Operator = enum {
    add,
    mul,
    sub,
    div,
    double,
    square,
    negate,
    inverse,
};

pub const ExprOp = struct {
    operator: Operator,
    operands: []const usize,
};

pub const ScalarRef = struct {
    round: usize,
    index: usize,
};

pub const ExprNode = union(enum) {
    column_claim: usize,
    cell_value: ScalarRef,
    coin_value: usize,
    constant: field.Element,
    op: ExprOp,
    // i32, not usize (matching Vanishing.cancelled_positions' own type): a
    // LagrangeSelector position may be end-relative (negative — -1 is the
    // module's last row, mirroring prover-ray wiop.LagrangeSelector's own
    // convention). Codegen resolves a STATIC module's negative position into
    // [0, size) at codegen time (the size is already known there); a DYNAMIC
    // module's position is left negative and resolved here at verify time
    // against the runtime size, since the size isn't known until then. See
    // evalLagrangeSelector / normalizePosition.
    lagrange_selector: i32,
};

pub const Vanishing = struct {
    expression: usize,
    cancelled_positions: []const i32 = &.{},
};

pub const Bucket = struct {
    ratio: usize,
    vanishings: []const Vanishing,
    quotient_claim_offset: usize,
};

pub const Module = struct {
    size: ModuleSize,
    expressions: []const ExprNode,
    buckets: []const Bucket,
    witness_claim_offset: usize,
    merge_coin_index: usize,
    eval_coin_index: usize,
};

pub const System = struct {
    modules: []const Module,
    dynamic_module_count: usize = 0,
    total_witness_claims: usize = 0,
    total_quotient_claims: usize = 0,
};

/// Input to the vanishing sub-verifier. Protocol-level data (coins and cell
/// openings) arrives pre-derived via `ctx`; only vanishing-specific claims are
/// added here. The sub-verifier performs only mathematical checks.
pub const CheckInput = struct {
    ctx: protocol.Context,
    witness_claims: []const ext.Ext,
    quotient_claims: []const ext.Ext,
    module_sizes: []const usize = &.{},
};

pub fn verify(system: System, input: CheckInput) Error!void {
    if (input.witness_claims.len != system.total_witness_claims) return error.InvalidClaimCount;
    if (input.quotient_claims.len != system.total_quotient_claims) return error.InvalidClaimCount;
    for (system.modules) |module| {
        const merge_coin = input.ctx.all_coins[module.merge_coin_index];
        const eval_coin = input.ctx.all_coins[module.eval_coin_index];
        switch (module.size) {
            .static => |n| try verifyModule(module, n, 0, input, merge_coin, eval_coin),
            .dynamic => |size_index| {
                if (size_index >= input.module_sizes.len) return error.MissingDynamicModuleSize;
                try verifyModule(module, 0, input.module_sizes[size_index], input, merge_coin, eval_coin);
            },
        }
    }
}

fn verifyModule(
    module: Module,
    static_n: usize,
    dynamic_n: usize,
    input: CheckInput,
    merge_coin: ext.Ext,
    eval_coin: ext.Ext,
) Error!void {
    // Static module sizes are stored in the decoded System. Dynamic modules use
    // static_n == 0 as a sentinel; the caller looks up n from module_sizes and
    // passes it here as dynamic_n.
    const n = if (static_n != 0) static_n else dynamic_n;
    if (!validModuleSize(n)) return error.InvalidModuleSize;
    _ = field.rootOfUnityBy(n) catch return error.InvalidModuleSize;

    // Let r be the evaluation coin and H the module domain of size n (= static_n
    // for static modules, else dynamic_n). The prover computes the domain
    // annihilator Z_H(r) = r^n - 1.
    const annihilator = powModuleSize(eval_coin, static_n, dynamic_n).sub(ext.Ext.one());

    const ctx = EvalCtx{ .coin = eval_coin, .annihilator = annihilator, .dynamic_n = dynamic_n };
    // Runtime loop over runtime buckets: see verifyBucket's own note on why
    // `bucket` is deliberately not comptime.
    for (module.buckets) |bucket| {
        try verifyBucket(module, bucket, static_n, input, merge_coin, ctx);
    }
}

fn powModuleSize(r: ext.Ext, static_n: usize, dynamic_n: usize) ext.Ext {
    // Both static and dynamic sizes arrive at runtime; static_n distinguishes
    // decoded fixed sizes from proof-supplied dynamic sizes.
    return r.pow(@intCast(if (static_n != 0) static_n else dynamic_n));
}

fn verifyBucket(
    module: Module,
    bucket: Bucket,
    static_n: usize,
    input: CheckInput,
    merge_coin: ext.Ext,
    ctx: EvalCtx,
) Error!void {
    // `bucket` is a RUNTIME parameter, and the loop over its vanishings below is
    // a runtime loop, for the same reason evalExpr/evalOp take a runtime
    // expr_index (see the long note there).
    //
    // When `bucket` was comptime, Zig monomorphized a distinct verifyBucket per
    // bucket and `inline for (bucket.vanishings)` unrolled every constraint of
    // that bucket into straight-line code. On the real RISC-V arithmetization
    // that produced 85 instantiations totalling ~6.0 MiB of the ~7.9 MiB
    // .text — enough to push the guest's executable span past elf_to_json's
    // 2,000,000-record pre-decoding cap and to dominate the interpreted
    // instruction-fetch cost in zkc.
    //
    // Nothing here needs bucket to be comptime: Bucket/Vanishing are plain data
    // (ratio, a slice of expression indices, a claim offset), the expression
    // indices are already consumed as runtime values by evalExpr, and
    // cancelled_positions is likewise handled at runtime by cancellationAtPoint.
    // `module` and `static_n` are runtime values on the compact-system path.

    // r^n = Z_H(r) + 1, recovered from the annihilator carried in ctx.
    const r_pow_n = ctx.annihilator.add(ext.Ext.one());
    var quotient = ext.Ext.zero();
    var r_pow_kn = ext.Ext.one();
    for (0..bucket.ratio) |i| {
        // Recombine quotient-share claims:
        // Q(r) = sum_k r^(k*n) * Q_k(r) = sum_k (r^n)^k * Q_k(r).
        quotient = quotient.add(r_pow_kn.mul(input.quotient_claims[bucket.quotient_claim_offset + i]));
        r_pow_kn = r_pow_kn.mul(r_pow_n);
    }

    var aggregate = ext.Ext.zero();
    var coin_power = ext.Ext.one();
    for (bucket.vanishings) |v| {
        // Aggregate the vanished numerators with the merge coin alpha:
        // P_agg(r) = sum_i alpha^i * P_i(r) * C_i(r).
        const value = try evalExpr(module, v.expression, static_n, ctx, input);
        const cancellation = try cancellationAtPoint(v.cancelled_positions, static_n, ctx);
        aggregate = aggregate.add(coin_power.mul(value.mul(cancellation)));
        coin_power = coin_power.mul(merge_coin);
    }

    // PLONK quotient identity checked by prover-ray/global.Verifier.Check:
    // P_agg(r) = Z_H(r) * Q(r) = (r^n - 1) * Q(r).
    if (!aggregate.eql(ctx.annihilator.mul(quotient))) return error.QuotientIdentityMismatch;
}

// EvalCtx carries the per-module evaluation context that is shared, unchanged,
// by every node of an expression: the eval coin r, the domain annihilator
// r^n - 1, and dynamic_n (the runtime module size, 0 for static modules). Only
// lagrange_selector leaves read dynamic_n, and only on the dynamic path: a
// static module's size is the decoded static_n threaded into the leaf, while
// dynamic_n stays the unused 0 sentinel. The other node kinds ignore the
// context and merely forward it down the recursion. Bundling it keeps
// evalExpr/evalOp from threading unused scalars.
const EvalCtx = struct {
    coin: ext.Ext,
    annihilator: ext.Ext,
    dynamic_n: usize,
};

// evalExpr/evalOp evaluate a single node of a module's expression tree,
// identified by a RUNTIME expr_index/op rather than a comptime one.
//
// module.expressions is built by codegen (see codegen/vanishing.go's
// appendExpr) as a post-order flattening of each vanishing constraint's
// expression tree: every operand is appended, and therefore assigned its
// index, strictly before the node that references it. So op.operands[i] is
// always < the node's own index, and recursion here always makes progress
// toward index 0 (the array's leaves) — there is no cycle.
//
// expr_index/op used to be `comptime` parameters. That made Zig monomorphize
// a distinct evalExpr/evalOp instantiation per unique node ever evaluated —
// for a real (non-synthetic) RISC-V arithmetization module with thousands of
// expression nodes, that blew up into a stack overflow (observed as a SIGSEGV
// inside verifyModule) well before any actual recursion depth problem: the
// per-tree depth is shallow in practice (a few dozen levels for real modules),
// but the sheer number of monomorphized node-specific function bodies bloated
// the generated code and its stack frames. Runtime module metadata gives one
// evalExpr/evalOp implementation, with recursion depth bounded by the module's
// actual (shallow) expression-tree depth — eliminating the blowup without
// changing any evaluation semantics or error behavior.
fn evalExpr(
    module: Module,
    expr_index: usize,
    static_n: usize,
    ctx: EvalCtx,
    input: CheckInput,
) Error!ext.Ext {
    const node = module.expressions[expr_index];
    return switch (node) {
        .column_claim => |claim_index| input.witness_claims[module.witness_claim_offset + claim_index],
        .cell_value => |ref| (try input.ctx.cell(ref.round, ref.index)).toExt(),
        .coin_value => |coin_index| input.ctx.all_coins[coin_index],
        .constant => |value| ext.Ext.lift(value),
        .op => |op| try evalOp(module, op, static_n, ctx, input),
        .lagrange_selector => |position| try evalLagrangeSelector(position, static_n, ctx),
    };
}

fn evalOp(
    module: Module,
    op: ExprOp,
    static_n: usize,
    ctx: EvalCtx,
    input: CheckInput,
) Error!ext.Ext {
    const a = try evalExpr(module, op.operands[0], static_n, ctx, input);
    return switch (op.operator) {
        .add => a.add(try evalExpr(module, op.operands[1], static_n, ctx, input)),
        .mul => a.mul(try evalExpr(module, op.operands[1], static_n, ctx, input)),
        .sub => a.sub(try evalExpr(module, op.operands[1], static_n, ctx, input)),
        .div => a.div(try evalExpr(module, op.operands[1], static_n, ctx, input)),
        .double => a.add(a),
        .square => a.square(),
        .negate => a.neg(),
        .inverse => a.inverse(),
    };
}

// evalLagrangeSelector evaluates the low-degree extension of a Lagrange
// selector at the eval coin r:
//
//     L_position(r) = omega^position * (r^n - 1) / (n * (r - omega^position)),
//
// where omega is the canonical n-th root of unity and n is the module size, and
// the (r^n - 1) factor is the domain annihilator precomputed in ctx. This
// mirrors prover-ray wiop.LagrangeSelector.EvaluateOutOfDomain, the reference
// used by global.Verifier.
//
// position comes from the node's runtime expression payload (module.expressions
// is looked up by a runtime expr_index — see evalExpr) and may be end-relative
// (negative — -1 is the module's last row, mirroring prover-ray
// wiop.LagrangeSelector's own convention), the same shape cancellationAtPoint's
// `positions` already handles via normalizePosition. For a STATIC module
// (static_n != 0), codegen has already resolved a negative position into
// [0, static_n) (the module size is known at codegen time), so
// normalizePosition is a no-op there in practice, but is still used for
// consistency with cancellationAtPoint. Both the root-of-unity lookup and the
// position-derived exponent are runtime operations. A DYNAMIC module's size is
// only known at verify time, so its position is
// normalized into [0, ctx.dynamic_n) here at runtime before the runtime pow.
// Everything else (the annihilator, the r - omega^position denominator, the
// division) depends on the runtime eval coin r and stays runtime in both cases.
fn evalLagrangeSelector(position: i32, static_n: usize, ctx: EvalCtx) Error!ext.Ext {
    // Bounds-check before normalizing. For a DYNAMIC module n comes from the
    // proof-supplied module_sizes, so a hostile size can push a codegen-baked
    // position out of [-n, n): a position < -n would underflow
    // normalizePosition's usize subtraction, and a position >= n would be
    // silently reduced mod n by the root-of-unity exponentiation, evaluating a
    // DIFFERENT selector than the constraint declares. Mirrors the Go
    // reference (wiop.LagrangeSelector.resolvedRow), which rejects positions
    // outside [-n, n).
    const n = if (static_n != 0) static_n else ctx.dynamic_n;
    if (!validPosition(position, n)) return error.LagrangeSelectorPositionOutOfRange;

    const omega = field.rootOfUnityBy(n) catch return error.InvalidModuleSize;
    const omega_pos = omega.pow(@as(u64, normalizePosition(position, static_n, ctx.dynamic_n)));

    // numerator = omega^position * (r^n - 1).
    const numerator = ctx.annihilator.mulByBase(omega_pos);

    // denominator = n * (r - omega^position), where n = static_n for static
    // modules else the runtime dynamic_n.
    // The field defines 1/0 = 0, so an in-domain eval coin (r == omega^position)
    // would silently yield 0; reject it explicitly to match the Go evaluator's
    // out-of-domain contract.
    const r_minus_omega = ctx.coin.sub(ext.Ext.lift(omega_pos));
    if (r_minus_omega.isZero()) return error.LagrangeSelectorInDomain;
    const denominator = r_minus_omega.mulByBase(field.Element.init(@as(u64, n)));

    return numerator.div(denominator);
}

// `positions` is a RUNTIME slice: it comes from a runtime Vanishing (see
// verifyBucket). Both static and dynamic root-of-unity lookups happen at
// runtime on the compact-system path.
fn cancellationAtPoint(
    positions: []const i32,
    static_n: usize,
    ctx: EvalCtx,
) Error!ext.Ext {
    if (positions.len == 0) return ext.Ext.one();

    const n = if (static_n != 0) static_n else ctx.dynamic_n;
    const omega = field.rootOfUnityBy(n) catch return error.InvalidModuleSize;
    var result = ext.Ext.one();

    for (positions) |position| {
        // Same runtime bounds check as evalLagrangeSelector, for the same
        // reason: on the dynamic path n is proof-supplied, so a hostile size
        // can push a codegen-baked position out of [-n, n) (usize underflow
        // when position < -n, silent mod-n reduction when position >= n). The
        // Static positions come from trusted generated metadata; the dynamic
        // path additionally depends on the proof-supplied module size.
        if (static_n == 0 and !validPosition(position, ctx.dynamic_n)) {
            return error.LagrangeSelectorPositionOutOfRange;
        }
        // Cancellation polynomial for openings already enforced elsewhere:
        // C(r) = product_{k in cancelled} (r - omega_n^norm(k)).
        const root = omega.pow(@as(u64, normalizePosition(position, static_n, ctx.dynamic_n)));
        result = result.mul(ctx.coin.sub(ext.Ext.lift(root)));
    }
    return result;
}

// Whether an (end-relative) selector position is addressable in a module of
// size n, i.e. lands in [-n, n). Callers must check this before
// normalizePosition, whose negative branch underflows for position < -n.
// The magnitude is widened through i64 so that position == minInt(i32) cannot
// overflow the negation.
fn validPosition(position: i32, n: usize) bool {
    if (position >= 0) {
        return @as(usize, @intCast(position)) < n;
    }
    const magnitude: usize = @intCast(-@as(i64, position));
    return magnitude <= n;
}

// Resolves an end-relative position into [0, n). Precondition: position is in
// [-n, n) — see validPosition; the subtraction below underflows otherwise.
fn normalizePosition(position: i32, static_n: usize, dynamic_n: usize) usize {
    const n = if (static_n != 0) static_n else dynamic_n;
    if (position < 0) return n - @as(usize, @intCast(-position));
    return @as(usize, @intCast(position));
}

fn validModuleSize(n: usize) bool {
    return field.isPowerOfTwo(n);
}
