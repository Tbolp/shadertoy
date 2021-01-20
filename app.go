package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg"
	_ "image/png"
	"io/ioutil"
	"os"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/go-gl/gl/v4.5-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

type TextureMgr struct {
	textures map[string]uint32
}

func (this *TextureMgr) LoadTexture(name string) (tex uint32, err error) {
	if v, ok := this.textures[name]; ok {
		tex = v
		return
	} else {
		if name == "buffer_a" ||
			name == "buffer_b" ||
			name == "buffer_c" ||
			name == "buffer_d" {
			gl.CreateTextures(gl.TEXTURE_2D, 1, &tex)
			this.textures[name] = tex
			return
		} else {
			tex, err = newTextureEx(name)
			if err == nil {
				this.textures[name] = tex
			}
			return
		}
	}
}

var textureMgr *TextureMgr
var textureMgrOnce sync.Once

func DefaultTextureMgr() *TextureMgr {
	textureMgrOnce.Do(func() {
		textureMgr = &TextureMgr{}
		textureMgr.textures = map[string]uint32{}
	})
	return textureMgr
}

type TextureConf struct {
	Path   string
	Filter int
	Wrap   int
}

type BufferConf struct {
	Path     string
	Textures []TextureConf
	program  uint32
	fbo      uint32
	name     string
}

func (this *BufferConf) init(commonStr string) (err error) {
	for _, conf := range this.Textures {
		_, err = DefaultTextureMgr().LoadTexture(conf.Path)
		if err != nil {
			return
		}
	}
	content, err := ioutil.ReadFile(this.Path)
	if err == nil {
		vertexShader := `
#version 450 core
layout(location = 0) in vec2 pos;

void main()
{
	gl_Position = vec4(pos, 1.0, 1.0);
}
` + "\x00"
		fragmentShader := `
#version 450 core
uniform vec3      iResolution;           // viewport resolution (in pixels)
uniform float     iTime;                 // shader playback time (in seconds)
//uniform float     iTimeDelta;            // render time (in seconds)
uniform int       iFrame;                // shader playback frame
//uniform float     iChannelTime[4];       // channel playback time (in seconds)
//uniform vec3      iChannelResolution[4]; // channel resolution (in pixels)
uniform vec4      iMouse;                // mouse pixel coords. xy: current (if MLB down), zw: click
layout (binding = 0) uniform sampler2D iChannel0;          // input channel. XX = 2D/Cube
layout (binding = 1) uniform sampler2D iChannel1;
layout (binding = 2) uniform sampler2D iChannel2;
layout (binding = 3) uniform sampler2D iChannel3;
//uniform vec4      iDate;                 // (year, month, day, time in seconds)
out vec4 fragColor;
` + commonStr + "\n" + string(content) + `
void main(){
	mainImage(fragColor, gl_FragCoord.xy);
}
` + "\x00"
		var program uint32
		program, err = newProgram(vertexShader, fragmentShader)
		if err == nil {
			this.program = program
		}
		return
	}
	if err != nil {
		return
	}
	return
}

func (this *BufferConf) clear() {
	gl.DeleteProgram(this.program)
}

type CommonConf struct {
	Path string
}

type PipelineConf struct {
	Buffer_A *BufferConf
	Buffer_B *BufferConf
	Buffer_C *BufferConf
	Buffer_D *BufferConf
	Image    *BufferConf
	Common   *CommonConf
}

func (this *PipelineConf) init() (err error) {
	commonStr := ""
	if this.Common != nil {
		var content []byte
		content, err = ioutil.ReadFile(this.Common.Path)
		if err != nil {
			return
		}
		commonStr = string(content)
	}
	if this.Image == nil {
		return
	} else {
		err = this.Image.init(commonStr)
		if err != nil {
			return
		}
	}
	initBuffer := func(conf *BufferConf, name string) (err error) {
		if conf == nil {
			return
		} else {
			err = conf.init(commonStr)
			if err != nil {
				return
			}
			conf.name = name
			gl.CreateFramebuffers(1, &conf.fbo)
			var tex uint32
			tex, err = DefaultTextureMgr().LoadTexture(name)
			if err != nil {
				return
			}
			gl.NamedFramebufferTexture(conf.fbo, gl.COLOR_ATTACHMENT0, tex, 0)
		}
		return
	}
	err = initBuffer(this.Buffer_A, "buffer_a")
	if err != nil {
		return
	}
	err = initBuffer(this.Buffer_B, "buffer_b")
	if err != nil {
		return
	}
	err = initBuffer(this.Buffer_C, "buffer_c")
	if err != nil {
		return
	}
	err = initBuffer(this.Buffer_D, "buffer_d")
	if err != nil {
		return
	}
	return
}

type pipelineUniform struct {
	Width  float32
	Height float32
	Time   float32
}

type appConf struct {
	Width    int
	Height   int
	Pipeline PipelineConf
}

type ShaderToy struct {
	conf      appConf
	window    *glfw.Window
	vao       uint32
	startTime time.Time
	loadMutex sync.Mutex
}

func NewShaderToy() *ShaderToy {
	instance := &ShaderToy{}
	instance.conf.Width = 400
	instance.conf.Height = 400
	return instance
}

func (this *ShaderToy) Exec() {
	runtime.LockOSThread()
	var err error
	err = glfw.Init()
	if err != nil {
		panic(err)
	}
	this.window, err = glfw.CreateWindow(this.conf.Width, this.conf.Height, "ShaderToy", nil, nil)
	if err != nil {
		panic(err)
	}
	this.window.MakeContextCurrent()

	err = this.initGL()
	if err != nil {
		panic(err)
	}

	err = this.initConf()
	if err != nil {
		panic(err)
	}

	this.window.SetSizeCallback(func(w *glfw.Window, width int, height int) {
		gl.Viewport(0, 0, int32(width), int32(height))
	})

	this.window.SetMouseButtonCallback(func(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
		if button == glfw.MouseButtonRight {
			//TODO
		}
	})

	this.startTime = time.Now()
	for !this.window.ShouldClose() {
		this.renderGL()
		this.window.SwapBuffers()
		glfw.PollEvents()
	}
}

func (this *ShaderToy) initGL() (err error) {
	err = gl.Init()
	if err != nil {
		return
	}

	version := gl.GoStr(gl.GetString(gl.VERSION))
	fmt.Println("OpenGL version", version)

	points := []float32{
		1.0, 1.0, 1.0, -1.0, -1.0, -1.0, -1.0, 1.0,
	}
	index := []uint32{
		0, 2, 1, 0, 3, 2,
	}

	gl.CreateVertexArrays(1, &this.vao)

	vbo := [2]uint32{}
	gl.CreateBuffers(2, &vbo[0])
	gl.NamedBufferData(vbo[0], len(points)*4, gl.Ptr(points), gl.STATIC_DRAW)
	gl.NamedBufferData(vbo[1], len(index)*4, gl.Ptr(index), gl.STATIC_DRAW)

	gl.VertexArrayVertexBuffer(this.vao, 0, vbo[0], 0, 2*4)
	gl.VertexArrayElementBuffer(this.vao, vbo[1])

	gl.EnableVertexArrayAttrib(this.vao, 0)
	gl.VertexArrayAttribFormat(this.vao, 0, 2, gl.FLOAT, false, 0)

	gl.ClearColor(0, 0, 0, 1)

	return
}

func (this *ShaderToy) initConf() (err error) {
	path := "config.json"
	if len(os.Args) == 2 {
		path = os.Args[1]
	}
	content, err := ioutil.ReadFile(path)
	if err == nil {
		err = json.Unmarshal(content, &this.conf)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("Use Config")
		}
	} else {
		fmt.Println(err)
	}
	err = this.conf.Pipeline.init()
	if err != nil {
		return
	}
	this.window.SetSize(this.conf.Width, this.conf.Height)
	return
}

func (this *ShaderToy) renderGL() {
	this.render(this.conf.Pipeline.Buffer_A)
	this.render(this.conf.Pipeline.Buffer_B)
	this.render(this.conf.Pipeline.Buffer_C)
	this.render(this.conf.Pipeline.Buffer_D)
	this.render(this.conf.Pipeline.Image)
}

func (this *ShaderToy) render(conf *BufferConf) {
	if conf == nil {
		return
	}
	w, h := this.window.GetSize()
	gl.BindFramebuffer(gl.FRAMEBUFFER, conf.fbo)
	if conf.fbo != 0 {
		tex, err := DefaultTextureMgr().LoadTexture(conf.name)
		if err != nil {
			return
		}
		gl.TextureImage2DEXT(tex, gl.TEXTURE_2D, 0, gl.RGBA8, int32(w), int32(h), 0, gl.RGBA, gl.UNSIGNED_BYTE, gl.Ptr(nil))
	}
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.UseProgram(conf.program)
	this.setUniform(conf.program)
	for i, conf := range conf.Textures {
		tex, err := DefaultTextureMgr().LoadTexture(conf.Path)
		if err != nil {
			return
		}
		gl.TextureParameteri(tex, gl.TEXTURE_MIN_FILTER, convertToFilter(conf.Filter))
		gl.TextureParameteri(tex, gl.TEXTURE_MAG_FILTER, convertToFilter(conf.Filter))
		gl.TextureParameteri(tex, gl.TEXTURE_WRAP_S, convertToWrap(conf.Wrap))
		gl.TextureParameteri(tex, gl.TEXTURE_WRAP_T, convertToWrap(conf.Wrap))
		gl.BindTextureUnit(uint32(i), tex)
	}
	gl.BindVertexArray(this.vao)
	gl.DrawElements(gl.TRIANGLES, 6, gl.UNSIGNED_INT, gl.Ptr(nil))
}

func (this *ShaderToy) setUniform(program uint32) {
	// iResolution
	loc := gl.GetUniformLocation(program, gl.Str("iResolution\x00"))
	w, h := this.window.GetSize()
	gl.Uniform3f(loc, float32(w), float32(h), 0)
	// iTime
	loc = gl.GetUniformLocation(program, gl.Str("iTime\x00"))
	t := float32(time.Since(this.startTime).Milliseconds()) / 1000.0
	gl.Uniform1f(loc, t)
	// iMouse
	x, y := this.window.GetCursorPos()
	loc = gl.GetUniformLocation(program, gl.Str("iMouse\x00"))
	if this.window.GetMouseButton(glfw.MouseButtonLeft) == glfw.Press {
		gl.Uniform4f(loc, float32(x), float32(y), float32(x), float32(y))
	} else {
		if x > 0 && y > 0 && x < float64(w) && y < float64(h) {
			gl.Uniform4f(loc, float32(x), float32(y), 0, 0)
		}
	}
}

func newProgram(vertexShaderSource, fragmentShaderSource string) (uint32, error) {
	vertexShader, err := compileShader(vertexShaderSource, gl.VERTEX_SHADER)
	if err != nil {
		return 0, err
	}

	fragmentShader, err := compileShader(fragmentShaderSource, gl.FRAGMENT_SHADER)
	if err != nil {
		return 0, err
	}

	program := gl.CreateProgram()

	gl.AttachShader(program, vertexShader)
	gl.AttachShader(program, fragmentShader)
	gl.LinkProgram(program)

	var status int32
	gl.GetProgramiv(program, gl.LINK_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetProgramiv(program, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetProgramInfoLog(program, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to link program: %v", log)
	}

	gl.DeleteShader(vertexShader)
	gl.DeleteShader(fragmentShader)

	return program, nil
}

func compileShader(source string, shaderType uint32) (uint32, error) {
	shader := gl.CreateShader(shaderType)

	csources, free := gl.Strs(source)
	gl.ShaderSource(shader, 1, csources, nil)
	free()
	gl.CompileShader(shader)

	var status int32
	gl.GetShaderiv(shader, gl.COMPILE_STATUS, &status)
	if status == gl.FALSE {
		var logLength int32
		gl.GetShaderiv(shader, gl.INFO_LOG_LENGTH, &logLength)

		log := strings.Repeat("\x00", int(logLength+1))
		gl.GetShaderInfoLog(shader, logLength, nil, gl.Str(log))

		return 0, fmt.Errorf("failed to compile %v: %v", source, log)
	}

	return shader, nil
}

func newTextureEx(file string) (uint32, error) {
	imgFile, err := os.Open(file)
	if err != nil {
		return 0, fmt.Errorf("texture %q not found on disk: %v", file, err)
	}
	img, _, err := image.Decode(imgFile)
	if err != nil {
		return 0, err
	}

	rgba := image.NewRGBA(img.Bounds())
	if rgba.Stride != rgba.Rect.Size().X*4 {
		return 0, fmt.Errorf("unsupported stride")
	}
	draw.Draw(rgba, rgba.Bounds(), img, image.Point{0, 0}, draw.Src)

	var texture uint32
	gl.GenTextures(1, &texture)
	gl.ActiveTexture(gl.TEXTURE0)
	gl.BindTexture(gl.TEXTURE_2D, texture)
	gl.TexImage2D(
		gl.TEXTURE_2D,
		0,
		gl.RGBA,
		int32(rgba.Rect.Size().X),
		int32(rgba.Rect.Size().Y),
		0,
		gl.RGBA,
		gl.UNSIGNED_BYTE,
		gl.Ptr(rgba.Pix))

	return texture, nil
}

func convertToFilter(val int) (param int32) {
	param = gl.LINEAR
	switch val {
	case 1:
		param = gl.NEAREST
	case 2:
		param = gl.LINEAR
	case 3:
		param = gl.MIPMAP
	}
	return
}
func convertToWrap(val int) (param int32) {
	param = gl.REPEAT
	switch val {
	case 1:
		param = gl.CLAMP_TO_EDGE
	case 2:
		param = gl.REPEAT
	}
	return
}
