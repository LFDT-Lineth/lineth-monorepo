package serde

import (
	"fmt"
	"os"
	"reflect"
	"runtime"
	"sync"
	"syscall"
	"unsafe"
)

const Magic = 0x5A45524F

// FileHeader (32-byte) describes the file layout(fixed-size header) at the beginning of a serialized file.
// It provides enough metadata to validate the file, determine how to interpret the
// payload, and locate the serialized data region.
//
// All fields are written in a stable binary format and must remain backward-compatible
// across versions.
// IMPORTANT: DO NOT CHANGE THE LAYOUT (ordering of the fields) of this struct.
type FileHeader struct {
	// Magic is a constant identifier used to quickly validate that the file
	// is of the expected format (e.g. to reject random or corrupted input).
	Magic uint32

	// Version specifies the serialization format version.
	// It allows the reader to handle backward/forward compatibility
	// and apply version-specific decoding logic.
	Version uint32

	// PayloadType identifies the logical type of the serialized payload.
	// This is typically an application-defined enum or type ID that tells
	// the deserializer how to interpret the root object.
	PayloadType uint64

	// PayloadOff is the byte offset (from the start of the file/buffer)
	// where the payload begins.
	// This allows the header to be fixed-size while the payload layout evolves.
	PayloadOff int64

	// DataSize is the total size in bytes of the serialized payload data.
	// It can be used for bounds checking, mmap sizing, and integrity validation.
	DataSize int64
}

// InterfaceHeader represents the binary header for an interface and is specifically designed to be
// exactly 16 bytes to ensure the Offset field remains 8-byte aligned for efficient reading/writing.
//
// IMPORTANT:
// DO NOT CHANGE THE LAYOUT (ordering of the fields) of this struct.
// We perform `binary.Write` which is very literal i.e. writes the bytes of the struct fields in exact order and
// does not automatically insert padding for us like the Go compiler does automatically (inserts hidden padding bytes
// between fields in memory to align them) for structs in memory.
// We have to explicitly put the padding there if the file format requires a fixed size (like 16 bytes).
type InterfaceHeader struct {
	// 2 bytes: Unique identifier for the concrete type
	TypeID uint16

	// 1 byte: Number of pointer dereferences (e.g., ***T)
	PtrIndirection uint8

	// Reserved: 5 bytes used for two distinct purposes depending on TypeID.
	//
	// Case A — named type (TypeID < 0xFFFF, i.e. a type from IDToType):
	//   All 5 bytes are unused padding.  Their only role is to push 'Offset'
	//   to byte-offset 8 so the 64-bit Ref stays naturally aligned.
	//   Total header size: 2 + 1 + 5 + 8 = 16 bytes.
	//
	// Case B — composite type (TypeID == compositeTypeID == 0xFFFF):
	//   PtrIndirection is unused (always 0).
	//   Reserved[0]   composite kind:
	//                   compositeKindSlice      (1) — []ElemType
	//                   compositeKindMap        (2) — map[KeyType]ValType
	//                   compositeKindArray      (3) — [N]ElemType
	//                   compositeKindEmptyStruct(4) — struct{} (zero-size)
	//   Reserved[1:3] primary element TypeID field (little-endian uint16):
	//                   elem type for slice/array; key type for map; unused for EmptyStruct.
	//   Reserved[3:5] secondary field (little-endian uint16):
	//                   val TypeID field for map; array length for array; unused otherwise.
	//
	// Element TypeID fields (Reserved[1:3] and Reserved[3:5] for map values) encode
	// both the registry TypeID and a pointer-indirection level:
	//   bits  0–13  base TypeID from IDToType  (supports up to 16 383 registered types)
	//   bits 14–15  pointer indirection: 0=T  1=*T  2=**T  3=***T
	// The sentinel compositeElemEmptyStruct (0xFFFF) in a TypeID field represents
	// the zero-size struct{} type, which has no entry in the TypeToID registry.
	// Array length (Reserved[3:5] for arrays) is always raw, never a TypeID field.
	//
	// Offset points to the serialised slice/map/array data, exactly as it would
	// be for a top-level value of that type.  For compositeKindEmptyStruct, Offset
	// is always 0 (struct{} has no data).
	//
	// Case B is used when the serialized data contains boxed composites such as
	// interface{}(map[string]T), interface{}([]T), interface{}([N]T), or interface{}(struct{})
	// because those types cannot be registered in the TypeToID registry.
	Reserved [5]uint8

	// 8 bytes: File or memory offset to the actual data
	Offset Ref
}

// Ref is a 8-byte offset in the serialized buffer
type Ref int64

func (r Ref) IsNull() bool { return r == 0 }

// FileSlice mirrors the struct of a Go slice header {Data uintptr, Len int, Cap int} conceptually
// for zero-copy slice reconstruction. Instead of Data (uintptr), we store the offset of the slice data
// in the serialized buffer. Notice the first two attributes shared similarties with Go string header
// representation {Data uintptr, Len int} and hence can also be used for zero-copy string construction.
//
// IMPORTANT: The ordering of the attributes MUST NOT be changed (Offset, Len, Cap) since the decoder relies on this format.
type FileSlice struct {
	// 8 Byte offset in the serialized buffer - beginning of the cursor
	Offset Ref

	// Number of elements in the slice
	Len int64

	// Original capacity (used to restore slice header)
	Cap int64
}

// compositeTypeID is a sentinel TypeID stored in InterfaceHeader.TypeID to
// signal that the interface holds a composite type (slice, map, array, or struct{})
// rather than a named type from the TypeToID registry.  0xFFFF is safely above the
// current maximum registered ID and cannot be confused with a real entry.
const compositeTypeID = uint16(0xFFFF)

// Composite kind codes stored in InterfaceHeader.Reserved[0].
const (
	compositeKindSlice       = uint8(1)
	compositeKindMap         = uint8(2)
	compositeKindArray       = uint8(3)
	compositeKindEmptyStruct = uint8(4) // struct{} directly behind an interface
)

// compositeElemEmptyStruct is a sentinel stored in the 2-byte TypeID sub-fields
// of InterfaceHeader.Reserved (Reserved[1:3] or Reserved[3:5]) to represent
// struct{} as a composite element, map key, or map value type.  0xFFFF is safe
// because a valid TypeID sub-field either equals this sentinel or has bits 0–13
// within the current registry range (< 16 384).
const compositeElemEmptyStruct = uint16(0xFFFF)

// compositeTypeMask and compositeIndirectionShift split a 2-byte composite
// TypeID sub-field into a base TypeID (bits 0–13, max 16 383) and a pointer
// indirection level (bits 14–15, range 0–3).
//
// This lets [](*Foo), map[K](*V), and [N](*T) be encoded without registering
// the pointer types, as long as the base types are in TypeToID.
const (
	compositeTypeMask         = uint16(0x3FFF)
	compositeIndirectionShift = uint(14)
)

// emptyStructType is the reflect.Type for struct{}, the zero-size empty struct.
// It is used as a sentinel in the composite encoding path.
var emptyStructType = reflect.TypeOf(struct{}{})

func SizeOf[T any]() int64 {
	var z T
	return int64(unsafe.Sizeof(z))
}

// MappedFile  maps the underlying file (READ-ONLY) into the process's virtual address space
// without loading them into the Go heap.
// It acts essentially like a "Manager" whose job is to own both the memory region and the file handle.
type MappedFile struct {
	file *os.File
	data []byte

	// Ensures Close logic runs exactly once
	once sync.Once
}

// Data returns the underlying byte slice of the memory map.
// IMPORTANT: This slice is only valid as long as the MappedFile has not been closed.
func (mf *MappedFile) Data() []byte {
	return mf.data
}

// openMappedFile opens a file and maps it into the process memory.
// It sets up a finalizer to ensure that kernel resources (file descriptors
// and memory maps) are released even if the caller forgets to call Close().
func openMappedFile(path string) (*MappedFile, error) {

	// Note: We don't call defer f.Close() here because we are handing over ownership of the file to the MappedFile struct.
	// The file's life is now tied to the memory map's life. They live together, and they die together when mf.Close() is called.
	// If we used defer f.Close() inside the OpenMappedFile function, the file would be closed the moment the function returns.
	// This would leave our MappedFile struct in a "half-alive" state where it has the data but has lost its connection to the
	// underlying file object. See `MappedFile` struct description for more details.
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}

	info, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("failed to stat file: %w", err)
	}

	size := info.Size()
	if size == 0 {
		// Mapping a zero-length file is generally not allowed by the OS.
		return &MappedFile{file: f, data: nil}, nil
	}

	// PROT_READ: Memory is read-only.
	// MAP_SHARED: The mapping is shared between multiple processes sharing the same physical RMA.
	// Allows mem sync (changes on disk are reflected in memory) and has lower OS overhead compared to MAP_PRIVATE.
	data, err := syscall.Mmap(int(f.Fd()), 0, int(size), syscall.PROT_READ, syscall.MAP_SHARED)
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("mmap failed: %w", err)
	}

	mf := &MappedFile{
		file: f,
		data: data,
	}

	// The Safety Net: If the MappedFile object becomes unreachable, the GC will
	// trigger this finalizer to prevent resource leaks.
	runtime.SetFinalizer(mf, func(m *MappedFile) {
		_ = m.Close()
	})

	return mf, nil
}

// Close explicitly unmaps the memory and closes the underlying file.
// This method is idempotent and thread-safe.
func (mf *MappedFile) Close() error {
	var err error

	// If different components might share a reference to the same MappedFile, then Component A might call Close()
	// and then Component B calls Close(), the syscall.Munmap might error out or, worse, unmap a different memory
	// region that the OS just reassigned. sync.Once ensures the cleanup logic runs exactly once, regardless of
	// how many times it is called during the runtime.
	mf.once.Do(func() {
		// 1. Remove the finalizer so it doesn't run again later.
		runtime.SetFinalizer(mf, nil)

		// 2. Unmap the memory if it exists.
		if mf.data != nil {
			err = syscall.Munmap(mf.data)

			// Invalidate the slice immediately -defensive programming practice. If any other part of the code
			// attempts to access mf.Data() after a close, it will get a nil slice rather than attempting to read
			// from an unmapped address which would cause a hard crash.
			mf.data = nil
		}

		// 3. Close the file descriptor.
		if mf.file != nil {
			if fErr := mf.file.Close(); fErr != nil && err == nil {
				err = fErr
			}
		}
	})
	return err
}
