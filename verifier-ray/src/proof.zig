const field = @import("field/koalabear.zig");

pub const Commitment = [8]field.Element;

pub const Proof = struct {
    commitments: []const Commitment,
    public_inputs: []const field.Element,
    proof_bytes: []const u8,

    pub fn empty() Proof {
        return .{
            .commitments = &.{},
            .public_inputs = &.{},
            .proof_bytes = &.{},
        };
    }
};
