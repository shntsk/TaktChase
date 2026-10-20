//go:build !android

package batch

import (
	"github.com/go-gl/mathgl/mgl32"
	"github.com/shntsk/taktchase/framework/assets"
	"github.com/shntsk/taktchase/framework/graphics/attribute"
	"github.com/shntsk/taktchase/framework/graphics/blend"
	"github.com/shntsk/taktchase/framework/graphics/buffer"
	"github.com/shntsk/taktchase/framework/graphics/shader"
	"github.com/shntsk/taktchase/framework/graphics/texture"
)

type desktopBatchRenderer struct {
	shader     *shader.RShader
	vao        *buffer.VertexArrayObject
	vertexSize int
	maxSprites int
}

var _ spriteBatchRenderer = (*desktopBatchRenderer)(nil)

func newBatchRenderer(maxSprites int, persistent bool) (spriteBatchRenderer, batchTarget, int) {
	vert, err := assets.GetString("assets/shaders/sprite.vsh")
	if err != nil {
		panic(err)
	}
	frag, err := assets.GetString("assets/shaders/sprite.fsh")
	if err != nil {
		panic(err)
	}

	rShader := shader.NewRShader(shader.NewSource(vert, shader.Vertex), shader.NewSource(frag, shader.Fragment))
	vao := buffer.NewVertexArrayObject()
	vao.AddVBO("default", 4, 0, attribute.Format{
		{Name: "base_pos", Type: attribute.Vec2},
		{Name: "base_uv", Type: attribute.Vec2},
	})

	format := attribute.Format{
		{Name: "in_origin", Type: attribute.Vec2Packed},
		{Name: "in_scale", Type: attribute.Vec2},
		{Name: "in_position", Type: attribute.Vec2},
		{Name: "in_rotation", Type: attribute.Float},
		{Name: "in_u", Type: attribute.Vec2},
		{Name: "in_v", Type: attribute.Vec2},
		{Name: "in_layer", Type: attribute.Float},
		{Name: "in_color", Type: attribute.ColorPacked},
		{Name: "in_additive", Type: attribute.Float},
	}

	if persistent {
		vao.AddPersistentVBO("quads", maxSprites*100, 1, format)
	} else {
		vao.AddMappedVBO("quads", maxSprites, 1, format)
	}
	vao.SetData("default", 0, []float32{
		-1, -1, 0, 0,
		1, -1, 1, 0,
		1, 1, 1, 1,
		-1, 1, 0, 1,
	})
	vao.Attach(rShader)

	ibo := buffer.NewIndexBufferObject(6)
	ibo.SetData(0, []uint16{0, 1, 2, 2, 3, 0})
	vao.AttachIBO(ibo)

	vertexSize := vao.GetVBOFormat("quads").Size() / 4
	chunk := vao.MapVBO("quads", maxSprites*vertexSize)
	renderer := &desktopBatchRenderer{
		shader: rShader, vao: vao, vertexSize: vertexSize, maxSprites: maxSprites,
	}
	return renderer, batchTarget{data: chunk.Data, offset: chunk.Offset}, vertexSize
}

func (r *desktopBatchRenderer) begin(projection mgl32.Mat4) {
	r.shader.Bind()
	r.shader.SetUniform("proj", projection)
	r.vao.Bind()
	blend.Push()
	blend.Enable()
	blend.SetFunction(blend.One, blend.OneMinusSrcAlpha)
}

func (r *desktopBatchRenderer) setProjection(projection mgl32.Mat4) {
	r.shader.SetUniform("proj", projection)
}

func (r *desktopBatchRenderer) flush(tex texture.Texture, data []float32, usedFloats, spriteCount, baseOffset int) batchTarget {
	_ = data // desktop writes directly through its mapped buffer chunk.
	if tex.GetLocation() == 0 {
		tex.Bind(0)
	}
	r.shader.SetUniform("tex", int32(tex.GetLocation()))
	r.vao.UnmapVBO("quads", 0, usedFloats)
	r.vao.DrawInstanced(baseOffset/r.vertexSize, spriteCount)
	next := r.vao.MapVBO("quads", r.maxSprites*r.vertexSize)
	return batchTarget{data: next.Data, offset: next.Offset}
}

func (r *desktopBatchRenderer) end() {
	r.vao.Unbind()
	r.shader.Unbind()
	blend.Pop()
}
