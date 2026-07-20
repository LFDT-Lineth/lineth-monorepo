# Keccak test file hierarchy

The following files are responsible for testing and benchmarking the keccak implementation in zkc against others.

```
src
├── main
│   ├── lib
│   │   ├── keccak
│   │   │   ├── constants.zkc
│   │   │   ├── impl.zkc       # zkc implementation of keccak
│   │   │   └── utils.zkc
│   │   └── README.md
│   └── wrappers
│       ├── custom_std.zig
│       ├── keccak_provide.zig # provider for either zkc implementation or native implementation of keccak at compile time
│       ├── keccak.zig         # wrapper for the zkc implementation of keccak
│       └── root.zig           # aggregates all zig files under wrappers and provides a single entry point for zig tests
└── test
    ├── common_inputs
    │   └── keccak.all
    └── zig
        ├── build.zig          # build file for zig tests importing root.zig
        ├── build.zig.zon      # dependency file for zig tests
        └── src
            └── keccak
                ├── keccak_batched_input_native.zig       # test using provider with batched inputs
                ├── keccak_native.zig                     # test using provider with trivial inputs
                ├── keccak_zkc.zig                        # test using zkc wrapper with trivial inputs
                └── README.md                             # You are here
```