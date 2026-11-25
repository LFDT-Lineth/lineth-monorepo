//! The `zkvm_accelerators` module of an instance built with `-Dzkvm-provide`.

comptime {
    @compileError("lineth_accelerators: import zkvm_accelerators from the instance without -Dzkvm-provide, as every library does, so the program holds one copy of it; the -Dzkvm-provide instance supplies zkvm_provide for the final link");
}
