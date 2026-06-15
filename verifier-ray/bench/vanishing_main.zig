const builtin = @import("builtin");
const verifier_ray = @import("verifier_ray");
const verify_data = @import("verify_data");
const bench_options = @import("vanishing_bench_options");

const protocol = verifier_ray.protocol;
const vanishing = verifier_ray.query.vanishing;

const is_r5_zkvm = builtin.target.cpu.arch == .riscv64 and builtin.target.os.tag == .freestanding;
const selected_case = verify_data.get(bench_options.case_index);

pub fn main() noreturn {
    if (comptime !is_r5_zkvm) {
        @compileError("vanishing benchmark guest is currently only wired for the R5 zkVM target");
    }
    r5_main();
}

pub export fn r5_main() noreturn {
    if (comptime !is_r5_zkvm) {
        @compileError("vanishing benchmark guest is currently only wired for the R5 zkVM target");
    }

    const proof = selected_case.proof;

    markR5(1);
    const replay_stats = protocol.replayWithStats(selected_case.spec, proof.rounds) catch exitR5(1);
    var all_coins = replay_stats.coins;

    markR5Value(2, replay_stats.compression_count);
    const replayed_input = vanishing.CheckInput{
        .ctx = .{
            .all_coins = &all_coins,
            .rounds = proof.rounds,
        },
        .witness_claims = proof.witness_claims,
        .quotient_claims = proof.quotient_claims,
        .module_sizes = proof.module_sizes,
    };
    vanishing.verify(selected_case.systems.vanishing, replayed_input) catch exitR5(1);

    markR5(3);
    exitR5(0);
}

fn markR5(comptime phase: u64) void {
    markR5Value(phase, 0);
}

fn markR5Value(comptime phase: u64, value: usize) void {
    asm volatile (
        \\mv a0, %[phase]
        \\mv a1, %[value]
        \\li a7, 4242
        \\ecall
        :
        : [phase] "r" (phase),
          [value] "r" (value),
        : .{ .a0 = true, .a1 = true, .a7 = true, .memory = true });
}

fn exitR5(code: u8) noreturn {
    asm volatile (
        \\mv a0, %[code]
        \\li a7, 93
        \\ecall
        :
        : [code] "r" (code),
    );
    unreachable;
}
