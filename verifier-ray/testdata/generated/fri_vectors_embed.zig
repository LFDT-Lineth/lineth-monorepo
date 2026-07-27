// Hand-written, not generated. Re-exports the sibling JSON file's raw bytes
// as a module (test_fri_vectors, wired in build.zig) that test/fri_test.zig
// imports.
pub const raw = @embedFile("fri_vectors.json");
