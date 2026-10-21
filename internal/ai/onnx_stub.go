//go:build !onnx

package ai

import "errors"

// ErrONNXUnavailable is returned when the binary was built without the
// "onnx" build tag (the default, so plain `go build` needs no cgo).
var ErrONNXUnavailable = errors.New("ai: built without the onnx tag; rebuild with -tags onnx (needs cgo + libonnxruntime)")

func newONNX(Options, Need) (onnxModels, error) { return nil, ErrONNXUnavailable }
