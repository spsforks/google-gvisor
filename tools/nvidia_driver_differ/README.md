# NVIDIA Driver Differ

This tool is intended to help us adopt new NVIDIA driver versions. It compares
two driver versions--one currently supported by `nvproxy` and one that is
not--and reports any changes to `ioctl` structs that exists between the two
versions. However, it only does this for `ioctl` structs that are currently
supported by `nvproxy`, giving us a targeted but comprehensive understanding of
what changes need to be reflected in nvproxy.

This requires us to parse the NVIDIA driver source code. We do this using
[Clang's AST Matcher API](https://clang.llvm.org/docs/LibASTMatchersReference.html).
This allows us to generate an AST of the NVIDIA driver, which we can easily
search and traverse to get a comprehensive definition of every struct we want.

## Usage

Everything is packaged for convenience inside `run_differ`. We start by building
this:

```bash
make copy TARGETS=//tools/nvidia_driver_differ:run_differ DESTINATION=bin/
```

Once built, we just need to pass the two versions we are interested in:

```bash
bin/run_differ --base 550.90.07 --next 560.31.02
```

This will fetch the corresponding source code from Github, parse it using Clang,
and then compare the definitions that were found. Any differences will be
printed to standard output.

## How it works

This tool relies on the list of supported `ioctl` structs that `nvproxy` reports
via `nvproxy.SupportedStructNames(version)`. We look up the definition of these
structs using Clang's Matcher API, which we can then compare between versions.

### C++ driver parser

The Matcher API is used by an intermediate tool, `driver_ast_parser.cc`. This
takes takes in a list of struct names to parse and outputs a comprehensive
definition for each one. The information is all recorded in JSON; the specific
format is written out in `parser/json_definitions.go`.

To find the definition for a given struct, we can note that every struct in
NVIDIA's driver is defined via a `typedef`, like so:

```c++
typedef struct NV00FD_CTRL_GET_INFO_PARAMS {
   NvU64 alignment;
   NvU64 allocSize;
   NvU32 pageSize;
   NvU32 numMaxGpus;
   NvU32 numAttachedGpus;
} NV00FD_CTRL_GET_INFO_PARAMS;
```

As such, we use a matcher expression that looks for `typedef`s with a given name
to get the attached struct definition. This will return a `clang::RecordDecl`
node, which contains information about the struct. Specifically, we can iterate
through its `clang::FieldDecl`s and get each field's name, type, and offset.
Other information like the size of the struct or its position in the source code
is also available through the `clang::RecordDecl` node.

While iterating through the fields of a record, we also want to recurse through
the types of each field. This way, we can capture changes that are hidden in
nested structs. We also record any aliases that we find, such as noting that
`NvU32` is an `unsigned int`. This is done by getting the true base type of any
type using `clang::QualType.getCanonicalType()`.

Finally, there are also a few structs in the driver that are `typedef`s of other
structs. For example:

```c++
typedef NV906F_CTRL_GET_CLASS_ENGINEID_PARAMS NVC36F_CTRL_GET_CLASS_ENGINEID_PARAMS;
```

As such, we need another matcher expression to capture these. This expression
does not bind to a `clang::RecordDecl` node; thus, when we get a match that does
not have such a binding, we know we have hit this case. From there, we simply
copy the definition of the `typedef`'d struct.

### Packaging everything together

Now that we have a tool to parse NVIDIA's driver source code, we can package it
into a pipeline for diffing two versions of the driver. Specifically, the
pipeline that `run_differ.go` implements can be broken down like so:

1.  Get the list of struct names that `nvproxy` depends on for the `base`
    version.
2.  For both version, `base` and `next`:
    1.  Make a temporary directory for all the files to be placed in.
    2.  Clone the source code from GitHub.
    3.  Create the source file that Clang will be parsing.
        -   This is merely one giant file with a bunch of `#include` directives
            pointing to files that may have definitions we care about.
        -   The specific files we `#include` are listed in `parser/sources.go`.
        -   The driver actually has two file roots: `src/` and `kernel-open/`.
            This means we create two source files, one for each root.
    4.  Create `compile_commands.json`.
        -   This is a special file for Clang, which gives it the path for all
            include directories to use.
        -   The format is documented
            [here](https://clang.llvm.org/docs/JSONCompilationDatabase.html).
    5.  Now we can run the `driver_ast_parser` binary on **both** source files
        we created and merge their outputs together.
3.  Diff the outputs gathered from each version, and report any differences
    found.
