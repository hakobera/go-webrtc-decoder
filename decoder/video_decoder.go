package decoder

import "image"

// DecodedImage defines interface for images which decoded by encoded video frame.
type DecodedImage interface {
	IsKeyFrame() bool
	Width() uint32
	Height() uint32
	Plane(n int) []byte
	Stride(n int) int
	ToBytes(format ColorFormat) []byte
	ToRGBA() *image.RGBA
}

// VideoDecodeResult contains decoded image for each frame or error when decode failed.
type VideoDecodeResult struct {
	Image DecodedImage
	Err   error
}

// VideoDecoder defines interfaces for video decoder.
type VideoDecoder interface {
	NewFrameBuilder() *FrameBuilder
	Process(src <-chan *Frame) chan VideoDecodeResult
	Close() error
}
