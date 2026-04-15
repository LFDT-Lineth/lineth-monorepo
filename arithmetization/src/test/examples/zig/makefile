OUTDIR=zig-out/bin

# Invokes zig build to compile the Zig code in src/ and outputs the binary to $(OUTDIR)/example
$(OUTDIR)/example: src/*.zig
	zig build

clean:
	rm -rf zig-out
