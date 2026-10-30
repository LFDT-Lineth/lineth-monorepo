pub const Output = []const u8;

pub const Result = union(enum) {
    accepted: Output,
    rejected: ?anyerror,
};
