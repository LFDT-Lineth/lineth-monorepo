const fiat_shamir = @import("crypto/fiat_shamir.zig");

pub const Runtime = struct {
    transcript: fiat_shamir.Transcript,
    current_round: usize,

    pub fn init() Runtime {
        return .{
            .transcript = fiat_shamir.Transcript.init(),
            .current_round = 0,
        };
    }

    pub fn advanceRound(self: *Runtime) void {
        self.current_round += 1;
    }
};
