const field = @import("../field/koalabear.zig");
const ext = @import("../field/koalabear_ext.zig");
const protocol = @import("../protocol/root.zig");

pub const Error = error{
    MissingDynamicModuleSize,
    InvalidModuleSize,
    InvalidClaimCount,
    QuotientIdentityMismatch,
    LagrangeSelectorInDomain,
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
    lagrange_selector: usize,
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

pub fn verify(comptime system: System, input: CheckInput) Error!void {
    if (input.witness_claims.len != system.total_witness_claims) return error.InvalidClaimCount;
    if (input.quotient_claims.len != system.total_quotient_claims) return error.InvalidClaimCount;
    inline for (system.modules) |module| {
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
    comptime module: Module,
    comptime static_n: usize,
    dynamic_n: usize,
    input: CheckInput,
    merge_coin: ext.Ext,
    eval_coin: ext.Ext,
) Error!void {
    // Static module sizes are embedded in the generated System, so Zig can
    // specialize this function at comptime. Dynamic modules use static_n == 0
    // as a sentinel; the caller in verify() looks up n from module_sizes and
    // passes it here as dynamic_n.
    //
    // The inline loops below are intentional: they traverse generated metadata
    // whose indices must stay comptime-known to avoid runtime expression-DAG
    // dispatch. Data loops, such as quotient-share recombination, remain plain
    // for loops.
    comptime {
        if (static_n != 0) {
            if (!validModuleSize(static_n)) @compileError("static vanishing module size must be a non-zero power of two");
            _ = field.rootOfUnityBy(static_n) catch @compileError("static vanishing module size exceeds supported KoalaBear root-of-unity order");
        }
    }
    if (static_n == 0 and !validModuleSize(dynamic_n)) return error.InvalidModuleSize;

    // Let r be the evaluation coin and H the module domain of size n (= static_n
    // for static modules, else dynamic_n). The prover computes the domain
    // annihilator Z_H(r) = r^n - 1.
    const annihilator = powModuleSize(eval_coin, static_n, dynamic_n).sub(ext.Ext.one());

    const ctx = EvalCtx{ .coin = eval_coin, .annihilator = annihilator, .dynamic_n = dynamic_n };
    inline for (module.buckets) |bucket| {
        try verifyBucket(module, bucket, static_n, input, merge_coin, ctx);
    }
}

fn powModuleSize(r: ext.Ext, comptime static_n: usize, dynamic_n: usize) ext.Ext {
    // When static_n is non-zero, the exponent n is part of the comptime System
    // and powComptime emits a fixed exponentiation chain. Otherwise n is known
    // only from the verifier input and we use the runtime exponentiation path.
    if (static_n != 0) {
        return r.powComptime(static_n);
    }
    return r.pow(@as(u256, dynamic_n));
}

fn verifyBucket(
    comptime module: Module,
    comptime bucket: Bucket,
    comptime static_n: usize,
    input: CheckInput,
    merge_coin: ext.Ext,
    ctx: EvalCtx,
) Error!void {
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
    inline for (bucket.vanishings) |v| {
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

fn evalExpr(
    comptime module: Module,
    comptime expr_index: usize,
    comptime static_n: usize,
    ctx: EvalCtx,
    input: CheckInput,
) Error!ext.Ext {
    const node = module.expressions[expr_index];
    return switch (node) {
        .column_claim => |claim_index| input.witness_claims[module.witness_claim_offset + claim_index],
        .cell_value => |ref| input.ctx.rounds[ref.round].cells[ref.index].toExt(),
        .coin_value => |coin_index| input.ctx.all_coins[coin_index],
        .constant => |value| ext.Ext.lift(value),
        .op => |op| try evalOp(module, op, static_n, ctx, input),
        .lagrange_selector => |position| try evalLagrangeSelector(position, static_n, ctx),
    };
}

fn evalOp(
    comptime module: Module,
    comptime op: ExprOp,
    comptime static_n: usize,
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
// position is comptime-known (it lives in the comptime expression DAG), so for
// static modules (static_n != 0) omega^position folds to a comptime field
// constant via staticRootPower — no runtime exponentiation. Dynamic modules
// (static_n == 0) derive the n-th root of unity from ctx.dynamic_n and take the
// runtime pow. Everything else (the annihilator, the r - omega^position
// denominator, the division) depends on the runtime eval coin r and stays
// runtime in both cases.
fn evalLagrangeSelector(comptime position: usize, comptime static_n: usize, ctx: EvalCtx) Error!ext.Ext {
    const omega_pos = if (static_n != 0)
        comptime staticRootPower(static_n, position)
    else blk: {
        const omega = field.rootOfUnityBy(ctx.dynamic_n) catch return error.InvalidModuleSize;
        break :blk omega.powComptime(position);
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
    const n = if (static_n != 0) static_n else ctx.dynamic_n;
    const denominator = r_minus_omega.mulByBase(field.Element.init(@as(u64, n)));

    return numerator.div(denominator);
}

fn cancellationAtPoint(
    comptime positions: []const i32,
    comptime static_n: usize,
    ctx: EvalCtx,
) Error!ext.Ext {
    if (positions.len == 0) return ext.Ext.one();

    const omega = if (static_n == 0) field.rootOfUnityBy(ctx.dynamic_n) catch return error.InvalidModuleSize else field.Element.one();
    var result = ext.Ext.one();

    inline for (positions) |position| {
        // Cancellation polynomial for openings already enforced elsewhere:
        // C(r) = product_{k in cancelled} (r - omega_n^norm(k)).
        const root = if (static_n != 0)
            comptime staticRootPower(static_n, normalizePosition(position, static_n, 0))
        else if (comptime position >= 0)
            omega.powComptime(comptime @as(usize, @intCast(position)))
        else
            omega.pow(@as(u64, normalizePosition(position, 0, ctx.dynamic_n)));
        result = result.mul(ctx.coin.sub(ext.Ext.lift(root)));
    }
    return result;
}

fn staticRootPower(comptime n: usize, comptime k: usize) field.Element {
    const omega = field.rootOfUnityBy(n) catch unreachable;
    return omega.powComptime(k);
}

fn normalizePosition(comptime position: i32, comptime static_n: usize, dynamic_n: usize) usize {
    const n = if (static_n != 0) static_n else dynamic_n;
    if (position < 0) return n - @as(usize, @intCast(-position));
    return @as(usize, @intCast(position));
}

fn validModuleSize(n: usize) bool {
    return field.isPowerOfTwo(n);
}
