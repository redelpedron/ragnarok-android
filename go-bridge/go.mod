module github.com/kivutar/goro-android-port

go 1.23

require (
	github.com/kivutar/goro v0.0.0
	golang.org/x/mobile v0.0.0-20240806205939-81131f6468ab
)

replace github.com/kivutar/goro => ../goro

// Pin gpucontext to a version compatible with the gg canvas integration.
// The ggcanvas error happens when gg and gpucontext drift out of sync.
require (
	github.com/gogpu/gpucontext v0.0.0-20240801000000-000000000000
)
