//go:build android

package batch

/*
#cgo android LDFLAGS: -lGLESv3
#include <GLES3/gl3.h>
#include <stdint.h>
#include <stdlib.h>

static void taktVertexAttribPointer(GLuint index, GLint size, GLenum type,
                                    GLboolean normalized, GLsizei stride,
                                    uintptr_t offset) {
    glVertexAttribPointer(index, size, type, normalized, stride,
                          (const void *)offset);
}
*/
import "C"

import (
	"fmt"
	"strings"
	"unsafe"

	"github.com/go-gl/mathgl/mgl32"
	"github.com/shntsk/taktchase/framework/graphics/textureapi"
	"github.com/shntsk/taktchase/framework/math/vector"
)

const androidSpriteFloatsPerInstance = 13
const androidSpriteStride = androidSpriteFloatsPerInstance * 4

type glesBatchRenderer struct {
	program    uint32
	vao        uint32
	baseVBO    uint32
	instanceVBO uint32
	indexBuffer uint32
	projectionLocation int32
	textureLocation int32
	maxSprites int
	vertexSize int

	previousProgram C.GLint
	previousVAO C.GLint
	previousArrayBuffer C.GLint
	previousActiveTexture C.GLint
	previousArrayTexture C.GLint

	previousBlendEnabled bool
	previousBlendSrcRGB C.GLint
	previousBlendDstRGB C.GLint
	previousBlendSrcAlpha C.GLint
	previousBlendDstAlpha C.GLint
	previousBlendEquationRGB C.GLint
	previousBlendEquationAlpha C.GLint
}

var _ spriteBatchRenderer = (*glesBatchRenderer)(nil)

func newBatchRenderer(maxSprites int, persistent bool) (spriteBatchRenderer, batchTarget, int) {
	_ = persistent // GLES uploads each batch to an offset-zero streaming buffer.
	if maxSprites <= 0 {
		panic("QuadBatch size must be positive")
	}

	vertexSource := spriteVertexGLESSource
	fragmentSource := spriteFragmentGLESSource

	program, err := linkSpriteProgram(vertexSource, fragmentSource)
	if err != nil {
		panic(err)
	}

	r := &glesBatchRenderer{
		program: program,
		maxSprites: maxSprites,
		vertexSize: androidSpriteFloatsPerInstance,
		projectionLocation: glesUniformLocation(program, "proj"),
		textureLocation: glesUniformLocation(program, "tex"),
	}
	if r.projectionLocation < 0 || r.textureLocation < 0 {
		panic("TaktChase sprite shader is missing proj or tex uniform")
	}

	var vao, baseVBO, instanceVBO, indexBuffer C.GLuint
	C.glGenVertexArrays(1, &vao)
	C.glGenBuffers(1, &baseVBO)
	C.glGenBuffers(1, &instanceVBO)
	C.glGenBuffers(1, &indexBuffer)
	r.vao, r.baseVBO, r.instanceVBO, r.indexBuffer =
		uint32(vao), uint32(baseVBO), uint32(instanceVBO), uint32(indexBuffer)

	C.glBindVertexArray(vao)
	C.glBindBuffer(C.GL_ARRAY_BUFFER, baseVBO)
	vertices := [...]C.GLfloat{
		-1, -1, 0, 0,
		1, -1, 1, 0,
		1, 1, 1, 1,
		-1, 1, 0, 1,
	}
	C.glBufferData(C.GL_ARRAY_BUFFER, C.GLsizeiptr(len(vertices)*4),
		unsafe.Pointer(&vertices[0]), C.GL_STATIC_DRAW)

	basePos := glesAttributeLocation(program, "base_pos")
	baseUV := glesAttributeLocation(program, "base_uv")
	if basePos < 0 || baseUV < 0 {
		panic("TaktChase sprite shader is missing base_pos or base_uv")
	}
	C.glEnableVertexAttribArray(C.GLuint(basePos))
	C.taktVertexAttribPointer(C.GLuint(basePos), 2, C.GL_FLOAT, C.GL_FALSE, 16, 0)
	C.glEnableVertexAttribArray(C.GLuint(baseUV))
	C.taktVertexAttribPointer(C.GLuint(baseUV), 2, C.GL_FLOAT, C.GL_FALSE, 16, 8)

	C.glBindBuffer(C.GL_ARRAY_BUFFER, instanceVBO)
	C.glBufferData(C.GL_ARRAY_BUFFER,
		C.GLsizeiptr(maxSprites*androidSpriteStride), nil, C.GL_STREAM_DRAW)

	// The CPU instance record is exactly 13 floats (52 bytes). Packed origin
	// and color fields are read as normalized unsigned shorts/bytes, matching
	// the desktop attribute.Format definitions. GLES uses an offset-zero upload
	// on each flush, so instanced drawing does not need baseInstance support.
	type instanceAttribute struct {
		name string
		size int
		kind C.GLenum
		normalized C.GLboolean
		offset uintptr
	}
	attributes := []instanceAttribute{
		{"in_origin", 2, C.GL_UNSIGNED_SHORT, C.GL_TRUE, 0},
		{"in_scale", 2, C.GL_FLOAT, C.GL_FALSE, 4},
		{"in_position", 2, C.GL_FLOAT, C.GL_FALSE, 12},
		{"in_rotation", 1, C.GL_FLOAT, C.GL_FALSE, 20},
		{"in_u", 2, C.GL_FLOAT, C.GL_FALSE, 24},
		{"in_v", 2, C.GL_FLOAT, C.GL_FALSE, 32},
		{"in_layer", 1, C.GL_FLOAT, C.GL_FALSE, 40},
		{"in_color", 4, C.GL_UNSIGNED_BYTE, C.GL_TRUE, 44},
		{"in_additive", 1, C.GL_FLOAT, C.GL_FALSE, 48},
	}
	for _, attr := range attributes {
		location := glesAttributeLocation(program, attr.name)
		if location < 0 {
			panic(fmt.Sprintf("TaktChase sprite shader is missing %s", attr.name))
		}
		C.glEnableVertexAttribArray(C.GLuint(location))
		C.taktVertexAttribPointer(C.GLuint(location), C.GLint(attr.size), attr.kind,
			attr.normalized, C.GLsizei(androidSpriteStride), C.uintptr_t(attr.offset))
		C.glVertexAttribDivisor(C.GLuint(location), 1)
	}

	C.glBindBuffer(C.GL_ELEMENT_ARRAY_BUFFER, indexBuffer)
	indices := [...]C.GLushort{0, 1, 2, 2, 3, 0}
	C.glBufferData(C.GL_ELEMENT_ARRAY_BUFFER, C.GLsizeiptr(len(indices)*2),
		unsafe.Pointer(&indices[0]), C.GL_STATIC_DRAW)

	C.glBindVertexArray(0)
	C.glBindBuffer(C.GL_ARRAY_BUFFER, 0)
	if glErr := C.glGetError(); glErr != C.GL_NO_ERROR {
		panic(fmt.Sprintf("GLES sprite backend initialization failed: 0x%x", uint32(glErr)))
	}

	return r, batchTarget{data: make([]float32, maxSprites*androidSpriteFloatsPerInstance)}, androidSpriteFloatsPerInstance
}

func linkSpriteProgram(vertexSource, fragmentSource string) (uint32, error) {
	vs, err := compileGLESShader(C.GL_VERTEX_SHADER, vertexSource)
	if err != nil {
		return 0, err
	}
	defer C.glDeleteShader(C.GLuint(vs))

	fs, err := compileGLESShader(C.GL_FRAGMENT_SHADER, fragmentSource)
	if err != nil {
		return 0, err
	}
	defer C.glDeleteShader(C.GLuint(fs))

	program := C.glCreateProgram()
	C.glAttachShader(program, C.GLuint(vs))
	C.glAttachShader(program, C.GLuint(fs))
	C.glLinkProgram(program)
	var status C.GLint
	C.glGetProgramiv(program, C.GL_LINK_STATUS, &status)
	if status != C.GL_TRUE {
		log := make([]byte, 2048)
		var length C.GLsizei
		C.glGetProgramInfoLog(program, C.GLsizei(len(log)-1), &length,
			(*C.GLchar)(unsafe.Pointer(&log[0])))
		C.glDeleteProgram(program)
		return 0, fmt.Errorf("TaktChase sprite shader link failed: %s", strings.TrimRight(string(log[:length]), "\x00"))
	}
	return uint32(program), nil
}

func compileGLESShader(kind C.GLenum, source string) (uint32, error) {
	csource := C.CString(source)
	defer C.free(unsafe.Pointer(csource))

	shader := C.glCreateShader(kind)
	csources := (**C.GLchar)(unsafe.Pointer(&csource))
	C.glShaderSource(shader, 1, csources, nil)
	C.glCompileShader(shader)
	var status C.GLint
	C.glGetShaderiv(shader, C.GL_COMPILE_STATUS, &status)
	if status != C.GL_TRUE {
		log := make([]byte, 2048)
		var length C.GLsizei
		C.glGetShaderInfoLog(shader, C.GLsizei(len(log)-1), &length,
			(*C.GLchar)(unsafe.Pointer(&log[0])))
		C.glDeleteShader(shader)
		return 0, fmt.Errorf("TaktChase sprite shader compile failed: %s", strings.TrimRight(string(log[:length]), "\x00"))
	}
	return uint32(shader), nil
}

func glesAttributeLocation(program uint32, name string) int32 {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return int32(C.glGetAttribLocation(C.GLuint(program), (*C.GLchar)(unsafe.Pointer(cname))))
}

func glesUniformLocation(program uint32, name string) int32 {
	cname := C.CString(name)
	defer C.free(unsafe.Pointer(cname))
	return int32(C.glGetUniformLocation(C.GLuint(program), (*C.GLchar)(unsafe.Pointer(cname))))
}

func (r *glesBatchRenderer) begin(projection mgl32.Mat4) {
	C.glGetIntegerv(C.GL_CURRENT_PROGRAM, &r.previousProgram)
	C.glGetIntegerv(C.GL_VERTEX_ARRAY_BINDING, &r.previousVAO)
	C.glGetIntegerv(C.GL_ARRAY_BUFFER_BINDING, &r.previousArrayBuffer)
	C.glGetIntegerv(C.GL_ACTIVE_TEXTURE, &r.previousActiveTexture)
	C.glActiveTexture(C.GL_TEXTURE0)
	C.glGetIntegerv(C.GL_TEXTURE_BINDING_2D_ARRAY, &r.previousArrayTexture)

	r.previousBlendEnabled = C.glIsEnabled(C.GL_BLEND) == C.GL_TRUE
	C.glGetIntegerv(C.GL_BLEND_SRC_RGB, &r.previousBlendSrcRGB)
	C.glGetIntegerv(C.GL_BLEND_DST_RGB, &r.previousBlendDstRGB)
	C.glGetIntegerv(C.GL_BLEND_SRC_ALPHA, &r.previousBlendSrcAlpha)
	C.glGetIntegerv(C.GL_BLEND_DST_ALPHA, &r.previousBlendDstAlpha)
	C.glGetIntegerv(C.GL_BLEND_EQUATION_RGB, &r.previousBlendEquationRGB)
	C.glGetIntegerv(C.GL_BLEND_EQUATION_ALPHA, &r.previousBlendEquationAlpha)

	C.glUseProgram(C.GLuint(r.program))
	C.glBindVertexArray(C.GLuint(r.vao))
	C.glEnable(C.GL_BLEND)
	C.glBlendEquationSeparate(C.GL_FUNC_ADD, C.GL_FUNC_ADD)
	C.glBlendFuncSeparate(C.GL_ONE, C.GL_ONE_MINUS_SRC_ALPHA, C.GL_ONE, C.GL_ONE_MINUS_SRC_ALPHA)
	r.setProjection(projection)
}

func (r *glesBatchRenderer) setProjection(projection mgl32.Mat4) {
	C.glUniformMatrix4fv(C.GLint(r.projectionLocation), 1, C.GL_FALSE,
		(*C.GLfloat)(unsafe.Pointer(&projection[0])))
}

func (r *glesBatchRenderer) flush(tex textureapi.Texture, data []float32, usedFloats, spriteCount, baseOffset int) batchTarget {
	_ = baseOffset // GLES path uploads the current CPU slice at offset zero.
	if tex == nil || tex.GetID() == 0 {
		panic("TaktChase GLES batch received an uninitialized texture")
	}
	if usedFloats <= 0 || spriteCount <= 0 {
		return batchTarget{data: make([]float32, r.maxSprites*r.vertexSize)}
	}

	C.glActiveTexture(C.GL_TEXTURE0)
	C.glBindTexture(C.GL_TEXTURE_2D_ARRAY, C.GLuint(tex.GetID()))
	C.glUniform1i(C.GLint(r.textureLocation), 0)
	C.glBindBuffer(C.GL_ARRAY_BUFFER, C.GLuint(r.instanceVBO))
	if usedFloats > len(data) {
		panic("TaktChase GLES batch data length does not cover the requested upload")
	}
	if usedFloats > 0 {
		// data is owned by Go for this synchronous upload; GL copies it before
		// returning and does not retain the Go pointer.
		C.glBufferData(C.GL_ARRAY_BUFFER, C.GLsizeiptr(usedFloats*4),
			unsafe.Pointer(&data[0]), C.GL_STREAM_DRAW)
	}
	C.glDrawElementsInstanced(C.GL_TRIANGLES, 6, C.GL_UNSIGNED_SHORT, nil, C.GLsizei(spriteCount))
	return batchTarget{data: make([]float32, r.maxSprites*r.vertexSize), offset: 0}
}

func (r *glesBatchRenderer) end() {
	C.glBindTexture(C.GL_TEXTURE_2D_ARRAY, C.GLuint(r.previousArrayTexture))
	C.glActiveTexture(C.GLenum(r.previousActiveTexture))
	C.glBindBuffer(C.GL_ARRAY_BUFFER, C.GLuint(r.previousArrayBuffer))
	C.glBindVertexArray(C.GLuint(r.previousVAO))
	C.glUseProgram(C.GLuint(r.previousProgram))

	C.glBlendEquationSeparate(C.GLenum(r.previousBlendEquationRGB), C.GLenum(r.previousBlendEquationAlpha))
	C.glBlendFuncSeparate(C.GLenum(r.previousBlendSrcRGB), C.GLenum(r.previousBlendDstRGB),
		C.GLenum(r.previousBlendSrcAlpha), C.GLenum(r.previousBlendDstAlpha))
	if r.previousBlendEnabled {
		C.glEnable(C.GL_BLEND)
	} else {
		C.glDisable(C.GL_BLEND)
	}
}

func (r *glesBatchRenderer) dispose() {
	var program C.GLuint = C.GLuint(r.program)
	var vao C.GLuint = C.GLuint(r.vao)
	var buffers [3]C.GLuint{C.GLuint(r.baseVBO), C.GLuint(r.instanceVBO), C.GLuint(r.indexBuffer)}
	C.glDeleteProgram(program)
	C.glDeleteVertexArrays(1, &vao)
	C.glDeleteBuffers(C.GLsizei(len(buffers)), &buffers[0])
	r.program, r.vao, r.baseVBO, r.instanceVBO, r.indexBuffer = 0, 0, 0, 0, 0
}

type AndroidBatchFixture struct {
	batch *QuadBatch
	texture *glesFixtureTexture
}

// NewAndroidBatchFixture creates a deterministic texture and renders it through
// TaktChase's real QuadBatch and GLES backend. It is a renderer integration
// fixture, not a replacement gameplay renderer.
func NewAndroidBatchFixture() *AndroidBatchFixture {
	b := NewQuadBatchSize(8)
	t := newGLESFixtureTexture()
	return &AndroidBatchFixture{batch: b, texture: t}
}

func (f *AndroidBatchFixture) Draw() bool {
	if f == nil || f.batch == nil || f.texture == nil || f.texture.GetID() == 0 {
		return false
	}
	f.batch.SetTranslation(vector.NewVec2d(0.60, 0.55))
	f.batch.SetScale(0.02, 0.02)
	f.batch.SetColor32(1, 1, 1, 1)
	f.batch.Begin()
	f.batch.DrawTexture(f.texture.GetRegion())
	f.batch.End()
	return C.glGetError() == C.GL_NO_ERROR
}

func (f *AndroidBatchFixture) Dispose() {
	if f == nil {
		return
	}
	if f.batch != nil {
		f.batch.Dispose()
		f.batch = nil
	}
	if f.texture != nil {
		f.texture.Dispose()
		f.texture = nil
	}
}

type glesFixtureTexture struct {
	id uint32
	width int32
	height int32
	location uint
}

var _ textureapi.Texture = (*glesFixtureTexture)(nil)

func newGLESFixtureTexture() *glesFixtureTexture {
	const width, height = 16, 16
	pixels := make([]byte, width*height*4)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dx, dy := float64(x)-7.5, float64(y)-7.5
			if dx*dx+dy*dy > 49 {
				continue
			}
			i := (y*width + x) * 4
			pixels[i+0] = 42
			pixels[i+1] = 210
			pixels[i+2] = 255
			pixels[i+3] = 255
		}
	}

	var previousActive, previousTexture C.GLint
	C.glGetIntegerv(C.GL_ACTIVE_TEXTURE, &previousActive)
	C.glActiveTexture(C.GL_TEXTURE0)
	C.glGetIntegerv(C.GL_TEXTURE_BINDING_2D_ARRAY, &previousTexture)

	var id C.GLuint
	C.glGenTextures(1, &id)
	if id == 0 {
		C.glActiveTexture(C.GLenum(previousActive))
		panic("GLES could not allocate the QuadBatch fixture texture")
	}
	C.glBindTexture(C.GL_TEXTURE_2D_ARRAY, id)
	C.glTexParameteri(C.GL_TEXTURE_2D_ARRAY, C.GL_TEXTURE_MIN_FILTER, C.GL_LINEAR)
	C.glTexParameteri(C.GL_TEXTURE_2D_ARRAY, C.GL_TEXTURE_MAG_FILTER, C.GL_LINEAR)
	C.glTexParameteri(C.GL_TEXTURE_2D_ARRAY, C.GL_TEXTURE_WRAP_S, C.GL_CLAMP_TO_EDGE)
	C.glTexParameteri(C.GL_TEXTURE_2D_ARRAY, C.GL_TEXTURE_WRAP_T, C.GL_CLAMP_TO_EDGE)
	C.glTexImage3D(C.GL_TEXTURE_2D_ARRAY, 0, C.GLint(C.GL_RGBA8),
		width, height, 1, 0, C.GL_RGBA, C.GL_UNSIGNED_BYTE,
		unsafe.Pointer(&pixels[0]))
	C.glBindTexture(C.GL_TEXTURE_2D_ARRAY, C.GLuint(previousTexture))
	C.glActiveTexture(C.GLenum(previousActive))
	if glErr := C.glGetError(); glErr != C.GL_NO_ERROR {
		C.glDeleteTextures(1, &id)
		panic(fmt.Sprintf("GLES fixture texture creation failed: 0x%x", uint32(glErr)))
	}
	return &glesFixtureTexture{id: uint32(id), width: width, height: height}
}

func (t *glesFixtureTexture) GetID() uint32 { return t.id }
func (t *glesFixtureTexture) GetWidth() int32 { return t.width }
func (t *glesFixtureTexture) GetHeight() int32 { return t.height }
func (t *glesFixtureTexture) GetLayers() int32 { return 1 }
func (t *glesFixtureTexture) GetLocation() uint { return t.location }

func (t *glesFixtureTexture) GetRegion() textureapi.TextureRegion {
	return textureapi.TextureRegion{
		Texture: t, U1: 0, U2: 1, V1: 0, V2: 1,
		Width: float32(t.width), Height: float32(t.height), Layer: 0,
	}
}

func (t *glesFixtureTexture) Bind(loc uint) {
	C.glActiveTexture(C.GLenum(C.GL_TEXTURE0) + C.GLenum(loc))
	C.glBindTexture(C.GL_TEXTURE_2D_ARRAY, C.GLuint(t.id))
	t.location = loc
}

func (t *glesFixtureTexture) SetFiltering(min, mag textureapi.Filter) {
	t.Bind(t.location)
	C.glTexParameteri(C.GL_TEXTURE_2D_ARRAY, C.GL_TEXTURE_MIN_FILTER, C.GLint(min))
	C.glTexParameteri(C.GL_TEXTURE_2D_ARRAY, C.GL_TEXTURE_MAG_FILTER, C.GLint(mag))
}

func (t *glesFixtureTexture) Dispose() {
	if t.id == 0 {
		return
	}
	id := C.GLuint(t.id)
	C.glDeleteTextures(1, &id)
	t.id = 0
}
