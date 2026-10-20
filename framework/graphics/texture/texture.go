package texture

import (
	"github.com/go-gl/gl/v3.3-core/gl"

	"github.com/shntsk/taktchase/framework/graphics/hacks"
	"github.com/shntsk/taktchase/framework/graphics/textureapi"
	color2 "github.com/shntsk/taktchase/framework/math/color"
)

// Re-export the backend-neutral API so existing desktop callers retain the
// same package names and source compatibility.
type Filter = textureapi.Filter
var Filtering = textureapi.Filtering
type Texture = textureapi.Texture
type TextureRegion = textureapi.TextureRegion

type textureStore struct {
	id                             uint32
	binding                        uint
	layers, width, height, mipmaps int32
	format                         Format
	min, mag                       Filter
	disposed                       bool
}

func newStore(layerNum, width, height int, format Format, mipmaps int) *textureStore {
	store := new(textureStore)

	gl.CreateTextures(gl.TEXTURE_2D_ARRAY, 1, &store.id)

	store.layers = int32(layerNum)
	store.width = int32(width)
	store.height = int32(height)
	store.format = format

	if mipmaps < 1 {
		mipmaps = 1
	}
	store.mipmaps = int32(mipmaps)

	gl.TextureStorage3D(store.id, store.mipmaps, format.InternalFormat(), store.width, store.height, store.layers)
	gl.TextureParameteri(store.id, gl.TEXTURE_BASE_LEVEL, 0)
	gl.TextureParameteri(store.id, gl.TEXTURE_MAX_LEVEL, store.mipmaps-1)
	gl.TextureParameteri(store.id, gl.TEXTURE_WRAP_S, gl.CLAMP_TO_EDGE)
	gl.TextureParameteri(store.id, gl.TEXTURE_WRAP_T, gl.CLAMP_TO_EDGE)

	if mipmaps > 1 {
		store.SetFiltering(Filtering.MipMap, Filtering.Linear)
	} else {
		store.SetFiltering(Filtering.Linear, Filtering.Linear)
	}

	return store
}

func (store *textureStore) SetData(x, y, width, height, layer int, data []uint8, generateMipmaps bool) {
	if len(data) != width*height*store.format.Size() {
		panic("Wrong number of pixels given!")
	}

	amdHack := store.binding == 0 && hacks.IsOldAMD

	if amdHack {
		gl.BindTextureUnit(11, store.id)
	}

	gl.TextureSubImage3D(store.id, 0, int32(x), int32(y), int32(layer), int32(width), int32(height), 1, store.format.Format(), store.format.Type(), gl.Ptr(data))

	if amdHack {
		gl.BindTextureUnit(uint32(store.binding), store.id)
	}

	if store.mipmaps > 1 && generateMipmaps {
		gl.GenerateTextureMipmap(store.id)
	}
}

func (store *textureStore) SetDataBuf(x, y, width, height, layer int, rowWidth int, ptr uintptr, generateMipmaps bool) {
	amdHack := store.binding == 0 && hacks.IsOldAMD

	if amdHack {
		gl.BindTextureUnit(11, store.id)
	}

	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, int32(rowWidth))

	gl.TextureSubImage3D(store.id, 0, int32(x), int32(y), int32(layer), int32(width), int32(height), 1, store.format.Format(), store.format.Type(), gl.Ptr(ptr))

	gl.PixelStorei(gl.UNPACK_ROW_LENGTH, 0)

	if amdHack {
		gl.BindTextureUnit(uint32(store.binding), store.id)
	}

	if store.mipmaps > 1 && generateMipmaps {
		gl.GenerateTextureMipmap(store.id)
	}
}

func (store *textureStore) Bind(loc uint) {
	store.binding = loc

	gl.BindTextureUnit(uint32(loc), store.id)
}

func (store *textureStore) Clear() {
	gl.ClearTexImage(store.id, 0, store.format.Format(), store.format.Type(), gl.Ptr(nil))
}

func (store *textureStore) ClearColor(clearColor color2.Color) {
	clr := clearColor.ToIntArray()

	gl.ClearTexImage(store.id, 0, store.format.Format(), store.format.Type(), gl.Ptr(&clr[0]))
}

func (store *textureStore) SetFiltering(min, mag Filter) {
	store.min = min
	store.mag = mag

	gl.TextureParameteri(store.id, gl.TEXTURE_MIN_FILTER, int32(min))
	gl.TextureParameteri(store.id, gl.TEXTURE_MAG_FILTER, int32(mag))
}

func (store *textureStore) Dispose() {
	if !store.disposed {
		gl.DeleteTextures(1, &store.id)
	}

	store.disposed = true
}
