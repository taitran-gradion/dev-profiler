module github.com/margince/margince/extensions/profiler

go 1.27.2

// The backend this unit compiles against is the one in this tree, as every
// unit under extensions/ does.
replace github.com/margince/margince/backend => ../../backend

require github.com/margince/margince/backend v0.0.0-00010101000000-000000000000
