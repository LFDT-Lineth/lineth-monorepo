const l2_execution_ssz = @import("l2_execution_ssz");

pub const Output = [l2_execution_ssz.OUTPUT_SIZE]u8;

pub const Result = union(enum) {
    accepted: Output,
    rejected: ?anyerror,
};
