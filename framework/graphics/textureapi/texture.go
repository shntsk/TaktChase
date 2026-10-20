// Package textureapi defines the backend-neutral texture contract shared by
// the desktop OpenGL renderer and the Android GLES renderer.
package textureapi

type Filter int32

// Filtering contains the OpenGL-compatible enum values also defined by GLES.
// Keeping the values here avoids importing the desktop GL binding into the
// renderer's public texture contract.
var Filtering = struct {
	Nearest,
	Linear,
	MipMap,
	MipMapNearestNearest,
	MipMapLinearNearest,
	MipMapNearestLinear,
	MipMapLinearLinear Filter
}{
	0x2600, // GL_NEAREST
	0x2601, // GL_LINEAR
	0x2703, // GL_LINEAR_MIPMAP_LINEAR
	0x2700, // GL_NEAREST_MIPMAP_NEAREST
	0x2701, // GL_LINEAR_MIPMAP_NEAREST
	0x2702, // GL_NEAREST_MIPMAP_LINEAR
	0x2703, // GL_LINEAR_MIPMAP_LINEAR
}

type Texture interface {
	GetID() uint32
	GetWidth() int32
	GetHeight() int32
	GetRegion() TextureRegion
	GetLayers() int32
	SetFiltering(min, mag Filter)
	Bind(loc uint)
	GetLocation() uint
	Dispose()
}

type TextureRegion struct {
	Texture        Texture
	U1, U2, V1, V2 float32
	Width, Height  float32
	Layer          int32
}
