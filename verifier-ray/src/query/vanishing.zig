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

/// A module's row count, or the index of the proof-supplied size for a dynamic
/// module.
///
/// u32, not `Index`: this is a domain size, not an index into an array. Real
/// modules already reach 65,536 rows and the PCS envelope supports 2^22, so a
/// u16 would overflow. A usize payload would make this union 16 bytes rather
/// than 8.
pub const ModuleSize = union(enum) {
    static: u32,
    dynamic: u32,
};

/// Index into a per-system array: a claim offset, a coin index, an expression
/// position.
///
/// u16, not usize, because a Zig `usize` field occupies 8 bytes whatever it
/// holds. The layout has to keep `ptr[i]` a single indexed load, so a small
/// value is stored as the value plus seven zero bytes — and nearly every index
/// here is small: of the first 200,000 8-byte lanes in the generated .rodata,
/// 174,867 (87%) held a value that fits in two bytes.
///
/// u16 bounds a module at 65,535 expression nodes. On the real RISC-V system
/// the widest values are 58,795 (expression indices), 15,235 (claim slots),
/// 14,354 (witness claim offsets) and 229 (coins), so the bound holds but is
/// not comfortable: codegen asserts every value fits and names the offending
/// module rather than truncating. Module sizes are deliberately NOT this type —
/// see `ModuleSize`.
pub const Index = u16;

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

/// One arithmetic node of an expression tree.
///
/// The operands are two fixed fields rather than a slice because every operator
/// this codebase emits is unary or binary: `rhs` is read only by the binary
/// operators, and is a don't-care (written as 0 by codegen) for the unary ones.
///
/// This is deliberately not a slice. A `[]const Index` is a 16-byte fat pointer
/// ({ptr, len}) whatever its element type, which made it the widest arm of
/// `ExprNode` and pinned the whole union at 32 bytes — so narrowing the index
/// type alone saved nothing. Two `Index` fields make the union 8 bytes instead,
/// a 4x cut across the 482,341 nodes the real RISC-V system emits.
pub const ExprOp = struct {
    operator: Operator,
    lhs: Index,
    rhs: Index = 0,
};

pub const ScalarRef = struct {
    round: Index,
    index: Index,
};

pub const ExprNode = union(enum) {
    column_claim: Index,
    cell_value: ScalarRef,
    coin_value: Index,
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
    expression: Index,
    cancelled_positions: []const i32 = &.{},
};

pub const Bucket = struct {
    ratio: Index,
    vanishings: []const Vanishing,
    quotient_claim_offset: Index,
};

pub const Module = struct {
    size: ModuleSize,
    expressions: []const ExprNode,
    buckets: []const Bucket,
    witness_claim_offset: Index,
    merge_coin_index: Index,
    eval_coin_index: Index,
};

pub const System = struct {
    modules: []const Module,
    dynamic_module_count: Index = 0,
    total_witness_claims: Index = 0,
    total_quotient_claims: Index = 0,
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
    // A runtime loop over runtime modules. When `system` was comptime and this
    // was an `inline for`, Zig monomorphized verifyModule/verifyBucket/evalExpr
    // per module and unrolled every constraint into straight-line code — on the
    // real RISC-V arithmetization that is ~100 modules and 482,341 expression
    // nodes, and it dominated .text. Nothing here needs comptime: System and
    // Module are plain data, and every index is already consumed as a runtime
    // value.
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
    static_n: u32,
    dynamic_n: usize,
    input: CheckInput,
    merge_coin: ext.Ext,
    eval_coin: ext.Ext,
) Error!void {
    // Static module sizes come from the generated System; dynamic modules use
    // static_n == 0 as a sentinel, and the caller in verify() looks up n from
    // module_sizes and passes it here as dynamic_n. Both are runtime values, so
    // what used to be a comptime assertion on the static size is a runtime
    // check — the error is returned rather than raised at compile time.
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

fn powModuleSize(r: ext.Ext, static_n: u32, dynamic_n: usize) ext.Ext {
    // Both sizes arrive at runtime now, so there is one exponentiation path;
    // static_n only distinguishes a generated fixed size from a proof-supplied
    // dynamic one.
    return r.pow(@as(u64, if (static_n != 0) static_n else dynamic_n));
}

fn verifyBucket(
    module: Module,
    bucket: Bucket,
    static_n: u32,
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
    // `module` and `static_n` are runtime too: monomorphizing per module cost
    // far more .text than the folded static-size exponentiation saved.

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
// static module's size is the comptime static_n threaded into the leaf, so its
// size-derived terms fold at comptime and dynamic_n stays the unused 0
// sentinel. The other node kinds ignore the context and merely forward it down
// the recursion. Bundling it keeps evalExpr/evalOp from threading unused scalars.
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
// index, strictly before the node that references it. So op.lhs/op.rhs are
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
// the generated code and its stack frames. Keeping module/static_n comptime
// (there are only ~100 modules total, and static_n legitimately folds
// static-size exponentiation/root-of-unity work at compile time) while making
// expr_index/op ordinary runtime values gives exactly one evalExpr/evalOp
// instantiation per module, with recursion depth bounded by that module's
// actual (shallow) expression-tree depth — eliminating the blowup without
// changing any evaluation semantics or error behavior.
fn evalExpr(
    module: Module,
    expr_index: Index,
    static_n: u32,
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
    static_n: u32,
    ctx: EvalCtx,
    input: CheckInput,
) Error!ext.Ext {
    const a = try evalExpr(module, op.lhs, static_n, ctx, input);
    return switch (op.operator) {
        .add => a.add(try evalExpr(module, op.rhs, static_n, ctx, input)),
        .mul => a.mul(try evalExpr(module, op.rhs, static_n, ctx, input)),
        .sub => a.sub(try evalExpr(module, op.rhs, static_n, ctx, input)),
        .div => a.div(try evalExpr(module, op.rhs, static_n, ctx, input)),
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
// consistency with cancellationAtPoint. static_n itself stays comptime (it's
// part of the comptime System), so staticRootPower's root-of-unity lookup
// still folds at compile time; only the exponent (position-derived) is
// runtime now, same cost as the already-runtime dynamic-module path below. A
// DYNAMIC module's size is only known at verify time, so its position is
// normalized into [0, ctx.dynamic_n) here at runtime before the runtime pow.
// Everything else (the annihilator, the r - omega^position denominator, the
// division) depends on the runtime eval coin r and stays runtime in both cases.
fn evalLagrangeSelector(position: i32, static_n: u32, ctx: EvalCtx) Error!ext.Ext {
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

    const omega_pos = if (static_n != 0)
        staticRootPower(static_n, normalizePosition(position, static_n, 0))
    else blk: {
        const omega = field.rootOfUnityBy(ctx.dynamic_n) catch return error.InvalidModuleSize;
        break :blk omega.pow(@as(u64, normalizePosition(position, 0, ctx.dynamic_n)));
    };

    // numerator = omega^position * (r^n - 1).
    const numerator = ctx.annihilator.mulByBase(omega_pos);

    // denominator = n * (r - omega^position), where n = static_n for static
    // modules (a comptime constant that folds here) else the runtime dynamic_n.
    // The field defines 1/0 = 0, so an in-domain eval coin (r == omega^position)
    // would silently yield 0; reject it explicitly to match the Go evaluator's
    // out-of-domain contract.
    const r_minus_omega = ctx.coin.sub(ext.Ext.lift(omega_pos));
    if (r_minus_omega.isZero()) return error.LagrangeSelectorInDomain;
    const denominator = r_minus_omega.mulByBase(field.Element.init(@as(u64, n)));

    return numerator.div(denominator);
}

// `positions` is a RUNTIME slice: it comes from a runtime Vanishing (see
// verifyBucket). static_n stays comptime so the static root-of-unity lookup
// still folds; only the position-derived exponent is runtime, which is what the
// dynamic path already did.
fn cancellationAtPoint(
    positions: []const i32,
    static_n: u32,
    ctx: EvalCtx,
) Error!ext.Ext {
    if (positions.len == 0) return ext.Ext.one();

    const omega = if (static_n == 0) field.rootOfUnityBy(ctx.dynamic_n) catch return error.InvalidModuleSize else field.Element.one();
    var result = ext.Ext.one();

    for (positions) |position| {
        // Same runtime bounds check as evalLagrangeSelector, for the same
        // reason: on the dynamic path n is proof-supplied, so a hostile size
        // can push a codegen-baked position out of [-n, n) (usize underflow
        // when position < -n, silent mod-n reduction when position >= n). The
        // static path normalizes at comptime against the trusted static_n, so
        // an out-of-range position there is a codegen bug caught at compile
        // time, not a proof-dependent condition.
        if (static_n == 0 and !validPosition(position, ctx.dynamic_n)) {
            return error.LagrangeSelectorPositionOutOfRange;
        }
        // Cancellation polynomial for openings already enforced elsewhere:
        // C(r) = product_{k in cancelled} (r - omega_n^norm(k)).
        const root = if (static_n != 0)
            staticRootPower(static_n, normalizePosition(position, static_n, 0))
        else
            omega.pow(@as(u64, normalizePosition(position, 0, ctx.dynamic_n)));
        result = result.mul(ctx.coin.sub(ext.Ext.lift(root)));
    }
    return result;
}

// The n-th root of unity raised to k. `n` is a runtime value now, so the root
// lookup happens at verify time rather than folding at compile time; the caller
// has already validated n via validModuleSize/rootOfUnityBy.
fn staticRootPower(n: u32, k: usize) field.Element {
    const omega = field.rootOfUnityBy(n) catch unreachable;
    return omega.pow(@as(u64, k));
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
fn normalizePosition(position: i32, static_n: u32, dynamic_n: usize) usize {
    const n = if (static_n != 0) static_n else dynamic_n;
    if (position < 0) return n - @as(usize, @intCast(-position));
    return @as(usize, @intCast(position));
}

fn validModuleSize(n: usize) bool {
    return field.isPowerOfTwo(n);
}
