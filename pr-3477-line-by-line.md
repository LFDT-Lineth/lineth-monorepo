# PR 3477 Inline Walkthrough

PR: https://github.com/LFDT-Lineth/lineth-monorepo/pull/3477

Base: `828a22e5b2db93f49893abadd395d8e6166832fe` (`verifier-ray/field-opt`)
Head: `82c3224b50e578e90c54b7d875cdb6876d27c764` (`verifier-ray/poseidon2`)

This is the same walkthrough as before, but in a more readable inline form. I am intentionally skipping lines whose meaning is purely syntactic, such as closing braces, blank lines, and import statements when the import is obvious from later usage.

## What This PR Does

The PR has two pieces:

1. It adds a small `bench_compress` benchmark under `verifier-ray/bench`. The benchmark builds a freestanding RISC-V guest, runs Poseidon2 compression inside it, emits textual markers, and lets a Go runner calculate cycles per call from `zkc` output.
2. It optimizes the Poseidon2 hot path by unrolling compile-time-known loops, replacing repeated division-by-two helpers with precomputed field constants, and spelling already-reduced constants as raw `field.Element` literals.

## `verifier-ray/bench/bench_compress/build.zig`

This file teaches `zig build` how to compile the benchmark guest.

```zig
const std = @import("std");
const common = @import("build_common");
```

`std` is Zig's build API. `build_common` is the shared local package that already knows how verifier-ray guests should be built for the RISC-V target.

```zig
pub fn build(b: *std.Build) void {
    // Same freestanding rv64im guest target + entry stub + linker script as the
    // verifier and the other riscv-guests, via the shared build_common helper.
    const target = common.standardGuestTarget(b);
```

The benchmark is deliberately compiled for the same freestanding `rv64im` target as the other verifier RISC-V guests. That matters because the point is to measure the path as zkc will execute it, not a native macOS/Linux binary.

```zig
    const verifier_mod = b.addModule("verifier_ray", .{
        .root_source_file = b.path("../../src/lib.zig"),
        .target = target,
        .optimize = .ReleaseSmall,
        .strip = true,
    });
```

This creates an importable module named `verifier_ray` from verifier-ray's library root. It is compiled for the guest target, optimized with `ReleaseSmall`, and stripped.

```zig
    const profiling_opts = b.addOptions();
    profiling_opts.addOption(bool, "is_enabled", false);
    profiling_opts.addOption(bool, "is_r5_marks", false);
    verifier_mod.addOptions("profiling_config", profiling_opts);
```

Verifier-ray expects a `profiling_config` options module. The benchmark provides it with profiling disabled because this benchmark emits its own `COMPRESS-MARK` lines.

```zig
    const main_mod = b.createModule(.{
        .root_source_file = b.path("main.zig"),
        .target = target,
        .optimize = .ReleaseSmall,
        .strip = true,
        .imports = &.{
            .{ .name = "verifier_ray", .module = verifier_mod },
        },
    });
```

This creates the actual benchmark guest module from `main.zig` and wires in the `verifier_ray` import.

```zig
    // Link the statically-linked rv64im ELF with the shared entry stub (start.s)
    // + rv64im memory layout + dead-section GC.
    common.installGuestElf(b, main_mod, "bench-compress");
}
```

The helper links and installs the final RISC-V ELF at `zig-out/bin/bench-compress`, using the same entry stub and memory layout as the other guests.

## `verifier-ray/bench/bench_compress/build.zig.zon`

This is the Zig package manifest for the benchmark.

```zig
.{
    .name = .bench_compress,
    .version = "0.1.0",
    .fingerprint = 0xe44cd272be379fa4,
    .minimum_zig_version = "0.16.0",
```

Standard package metadata: name, version, fingerprint, and minimum Zig version.

```zig
    .dependencies = .{
        // shared build utilities for Zig build to R5 target
        .build_common = .{ .path = "../../../riscv-guests/build_common" },
    },
```

The benchmark depends on the local shared guest-build utilities, not a downloaded package.

```zig
    .paths = .{
        "build.zig",
        "build.zig.zon",
        "main.zig",
    },
}
```

Only the Zig package inputs are listed here. `run.go` is intentionally absent because it is the external runner, not part of the Zig package.

## `verifier-ray/bench/bench_compress/main.zig`

This is the guest program that actually runs inside the RISC-V/zkc environment.

```zig
// Micro-benchmark: measures RISC-V cycle cost of Poseidon2 compression.
//
// Marker IDs:
//    0 = start baseline,   1 = end baseline
//   10 = start compress,  11 = end compress
//
// The baseline loop matches the measured loop shape with an empty body so the
// runner can subtract loop-counter / branch overhead.
```

The markers define two timed regions. Region `0 -> 1` is an empty loop baseline. Region `10 -> 11` is the real compression loop. The runner subtracts the baseline from the compression region.

```zig
const verifier_ray = @import("verifier_ray");
const poseidon2 = verifier_ray.crypto.poseidon2;
const field = verifier_ray.field.koalabear;

const N: u64 = 10;
```

The guest imports Poseidon2 and KoalaBear from verifier-ray. `N` is the number of compression calls to measure.

```zig
fn writeR5(bytes: []const u8) void {
    asm volatile (
        \\li a0, 1
        \\mv a1, %[ptr]
        \\mv a2, %[len]
        \\li a7, 64
        \\ecall
        :
        : [ptr] "r" (@intFromPtr(bytes.ptr)),
          [len] "r" (bytes.len),
        : .{ .a0 = true, .a1 = true, .a2 = true, .a7 = true, .memory = true });
}
```

This is a tiny stdout writer for the RISC-V guest. It loads:

- `a0 = 1`, stdout.
- `a1 = bytes.ptr`, the buffer pointer.
- `a2 = bytes.len`, the buffer length.
- `a7 = 64`, the write syscall.

The clobber list tells Zig the syscall mutates those registers and memory visibility.

```zig
fn decimalBuf(buf: []u8, value: u64) []u8 {
    if (value == 0) {
        buf[0] = '0';
        return buf[0..1];
    }
```

`decimalBuf` formats a `u64` into caller-provided storage. Zero needs a special case because the digit loop below would otherwise write nothing.

```zig
    var tmp: [20]u8 = undefined;
    var n = value;
    var i: usize = tmp.len;
    while (n != 0) {
        i -= 1;
        tmp[i] = '0' + @as(u8, @intCast(n % 10));
        n /= 10;
    }
```

This writes digits right-to-left into a 20-byte temp buffer, which is enough for any `u64`.

```zig
    const digits = tmp[i..];
    @memcpy(buf[0..digits.len], digits);
    return buf[0..digits.len];
}
```

Then it copies the populated suffix into the caller's buffer and returns the slice containing the decimal text.

```zig
fn emitMark(phase: u64, checksum: u32) void {
    const prefix = "COMPRESS-MARK\t";
    var buf: [64]u8 = undefined;
```

`emitMark` creates a line that the Go runner can parse. A final line looks like:

```text
COMPRESS-MARK    10    0
```

Tabs separate the fields in the actual output.

```zig
    @memcpy(buf[0..prefix.len], prefix);
    var pos: usize = prefix.len;
    pos += decimalBuf(buf[pos..], phase).len;
    buf[pos] = '\t';
    pos += 1;
    pos += decimalBuf(buf[pos..], checksum).len;
    buf[pos] = '\n';
    pos += 1;
    writeR5(buf[0..pos]);
}
```

This appends `phase`, a tab, `checksum`, and a newline, then writes the completed marker line through the syscall helper.

```zig
// build_common's start.s entry stub calls `main`, so export under that name.
pub export fn main() noreturn {
```

The guest entry stub expects an exported symbol named `main`. `noreturn` is correct because the guest exits with a syscall.

```zig
    // Volatile reads make the input digests opaque to the optimizer, preventing
    // the compression chain from being constant-folded or deleted.
    var seed0: u32 = 0x12345678;
    var seed1: u32 = 0x9ABCDEF0;
```

The seeds are not secrets. They exist to generate deterministic but optimizer-resistant inputs.

```zig
    var left: poseidon2.Digest = undefined;
    var right: poseidon2.Digest = undefined;
    inline for (0..poseidon2.block_size) |i| {
        const left_seed = (@as(*volatile u32, &seed0)).*;
        const right_seed = (@as(*volatile u32, &seed1)).*;
        left[i] = .{ .value = @as(u32, @intCast((@as(u64, left_seed) + i + 1) % field.modulus)) };
        right[i] = .{ .value = @as(u32, @intCast((@as(u64, right_seed) + i + poseidon2.block_size + 1) % field.modulus)) };
    }
```

The digest arrays are filled at compile-time-unrolled indices. The volatile seed loads make the values opaque to the optimizer. The modulo keeps every limb inside the KoalaBear field.

```zig
    var i: u64 = 0;

    emitMark(0, 0);
    while (i < N) : (i += 1) {
        asm volatile ("" ::: .{ .memory = true });
    }
    emitMark(1, 0);
```

This is the baseline region. It has the same loop counter and branch structure as the real loop, but only a volatile empty assembly memory barrier inside. That prevents the loop from disappearing while still measuring mostly loop overhead.

```zig
    emitMark(10, 0);
    i = 0;
    while (i < N) : (i += 1) {
        left = poseidon2.compress(left, right);
    }
```

This is the measured region. The result feeds back into `left`, so each iteration depends on the previous one and the compiler cannot treat the calls as independent throwaway work.

```zig
    var checksum = left[0];
    inline for (left[1..]) |limb| {
        checksum = checksum.add(limb);
    }
    emitMark(11, checksum.value);
```

The checksum consumes all output limbs so the whole compression result stays live. Marker `11` closes the measured compression region and carries the checksum payload.

```zig
    asm volatile (
        \\li a0, 0
        \\li a7, 93
        \\ecall
    );
    unreachable;
}
```

The guest exits with status `0` using syscall `93`. `unreachable` tells Zig that execution should not continue after the exit syscall.

## `verifier-ray/bench/bench_compress/run.go`

This file is the host-side runner. It builds the guest, converts the ELF, runs zkc, parses markers, and prints a small table.

```go
const (
	elfToJSON      = "../../../arithmetization/src/test/scripts/elf_to_json_gen/main.go"
	zkcMain        = "../../../arithmetization/src/main/riscv/main.zkc"
	r5Bin          = "zig-out/bin/bench-compress"
	r5JSON         = "zig-out/bin/bench-compress.json"
	traceTailLimit = 40
	// n must match const N in main.zig - update both together.
	n = 10
)
```

These paths define the runner pipeline:

1. `zig build` produces `r5Bin`.
2. `elf_to_json_gen` converts `r5Bin` to `r5JSON`.
3. `zkc exec --fast` runs `r5JSON` with `zkcMain`.

The duplicated `n` is a small maintenance hazard called out by the comment.

```go
var baseline = struct {
	start uint64
	end   uint64
}{0, 1}

var compressPhase = struct {
	start uint64
	end   uint64
}{10, 11}
```

These mirror the marker IDs emitted by `main.zig`: baseline is `0 -> 1`, compression is `10 -> 11`.

```go
var (
	markRE  = regexp.MustCompile(`COMPRESS-MARK\s+([0-9]+)\s+([0-9]+)`)
	cycleRE = regexp.MustCompile(`clock cycle: ([0-9]+)`)
)
```

`markRE` extracts the benchmark's phase and payload. `cycleRE` tracks the current zkc cycle count so each marker can be assigned the cycle value most recently printed by zkc.

```go
type marker struct {
	cycle uint64
	value uint64
}

type traceStats struct {
	totalCycles uint64
	markers     map[uint64]marker
	tail        []string
}
```

`marker` stores a phase's cycle and payload. `traceStats` is the whole parse result, including a rolling output tail for useful failure messages.

```go
func main() {
	zkcBin := "zkc"
	if len(os.Args) > 1 {
		zkcBin = os.Args[1]
	}
```

By default the runner uses `zkc` from `PATH`, but you can pass a custom executable path as the first argument.

```go
	fmt.Fprintln(os.Stderr, "building R5 ELF...")
	if err := run("zig", "build", "--release=small"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
```

The first real step is building the benchmark ELF. Progress goes to stderr so stdout can remain reserved for the final result table.

```go
	fmt.Fprintln(os.Stderr, "converting ELF to zkc JSON...")
	if err := os.MkdirAll("zig-out/bin", 0o755); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	out, err := os.Create(r5JSON)
```

The runner ensures the output directory exists, then creates the JSON output file.

```go
	cmd := exec.Command("go", "run", elfToJSON, r5Bin, "0x00", "0x08800000")
	cmd.Stdout = out
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		_ = out.Close()
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := out.Close(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
```

The converter writes JSON to `out`. The explicit close check matters because file write errors can surface on close. On converter failure, the runner still closes the file but ignores that close error because the command already failed.

```go
	fmt.Fprintln(os.Stderr, "running zkc...")
	// --fast: execute for cycle counts only. The default (tracing) mode lowers
	// the word machine to a field machine for AIR constraints, which currently
	// panics under KOALABEAR_16 - the 32-bit `instruction` register exceeds the
	// 16-bit field register width and register splitting (--split) is incomplete
	// for multi-limb arithmetic. The benchmark only needs cycle counts, so the
	// trace/AIR path is unnecessary.
	zkcCmd := exec.Command(zkcBin, "exec", "--fast", r5JSON, zkcMain)
```

The runner uses `--fast` because this benchmark only needs execution cycle counts. The comment documents why the full tracing/AIR path is not currently viable for this guest.

```go
	stdout, err := zkcCmd.StdoutPipe()
	zkcCmd.Stderr = os.Stderr
	if err := zkcCmd.Start(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}

	stats, scanErr := parseTrace(stdout)
	waitErr := zkcCmd.Wait()
```

Stdout is parsed by this process. Stderr is passed through. After parsing stdout, the runner waits for zkc and keeps both scanner errors and process exit errors.

```go
	if scanErr != nil || waitErr != nil {
		if scanErr != nil {
			fmt.Fprintln(os.Stderr, scanErr)
		}
		if waitErr != nil {
			fmt.Fprintf(os.Stderr, "zkc exec failed: %v\n", waitErr)
		}
		if len(stats.tail) != 0 {
			fmt.Fprintf(os.Stderr, "last zkc output:\n%s\n", strings.Join(stats.tail, "\n"))
		}
		os.Exit(1)
	}
```

Failure reporting includes the last parsed zkc output lines, which makes debugging missing markers or zkc crashes much easier.

```go
	if stats.totalCycles == 0 {
		fmt.Fprintf(os.Stderr, "no cycles recorded; last zkc output:\n%s\n", strings.Join(stats.tail, "\n"))
		os.Exit(1)
	}
```

If no cycle line was ever parsed, the benchmark cannot produce meaningful output.

```go
	bStart, bStartOK := stats.markers[baseline.start]
	bEnd, bEndOK := stats.markers[baseline.end]
	if !bStartOK || !bEndOK {
		fmt.Fprintln(os.Stderr, "baseline markers not found in zkc output")
		os.Exit(1)
	}
	baselineDelta := bEnd.cycle - bStart.cycle
```

This extracts and computes the empty-loop overhead.

```go
	start, startOK := stats.markers[compressPhase.start]
	end, endOK := stats.markers[compressPhase.end]
	if !startOK || !endOK {
		fmt.Fprintln(os.Stderr, "compression markers not found in zkc output")
		os.Exit(1)
	}

	raw := end.cycle - start.cycle
	var net uint64
	if raw > baselineDelta {
		net = raw - baselineDelta
	}
```

This extracts the compression region, computes raw cycles, then subtracts baseline overhead. The `raw > baselineDelta` check avoids unsigned underflow if something weird happens.

```go
	fmt.Printf("\nN = %d\n", n)
	fmt.Printf("baseline (empty loop) = %d cycles (%.2f/iter), subtracted below\n\n",
		baselineDelta, float64(baselineDelta)/n)
	fmt.Printf("%-28s  %12s  %12s  %12s\n", "operation", "raw_cycles", "net_cycles", "cycles/call")
	fmt.Printf("%-28s  %12s  %12s  %12s\n", "---", "----------", "----------", "-----------")
	fmt.Printf("%-28s  %12d  %12d  %12.2f\n",
		"poseidon2 compress", raw, net, float64(net)/n)
}
```

The final report prints the iteration count, baseline overhead, and raw/net/cycles-per-call numbers.

```go
func parseTrace(r io.Reader) (traceStats, error) {
	stats := traceStats{markers: make(map[uint64]marker)}
	scanner := bufio.NewScanner(r)
	scanner.Buffer(make([]byte, 64*1024), 1024*1024)
```

`parseTrace` scans zkc output. The larger scanner buffer avoids failures on unusually long lines.

```go
	for scanner.Scan() {
		line := scanner.Text()
		stats.tail = appendTail(stats.tail, line)
		if m := cycleRE.FindStringSubmatch(line); m != nil {
			stats.totalCycles, _ = strconv.ParseUint(m[1], 10, 64)
			continue
		}
		if m := markRE.FindStringSubmatch(line); m != nil {
			phase, _ := strconv.ParseUint(m[1], 10, 64)
			value, _ := strconv.ParseUint(m[2], 10, 64)
			stats.markers[phase] = marker{cycle: stats.totalCycles, value: value}
		}
	}
	return stats, scanner.Err()
}
```

Every line is saved into the rolling tail. Cycle lines update `totalCycles`. Marker lines snapshot the current `totalCycles` into `stats.markers`.

```go
func appendTail(tail []string, line string) []string {
	if len(tail) == traceTailLimit {
		copy(tail, tail[1:])
		tail[len(tail)-1] = line
		return tail
	}
	return append(tail, line)
}
```

This keeps only the last 40 lines of zkc output.

```go
func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stderr
	cmd.Stderr = os.Stderr
	return cmd.Run()
}
```

Small helper for foreground commands where progress should be visible on stderr.

## `verifier-ray/src/crypto/poseidon2.zig`

This file contains the actual hot-path optimization. The math is intended to stay the same.

```diff
-    for (0..width / 4) |chunk| {
+    inline for (0..width / 4) |chunk| {
```

This change appears in `matMulM4InPlace` and twice in `matMulExternalInPlace`. Since `width` is compile-time-known, `inline for` lets Zig unroll the loops. That removes loop-control overhead and can make the RISC-V output simpler.

```zig
// Precomputed 2^{-n} mod p for the KoalaBear field (p = 2_130_706_433 = 2^31 - 2^24 + 1).
const inv2Exp1: field.Element = .{ .value = 1_065_353_217 };
const inv2Exp2: field.Element = .{ .value = 1_598_029_825 };
const inv2Exp3: field.Element = .{ .value = 1_864_368_129 };
const inv2Exp4: field.Element = .{ .value = 1_997_537_281 };
const inv2Exp5: field.Element = .{ .value = 2_064_121_857 };
const inv2Exp6: field.Element = .{ .value = 2_097_414_145 };
const inv2Exp7: field.Element = .{ .value = 2_114_060_289 };
const inv2Exp8: field.Element = .{ .value = 2_122_383_361 };
const inv2Exp9: field.Element = .{ .value = 2_126_544_897 };
const inv2Exp24: field.Element = .{ .value = 2_130_706_306 };
```

These constants replace calls like `halve()` and `mul2ExpNegN(8)`. Instead of repeatedly dividing by two at runtime, the code multiplies once by a known modular inverse.

```diff
-    for (state[1..]) |limb| {
-        sum = sum.add(limb);
+    inline for (1..width) |i| {
+        sum = sum.add(state[i]);
     }
```

The state summation is now indexed and unrolled. Same result, less runtime loop machinery.

```diff
-    state[3] = sum.add(state[3].halve());
-    state[4] = sum.add(state[4].mul(field.Element.init(3)));
+    state[3] = sum.add(state[3].mul(inv2Exp1));
+    state[4] = sum.add(state[4].mul(.{ .value = 3 }));
```

`halve()` becomes multiplication by `2^-1`. `field.Element.init(3)` becomes a direct literal because `3` is already a valid reduced field value.

```diff
-    state[6] = sum.sub(state[6].halve());
-    state[7] = sum.sub(state[7].mul(field.Element.init(3)));
+    state[6] = sum.sub(state[6].mul(inv2Exp1));
+    state[7] = sum.sub(state[7].mul(.{ .value = 3 }));
```

Same replacements, but in subtracting matrix entries.

```diff
-    state[9] = sum.add(state[9].mul2ExpNegN(8));
+    state[9] = sum.add(state[9].mul(inv2Exp8));
```

This changes "halve eight times" into one multiplication by precomputed `2^-8`.

```diff
     switch (width) {
         16 => {
-            state[10] = sum.add(state[10].mul2ExpNegN(3));
-            state[11] = sum.add(state[11].mul2ExpNegN(24));
-            state[12] = sum.sub(state[12].mul2ExpNegN(8));
-            state[13] = sum.sub(state[13].mul2ExpNegN(3));
-            state[14] = sum.sub(state[14].mul2ExpNegN(4));
-            state[15] = sum.sub(state[15].mul2ExpNegN(24));
+            state[10] = sum.add(state[10].mul(inv2Exp3));
+            state[11] = sum.add(state[11].mul(inv2Exp24));
+            state[12] = sum.sub(state[12].mul(inv2Exp8));
+            state[13] = sum.sub(state[13].mul(inv2Exp3));
+            state[14] = sum.sub(state[14].mul(inv2Exp4));
+            state[15] = sum.sub(state[15].mul(inv2Exp24));
         },
```

For the width-16 Poseidon2 matrix, every `mul2ExpNegN(k)` is replaced by `mul(inv2ExpK)`. This is the compression-relevant width.

```diff
         24 => {
-            state[10] = sum.add(state[10].mul2ExpNegN(2));
-            state[11] = sum.add(state[11].mul2ExpNegN(3));
-            state[12] = sum.add(state[12].mul2ExpNegN(4));
-            state[13] = sum.add(state[13].mul2ExpNegN(5));
-            state[14] = sum.add(state[14].mul2ExpNegN(6));
-            state[15] = sum.add(state[15].mul2ExpNegN(24));
-            state[16] = sum.sub(state[16].mul2ExpNegN(8));
-            state[17] = sum.sub(state[17].mul2ExpNegN(3));
-            state[18] = sum.sub(state[18].mul2ExpNegN(4));
-            state[19] = sum.sub(state[19].mul2ExpNegN(5));
-            state[20] = sum.sub(state[20].mul2ExpNegN(6));
-            state[21] = sum.sub(state[21].mul2ExpNegN(7));
-            state[22] = sum.sub(state[22].mul2ExpNegN(9));
-            state[23] = sum.sub(state[23].mul2ExpNegN(24));
+            state[10] = sum.add(state[10].mul(inv2Exp2));
+            state[11] = sum.add(state[11].mul(inv2Exp3));
+            state[12] = sum.add(state[12].mul(inv2Exp4));
+            state[13] = sum.add(state[13].mul(inv2Exp5));
+            state[14] = sum.add(state[14].mul(inv2Exp6));
+            state[15] = sum.add(state[15].mul(inv2Exp24));
+            state[16] = sum.sub(state[16].mul(inv2Exp8));
+            state[17] = sum.sub(state[17].mul(inv2Exp3));
+            state[18] = sum.sub(state[18].mul(inv2Exp4));
+            state[19] = sum.sub(state[19].mul(inv2Exp5));
+            state[20] = sum.sub(state[20].mul(inv2Exp6));
+            state[21] = sum.sub(state[21].mul(inv2Exp7));
+            state[22] = sum.sub(state[22].mul(inv2Exp9));
+            state[23] = sum.sub(state[23].mul(inv2Exp24));
         },
```

The same optimization is applied to the width-24 matrix path so both supported widths keep the same implementation style.

## `verifier-ray/src/crypto/poseidon2_constants.zig`

This file keeps the same constants but changes their spelling.

```diff
-    .{ field.Element.init(1954447561), field.Element.init(2103440337), ... },
+    .{ .{ .value = 1954447561 }, .{ .value = 2103440337 }, ... },
```

This pattern is applied to every round-key row. The numeric constants do not change.

Why it matters:

- `field.Element.init(x)` reduces `x` modulo the field and constructs an element.
- These generated constants are already valid field elements.
- Direct `.{ .value = x }` literals avoid doing construction/reduction work for constants that do not need it.

The rows with one nonzero value and fifteen zeroes are partial-round keys. The full rows at the top and bottom are full-round keys. Again, the PR changes representation, not the parameter values.

## `verifier-ray/src/field/koalabear.zig`

These helper methods are removed because Poseidon2 no longer calls them.

```diff
-    pub fn halve(self: Element) Element {
-        if ((self.value & 1) == 0) return .{ .value = self.value >> 1 };
-        return .{ .value = @as(u32, @intCast((@as(u64, self.value) + modulus) >> 1)) };
-    }
```

`halve` implemented multiplication by `2^-1` in the field. Even values could be shifted directly; odd values added the modulus before shifting so the result stayed field-correct. The PR replaces the Poseidon2 uses with `mul(inv2Exp1)`, so this helper becomes unused.

```diff
-    pub fn mul2ExpNegN(self: Element, n: u32) Element {
-        if (n > 32) unreachable;
-        var result = self;
-        var i: u32 = 0;
-        while (i < n) : (i += 1) {
-            result = result.halve();
-        }
-        return result;
-    }
```

`mul2ExpNegN` multiplied by `2^-n` by calling `halve` repeatedly. The hot Poseidon2 path now uses precomputed constants such as `inv2Exp8` and `inv2Exp24`, so this repeated-halving helper is removed too.

## Net Reading

The new benchmark gives a focused way to measure Poseidon2 compression in the RISC-V execution path. The implementation changes are classic hot-path cleanup: unroll known loops, replace repeated helper logic with constants, and avoid initializing values that are already canonical field elements.
