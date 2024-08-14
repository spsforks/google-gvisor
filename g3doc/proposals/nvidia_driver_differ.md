# Nvidia Driver Differ Tool

Status as of 2024-08-14: Completed. To get an overview of what was ultimately
implemented, check out the presentation in
`g3doc/presentations/nvidia_tooling.pdf`.

## Overview

Currently, any new version of an Nvidia driver can come with changes to its
ioctl structs. If these structs are currently supported in nvproxy, then we're
required to copy these changes in nvproxy in order to support this new version.
The issue, however, is that finding these changes is both difficult and tedious;
as such, we would like to have a tool that can do the bulk of this work for us.

This document goes over some design proposals of how this tool should be built,
especially how it should interface with nvproxy.

## Problem Statement

Let's say we want to add support for a new driver version B. At a high level,
the work of this tool can be broken into the following steps:

1.  Find the nearest ancestor version, A, that nvproxy supports. We support
    multiple major version numbers, so these versions form a tree instead of a
    simple line of dependencies.
2.  Get the list of currently used structs in nvproxy for version A.
3.  For each struct, compare its definition in versions A and B. Report any
    differences–type, number, and ordering of fields all matter.

The biggest roadblock to implementing this is that the only immediate
information we have in nvproxy is the ioctl calls that are supported. Some ioctl
calls have corresponding structs defined, but:

-   We have no way to directly get this mapping; we would have to analyze the
    code with some AST parser.
-   Some ioctls have multiple structs defined due to changes across version; we
    need some way to know which struct nvproxy is using for version A.
-   Some ioctls simply don't have struct definitions written out, since they're
    simply passed by copying a given `size` bytes. Most control commands are
    like this.

Additionally, we also care about nested structs. Some ioctls, such as
`UVM_ALLOC_SEMAPHORE_POOL_PARAMS`, contain arrays of other structs whose
definitions can also change:

```go
type UVM_ALLOC_SEMAPHORE_POOL_PARAMS struct {
    Base               uint64
    Length             uint64
    PerGPUAttributes   [UVM_MAX_GPUS]UvmGpuMappingAttributes
    GPUAttributesCount uint64
    RMStatus           uint32
    Pad0               [4]byte
}

type UvmGpuMappingAttributes struct {
    GPUUUID            NvUUID
    GPUMappingType     uint32
    GPUCachingType     uint32
    GPUFormatType      uint32
    GPUElementBits     uint32
    GPUCompressionType uint32
}
```

This means we not only have to map ioctl calls to their corresponding structs,
but also (recursively) parse their fields to see if there are nested structs.

## Proposal

We can split this tool into two parts. The first part can be a Go tool that is
built against nvproxy and finds the list of structs for version B. Once we have
a specific list of structs to look up, we can pass that to a C++ tool that uses
[Clang's C++ AST Matcher API](https://clang.llvm.org/docs/LibASTMatchersReference.html)
to find the corresponding struct definitions in the driver source code. These
definitions are then passed back to the Go tool, which does the necessary
diffing and reporting back to the user.

### Fetching struct names from nvproxy

The primary problem to tackle on the Go side is how we get the list of struct
names nvproxy depends on for a given version B. Since we want to allow
versioning of these struct names, we can extend the existing `driverABI` struct
to also capture these names.

However, almost every normal use case of `driverABI` will not need to use these
names, and we don’t want them sitting around wasting memory. Thus, we can add a
`getStructNames` function to `driverABI` that will construct and return the list
of relevant names only when they are needed. It should look like this:

```go
type driverABI struct {
    frontendIoctl   map[uint32]frontendIoctlHandler
    uvmIoctl        map[uint32]uvmIoctlHandler
    controlCmd      map[uint32]controlCmdHandler
    allocationClass map[nvgpu.ClassID]allocationClassHandler

    getStructNames driverStructNamesFunc
}

type driverStructNamesFunc func() *driverStructNames

type driverStructNames struct {
    frontendNames   map[uint32][]string
    uvmNames        map[uint32][]string
    controlNames    map[uint32][]string
    allocationNames map[nvgpu.ClassID][]string
}
```

The fields in `driverStructNames` map every ioctl to a list of struct names that
it depends on (this is a list to support the case of nested structs). By
explicitly mapping each struct name to their corresponding ioctl, it should make
this list much easier to maintain. We can compare against the ioctls included in
the ABI map to ensure every ioctl call is accounted for in each version. It also
makes it easier to modify definitions for a specific ioctl number due to a
version change.

There are a few cases to consider when generating the list of names for an
ioctl:

-   For ioctls with a struct defined in nvproxy, we can provide a function
    `getStructName(any)` that takes a struct and returns its corresponding
    driver name. How this should be done is discussed further below.
-   For ioctls without a struct defined in nvproxy, we can directly write the
    corresponding struct names. We can also introduce a function
    `simpleIoctl(name)` that simply returns a slice with one element, to make
    this more explicit.
-   Finally, there are some ioctls (or maybe just `NV_ESC_RM_ALLOC`) that allow
    multiple types of parameters. In this case, we can simply merge the
    corresponding lists for each parameter type.

Concretely, this would look something like this:

```go
driverStructNames{
    frontendNames: map[uint32][]string{
        NV_ESC_RM_ALLOC_MEMORY: append(getStructName(NVOS21Parameters{}), getStructName(NVOS64Parameters{})...),
    },
    uvmNames: map[uint32][]string{
        UVM_ALLOC_SEMAPHORE_POOL: getStructName(UVM_ALLOC_SEMAPHORE_POOL_PARAMS{})
    },
    controlCmd: map[uint32][]string{
        NV2080_CTRL_CMD_GPU_GET_NAME_STRING: simpleIoctl("NV2080_CTRL_GPU_GET_NAME_STRING_PARAMS"),
    },
    allocationNames: map[nvgpu.ClassID][]string{
        NV01_MEMORY_SYSTEM: getStructName(NV_MEMORY_ALLOCATION_PARAMS{}),
    },
}
```

Looking specifically now at `getStructName`, there are a few ways in which it
can be implemented:

1.  We can require that struct names in nvproxy are exactly the same as their
    counterpart in the Nvidia driver. This way, we can use Go's
    [reflect](https://pkg.go.dev/reflect) package to simply read the name of the
    struct being passed in.

    To handle versioning changes, we can agree on some suffix format. For
    example, everything after a double underscore is ignored. This way, we can
    define both `PARAMS` and `PARAMS__V550`.

2.  We can introduce struct tags that specify the name of the corresponding
    struct in the Nvidia driver code, which would always sit on the first field.
    This could look something like this:

    ```go
    type IoctlFreeOSEvent struct {
      HClient Handle   `nvproxy:"nv_ioctl_free_os_event_t"`
      HDevice Handle
      FD      uint32
      Status  uint32
    }
    ```

    This struct tag can be read using reflect. For structs that are named the
    same between nvproxy and the Nvidia driver, we can also have a convenient
    `nvproxy:"same"` case that simply uses the struct’s name.

3.  Instead of using a struct tag, we can use a struct comment similar to `//
    +marshal` or `// +stateify`. An external tool would then run on the nvproxy
    package, find each struct with the struct comment, and implement an
    interface that reports back the corresponding driver name.

    ```go
    type NvidiaDriverStruct interface {
      func GetDriverName() string
    }

    // +nvproxy nv_ioctl_free_os_event_t
    type IoctlFreeOSEvent struct {
      HClient Handle
    HDevice Handle
      FD      uint32
      Status  uint32
    }

    // Auto-generated
    func (s IoctlFreeOSEvent) GetDriverName() string {
      return "nv_ioctl_free_os_event_t"
    }
    ```

    The use of an external tool makes this method a lot more involved, and
    potentially expensive to maintain. The main benefit is that it is a better
    convention than requiring tags on the first field. The code will also be
    similar to `go_marshal` or `go_stateify`, so we can probably copy over a lot
    of it. Specifically, we would likely want a similar BAZEL definition, and
    the code to collect all annotated types in
    `Generator.collectMarshallableTypes` can be the same.

Comparing these three ideas, numbers 1 and 2 are definitely the easiest to
implement. Idea 2 will be more robust as well, since we don’t have to worry
about Nvidia driver structs potentially having double underscores or whatever
separator we decide on. As such, I think **implementing idea 2 would be the best
option**; if we really care about maintaining convention, we can switch to idea
3 after an MVP.

There is also the problem of nested structs that we need to address. Although we
can tackle this on the Go side, it would be hard to maintain for the simple
structs that are not defined in nvproxy, as we would have to manually check if
they have nested structs and write down what they are. Thus, it would be easier
to make the C++ Clang tool do this, and simply have the Go tool find the list of
all top-level structs.

### C++ Clang parser

Once we have a list of struct names to verify, we can locally clone the code for
both versions A and B. From here, we can use Clang's C++ AST Matcher API to
generate an AST and find the struct definitions given the name.

The Clang API includes the ability to quickly set up command line tools to run
the AST matcher; this
[tutorial](https://clang.llvm.org/docs/LibASTMatchersTutorial.html) in the
documentation covers everything this tool needs to do. Out of the box, it takes
in a source file, and allows you to run any set of matchers on that source file.
This means we can create a small C++ file that `#include`s all the header files
that contain struct definitions, similar to what
[Geohot does with his sniffer](https://github.com/geohot/cuda_ioctl_sniffer/blob/master/pstruct/include.cc).
Clang will automatically expand these `#include`s, so any struct defined in
there will be matchable.

In the driver source code, all structs are named via a `typedef`. This means
when we’re given a struct name, we should match against a `typedef` with that
name, and then look at the struct type aliases. This is done with the following
Clang matcher expression:

```c++
typedefDecl(
  allOf(
    hasName(struct_name),
    // Match and bind to the struct declaration.
    hasType(
      // Need to specify elaboratedType, otherwise hasType
      // will complain that the type is ambiguous.
      elaboratedType(
        hasDeclaration(recordDecl().bind("struct_decl"))
      )
    )
  )
).bind("typedef_decl");
```

A few structs in the driver share the same definition, so they are defined via
`typedef`s to each other. These structs will not get matched by the expression
above; instead, we need to check that the typedefDecl is mapped to another
`typedefDecl` rather than a `recordDecl`, like so:

```c++
// Matches definitions like
// typedef NV906F_CTRL_GET_CLASS_ENGINEID_PARAMS NVC36F_CTRL_GET_CLASS_ENGINEID_PARAMS;
typedefDecl(
  allOf(
    hasName(struct_name),
    // Match and bind to the struct declaration.
    hasType(
      // Need to specify elaboratedType, otherwise hasType
      // will complain that the type is ambiguous.
      elaboratedType(
        hasDeclaration(typedefDecl())
      )
    )
  )
).bind("typedef_decl");
```

These cases can be recorded as type aliases in the JSON output, described in
more detail below.

Running this matcher will allow us to access the `clang::RecordDecl` node
corresponding to the struct definition. From here, we can simply iterate through
the fields and get their name and type using
`clang::FieldDecl.getNameAsString()` and
`clang::FieldDecl->getType().getAsString()`.

One edge case is if the field type is an anonymous struct or union, like so:

```c++
typedef struct NV2080_CTRL_GPU_GET_NAME_STRING_PARAMS {
    NvU32 gpuNameStringFlags;
    union {
        NvU8  ascii[NV2080_GPU_MAX_NAME_STRING_LENGTH];
        NvU16 unicode[NV2080_GPU_MAX_NAME_STRING_LENGTH];
    } gpuNameString;
} NV2080_CTRL_GPU_GET_NAME_STRING_PARAMS;
```

Trying to get the type name directly will yield an auto generated name that
includes the absolute file path, which is not easy to compare. This means we
need to check if the given type has a name, and create a standardized name if
not. Luckily, we can determine if this is the case using
`clang::Type.hasUnnamedOrLocalType`.

The standardized name can be of the form `PARENT_RECORD::FIELD_t`; for example,
`NV2080_CTRL_GPU_GET_NAME_STRING_PARAMS::gpuNameString_t` for the example above.

The Clang tool should also recurse into any nested structs. Since we already
have the `clang::QualType` of each field, there are two few cases we need to
consider:

-   If the type is an array, we should recurse on the array element type.
-   If the type is a struct, we can recurse on its `clang::RecordDecl` node.

Along the way, we can also record the true type of any other field types we
find. For example, if we see the simple type `NvHandle`, we can use
`clang::QualType.getCanonicalType()` to record that `NvHandle` is an `unsigned
int`, in case these simple types ever change as well.

Finally, the Go side needs some way to interface with the C++ Clang side. To
make things simple, we can pass in the list of structs and output the struct
definitions using JSON. Overall, interfacing with the parser would go something
like this:

```bash
./driver_ast_parser --structs=structs.json source_file.cc
```

Input:

```json
{
    "structs": ["STRUCT", "NAMES", "HERE", ...]
}
```

Output:

```json
{
    // Named records since this captures both structs and unions we find
    "records": {
        "STRUCT_NAME": {
            "fields": [
                {"name": "field1", "type": "int"},
                {"name": "field2", "type": "NvHandle"}
            ],
            "source": "/path/to/source/file.cc:line_number"
        },
        ...
    },
    // All the typedefs that we find
    "aliases": {
        "NvHandle": "unsigned int"
    }
}
```

### Remaining details

Beyond the nvproxy changes and C++ Clang parser, there are a few other details
to work out.

The first is actually getting the driver source code locally for Clang to parse
through. Since we only care about released versions of the driver, we can clone
them using the following git command:

```bash
git clone -b $VERSION --depth 1 https://github.com/NVIDIA/open-gpu-kernel-modules.git $SAVE_PATH
```

Next, we need to create the source file for the parser to reference. As
mentioned above, the easiest way to make this would be to create a single C++
file that `#include`s every relevant driver header file with struct definitions.
Finding these relevant header files does require hard-coding some paths;
however, the driver file structure seems very stable, so it is unlikely that
we’d need any kind of version control over these paths as well. Currently, the
list of header files is:

-   Frontend:
    -   `src/common/sdk/nvidia/inc/nvos.h`
    -   `src/nvidia/arch/nvalloc/unix/include/nv-ioctl.h`
    -   `src/nvidia/arch/nvalloc/unix/include/nv-unix-nvos-params-wrappers.h`
-   UVM:
    -   `kernel-open/nvidia-uvm/uvm_ioctl.h`
    -   `kernel-open/nvidia-uvm/uvm_linux_ioctl.h`
-   Control commands:
    -   `src/common/sdk/nvidia/inc/ctrl/*.h`
    -   `src/common/sdk/nvidia/inc/ctrl/*/*.h`
-   Allocation classes:
    -   `src/common/sdk/nvidia/inc/class/*.h`

These header files also #include from other header files. The include paths for
these files are as follows:

-   Non-UVM:
    -   `src/common/sdk/nvidia/inc`
    -   `src/common/shared/inc`
    -   `src/nvidia/arch/nvalloc/unix/include`
-   UVM:
    -   `kernel-open/common/inc`

Unfortunately, there are many duplicate definitions between non-UVM and UVM
files. This means that we need to run the C++ parser **twice** per driver
version, and generally keep the non-UVM and UVM sources separate.

To let Clang know about these include paths, we need to use a
`compile_commands.json` file. The format of this file is documented
[here](https://clang.llvm.org/docs/JSONCompilationDatabase.html), but for our
purposes, the structure will always look as follows:

```json
[
    { "directory": "source/file/directory",
      "arguments": ["clang", "-I", "include/path/1", "-I", "include/path/2", ..., "non_uvm_source_file.cc"],
      "file": "non_uvm_source_file.cc"
    },
    // repeated for UVM source file
]
```

Clang **requires** that the file is called `compile_commands.json`, and it
assumes that it exists in the same directory as the file being parsed. As such,
our differ will likely need to create a temporary directory when running, with
the following format:

```
temp_dir
  \ driver_source_dir
  \ compile_commands.json
  \ non_uvm_source_file.cc
  \ uvm_source_file.cc
```

Altogether, the differ will behave as follows:

1.  Get the versions A and B that we will be diffing.
    -   Initially, these can just be passed in via command line arguments. In
        the future, we should only take in the new version B, and automatically
        figure out the latest version A that nvproxy supports.
2.  Query nvproxy for the list of structs it depends on for version A.
3.  Save the list of structs to a temporary JSON file.
4.  For each version:
    1.  Create a temporary directory.
    2.  Clone the git repo for the current version.
    3.  Match the list of header file paths to create `non_uvm_source_file.cc`
        and `uvm_source_file.cc`.
    4.  Create `compile_commands.json`.
    5.  Run the C++ parser on both source files, referring to the list of
        structs saved above.
    6.  Combine outputs from two parser runs.
5.  Compare the combined outputs of each version, reporting any differences
    found.

### Tests

Do you love tests? Well luckily for you, there are a few tests we should build
around this diffing tool.

First, we can introduce a few continuous tests that ensure we keep our list of
struct names up to date. For every version covered by nvproxy’s ABI tree, one
test can check whether there are any supported ioctls that are missing in
`driverStructNames`, and another can run the parser to verify that every struct
name reported in `driverStructNames` actually exists in the driver source code.

We should also have a continuous test that uses this tool to verify that nvproxy
is correct. Rather than trying to use the differ, however, it might be easier to
just use the C++ Clang parser and verify individual versions of the ABI. This
test should take the `driverStructNames` for a given version, find the
corresponding driver struct definitions, and then match it against the nvproxy
equivalent struct.

This would require us to augment our `driverABI` mapping to also return struct
instances, which we can then use Go’s `reflect` library to read. Specifically,
instead of mapping ioctls to `[]strings`, we can map them to slices of strings
and struct instances, like so:

```go
type DriverStruct struct {
    Name          string
    Instance    any
}

type driverStructNames struct {
    frontendNames   map[uint32][]DriverStruct
    uvmNames        map[uint32][]DriverStruct
    controlNames    map[uint32][]DriverStruct
    allocationNames map[nvgpu.ClassID][]DriverStruct
}
```

This way, we can look at the nvproxy instance when verifying a struct.

When verifying a struct, there are a few cases that can happen. The first case
is when we have a simple struct (`DriverStruct.Instance == nil`). Here, we can
look for a few signs in the driver definition to see if the struct is actually
simple:

-   If a field is `NvP64`
-   If a field name ends in `"fd"`

Another case is when nvproxy defines a struct for a parameter, but the Nvidia
driver uses a simple type alias. `NvHandle` seems to be the only example of
this:

```go
// nvproxy definition
type Handle struct {
    Val uint32 `nvproxy:"NvHandle"`
}
```

```c++
// Driver definition
typedef NvU32 NvHandle;
```

To verify this, we can compare the sizes of the two types and ensure they remain
identical.

The last case is when we are comparing two struct definitions between nvproxy
and the driver. When thinking about how this can be done, there are a few
complications to keep in mind:

-   Sometimes nvproxy flattens structs or unions. For example:

    ```go
    // nvproxy definition
    type NV0000_CTRL_OS_UNIX_EXPORT_OBJECT struct {
      Type uint32 // enum NV0000_CTRL_OS_UNIX_EXPORT_OBJECT_TYPE
      // These fields are inside union `data`, in struct `rmObject`.
      HDevice Handle
      HParent Handle
      HObject Handle
    }
    ```

    ```c++
    // Driver definition
    typedef struct NV0000_CTRL_OS_UNIX_EXPORT_OBJECT {
        NV0000_CTRL_OS_UNIX_EXPORT_OBJECT_TYPE type;

        union {
            struct {
                NvHandle hDevice;
                NvHandle hParent;
                NvHandle hObject;
            } rmObject;
        } data;
    } NV0000_CTRL_OS_UNIX_EXPORT_OBJECT;
    ```

-   Some unions are simply represented by `[n]byte` fields.

-   Some nvproxy structs use struct embedding, which we have to keep in mind
    when looking through the fields using `reflect`.

    ```go
    type NV_MEMORY_ALLOCATION_PARAMS_V545 struct {
      NV_MEMORY_ALLOCATION_PARAMS `nvproxy:"NV_MEMORY_ALLOCATION_PARAMS"`
      NumaNode                    int32
      _                           uint32
    }
    ```

-   nvproxy structs can have additional fields added for padding.

To alleviate the problem of nested or flattened structs, we can pre-flatten our
struct definitions before comparing them. This will yield us an array of fields
for both sides. For example, we would flatten

```go
type NVOS57Parameters struct {
    HClient     Handle `nvproxy:"NVOS57_PARAMETERS"`
    HObject     Handle
    SharePolicy RS_SHARE_POLICY
    Status      uint32
}

type RS_SHARE_POLICY struct {
    Target     uint32
    AccessMask RS_ACCESS_MASK
    Type       uint16
    Action     uint8
    Pad        [1]byte
}

type RS_ACCESS_MASK struct {
    Limbs [SDK_RS_ACCESS_MAX_LIMBS]uint32 // RsAccessLimb
}
```

into

```go
[
  HClient     Handle,
  HObject     Handle,
  Target      uint32,
  Limbs       [SDK_RS_ACCESS_MAX_LIMBS]uint32,
  Type        uint16,
  Action      uint8,
  Pad         [1]byte,
  Status      uint32,
]
```

Next, we want to compare fields that **have the same offset**. Due to padding
and union types, multiple nvproxy fields may correspond to a single driver
field; however, as long as each driver field has a corresponding nvproxy field
at the same offset, the extraneous fields do not matter. The following
pseudo-code accomplishes all of this:

```
doStructsMatch(nvproxyType, driverType) -> bool
  if nvproxyType.Size != driverType.Size:
    return false

  nvproxyFields = Flatten(nvproxyType)
  driverFields = Flatten(driverType)

  for each ith field in driverFields:
    find the jth field in nvproxyFields with the same offset
    if such a field doesn't exist:
      return false

    if !doTypesMatch(nvproxyFields[j].Type, driverFields[i].Type):
      return false
  return true

doTypesMatch(nvproxyType, driverType) -> bool
  if driverType is an array:
    if nvproxyType is not an array of the same length:
      return false
    recurse on the base type of each array

  // These are special types that nvproxy has type definitions for
  Check the following mappings from driverType -> nvproxyType:
    NvHandle -> Handle
    NvP64 -> P64
    NvProcessorUuid -> NvUUID

  // E.g. NvU32 aliases unsigned int
  if driverType has an alias:
    driverType = alias
  Check the following mappings from driverType -> nvproxyType:
    char -> byte
    unsigned char -> uint8
    short -> int16
    unsigned short -> uint16
    int -> int32
    unsigned int -> uint32
    long long -> int64
    unsigned long long -> uint64
    enum _ -> uint32
    union -> [n]byte
    struct -> doStructsMatch(nvproxyType, driverType)
```

This all requires some changes on the C++ parser side as well. Namely, we need
to report sizes for `records` and `aliases`, whether a record is a union type,
and offsets for each record field. This can be done with
`clang::ASTContext.getTypeInfo`, `clang::TagDecl.isUnion`, and
`clang::ASTContext.getFieldOffset` respectively.

## Future Work

### Interpreting struct field names

Occasionally, driver structs might change not by introducing a new field, but by
changing the purpose of an existing field. For example, a previously reserved
integer field might now be used as a file descriptor field, meaning that nvproxy
would need to add special handling for it. Although we already report changes in
field names, we could also have the differ report any code changes it thinks is
necessary. This could behave similarly to the verification test, which looks at
simple clues such as `NvP64` types or fields ending in `"fd"`.

### Check ABI ranges for nvproxy

Currently, we only support specific versions of the Nvidia driver within
nvproxy. However, many intermediate versions likely do not have any breaking
changes, and it is detrimental to users if we force them to only use some
specific driver versions. If we can use this differ tool to find ranges of ABI
versions that have no change, this could help us greatly relax the version
constraints within nvproxy.

### Additional nvproxy struct tags

In the future, we can add additional information onto nvproxy structs using the
`nvproxy:"..."` tags. For example, we could tag any `NvP64` fields with the
struct type that the pointer represents, allowing us to recurse on these hidden
dependencies.
