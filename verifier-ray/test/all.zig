comptime {
    @setEvalBranchQuota(2_000_000);
    _ = @import("verifier_test.zig");
    _ = @import("pcs_average_binding_test.zig");
    _ = @import("pcs_input_tree_shape_test.zig");
    _ = @import("proof_abi_test.zig");
    _ = @import("image_relocation_test.zig");
    _ = @import("riscv_proof_image_test.zig");
    _ = @import("riscv_system_test.zig");
    _ = @import("devmode_system_test.zig");
}
