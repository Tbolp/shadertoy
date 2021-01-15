package main

import (
	"encoding/json"
	"fmt"
	"image"
	"image/draw"
	_ "image/png"
	"io/ioutil"
	"os"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-gl/gl/v4.5-core/gl"
	"github.com/go-gl/glfw/v3.3/glfw"
)

type channel struct {
	Path   string
	Type   int
	Filter int
	Wrap   int
}

type windowConf struct {
	Width        int
	Height       int
	FragmentPath string
	Channels     []channel
	Use4_5       bool
}

type ShaderToy struct {
	wConf          windowConf
	window         *glfw.Window
	vao            uint32
	program        uint32
	vertexShader   string
	fragmentShader string
	startTime      time.Time
	loadMutex      sync.Mutex
	channels       []uint32
	channelStr     []string
}

func NewShaderToy() *ShaderToy {
	instance := &ShaderToy{}
	instance.wConf.Width = 400
	instance.wConf.Height = 400
	instance.vertexShader = `
	#version 330 core
	layout(location = 0) in vec2 pos;

	void main()
	{
		gl_Position = vec4(pos, 1.0, 1.0);
	}
	`
	instance.fragmentShader = ``
	return instance
}

func (this *ShaderToy) Exec() {
	runtime.LockOSThread()
	content, err := ioutil.ReadFile("config.json")
	if err == nil {
		err = json.Unmarshal(content, &this.wConf)
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("Use Config")
		}
	} else {
		fmt.Println(err)
	}
	err = glfw.Init()
	if err != nil {
		panic(err)
	}
	this.window, err = glfw.CreateWindow(this.wConf.Width, this.wConf.Height, "ShaderToy", nil, nil)
	if err != nil {
		panic(err)
	}
	this.window.MakeContextCurrent()

	err = this.initGL()
	if err != nil {
		panic(err)
	}

	this.window.SetSizeCallback(func(w *glfw.Window, width int, height int) {
		gl.Viewport(0, 0, int32(width), int32(height))
	})

	this.window.SetMouseButtonCallback(func(w *glfw.Window, button glfw.MouseButton, action glfw.Action, mods glfw.ModifierKey) {
		if button == glfw.MouseButtonRight {
			err := this.loadShaderGL()
			if err != nil {
				fmt.Println(err)
			} else {
				err := this.loadChan()
				if err != nil {
					fmt.Println(err)
				} else {
					this.startTime = time.Now()
				}
			}
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

	err = this.loadChan()
	if err != nil {
		return
	}

	err = this.loadShaderGL()
	if err != nil {
		return
	}

	points := []float32{
		1.0, 1.0, 1.0, -1.0, -1.0, -1.0, -1.0, 1.0,
	}
	index := []uint32{
		0, 2, 1, 0, 3, 2,
	}

	if this.wConf.Use4_5 {
		gl.CreateVertexArrays(1, &this.vao)

		vbo := [2]uint32{}
		gl.CreateBuffers(2, &vbo[0])
		gl.NamedBufferData(vbo[0], len(points)*4, gl.Ptr(points), gl.STATIC_DRAW)
		gl.NamedBufferData(vbo[1], len(index)*4, gl.Ptr(index), gl.STATIC_DRAW)

		gl.VertexArrayVertexBuffer(this.vao, 0, vbo[0], 0, 2*4)
		gl.VertexArrayElementBuffer(this.vao, vbo[1])

		gl.EnableVertexArrayAttrib(this.vao, 0)
		gl.VertexArrayAttribFormat(this.vao, 0, 2, gl.FLOAT, false, 0)
	} else {
		gl.GenVertexArrays(1, &this.vao)
		gl.BindVertexArray(this.vao)

		var vbo uint32
		gl.GenBuffers(1, &vbo)
		gl.BindBuffer(gl.ARRAY_BUFFER, vbo)
		gl.BufferData(gl.ARRAY_BUFFER, len(points)*4, gl.Ptr(points), gl.STATIC_DRAW)
		gl.EnableVertexAttribArray(0)
		gl.VertexAttribPointer(0, 2, gl.FLOAT, false, 2*4, gl.PtrOffset(0))

		gl.GenBuffers(1, &vbo)
		gl.BindBuffer(gl.ELEMENT_ARRAY_BUFFER, vbo)
		gl.BufferData(gl.ELEMENT_ARRAY_BUFFER, len(index)*4, gl.Ptr(index), gl.STATIC_DRAW)
	}

	gl.ClearColor(0, 0, 0, 1)

	return
}

func (this *ShaderToy) loadShaderGL() (err error) {
	this.loadMutex.Lock()
	defer this.loadMutex.Unlock()
	content, err := ioutil.ReadFile(this.wConf.FragmentPath)
	if err == nil {
		this.fragmentShader =
			`#version 330 core
		uniform vec3      iResolution;           // viewport resolution (in pixels)
		uniform float     iTime;                 // shader playback time (in seconds)
		//uniform float     iTimeDelta;            // render time (in seconds)
		//uniform int       iFrame;                // shader playback frame
		//uniform float     iChannelTime[4];       // channel playback time (in seconds)
		//uniform vec3      iChannelResolution[4]; // channel resolution (in pixels)
		uniform vec4      iMouse;                // mouse pixel coords. xy: current (if MLB down), zw: click
		//uniform samplerXX iChannel0;          // input channel. XX = 2D/Cube
		//uniform vec4      iDate;                 // (year, month, day, time in seconds)
		//uniform float     iSampleRate;           // sound sample rate (i.e., 44100)
		` + strings.Join(this.channelStr, "\n") +
				`
				out vec4 fragColor;
				` + string(content) +
				`
				void main(){
			mainImage(fragColor, gl_FragCoord.xy);
		}`
		var program uint32
		program, err = newProgram(this.vertexShader, this.fragmentShader)
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

func (this *ShaderToy) loadChan() (err error) {
	channels := []uint32{}
	channelstr := []string{}
	for i := 0; i < len(this.wConf.Channels) && i < 4; i++ {
		channel := this.wConf.Channels[i]
		filter := gl.MIPMAP
		switch channel.Filter {
		case 1:
			filter = gl.NEAREST
		case 2:
			filter = gl.LINEAR
		case 3:
			filter = gl.MIPMAP
		}
		wrap := gl.REPEAT
		switch channel.Wrap {
		case 1:
			wrap = gl.CLAMP_TO_EDGE
		case 2:
			wrap = gl.REPEAT
		}
		tex, err := newTexture(this.wConf.Channels[i].Path, int32(filter), int32(wrap))
		if err != nil {
			fmt.Println(err)
			continue
		}
		texType := "2D"
		switch channel.Type {
		case 1:
			texType = "2D"
		case 2:
			texType = "Cube"
		}
		channelstr = append(channelstr, "uniform sampler"+texType+" iChannel"+strconv.Itoa(i)+";")
		channels = append(channels, tex)
	}
	if len(this.channels) > 0 {
		gl.DeleteTextures(int32(len(this.channels)), &this.channels[0])
	}
	this.channelStr = channelstr
	this.channels = channels
	return
}

func (this *ShaderToy) renderGL() {
	gl.Clear(gl.COLOR_BUFFER_BIT)
	gl.UseProgram(this.program)
	this.setUniform()
	if len(this.channels) > 0 {
		gl.ActiveTexture(gl.TEXTURE0)
		gl.BindTexture(gl.TEXTURE_2D, this.channels[0])
	}
	gl.BindVertexArray(this.vao)
	gl.DrawElements(gl.TRIANGLES, 6, gl.UNSIGNED_INT, gl.Ptr(nil))
}

func (this *ShaderToy) setUniform() {
	// iResolution
	loc := gl.GetUniformLocation(this.program, gl.Str("iResolution\x00"))
	w, h := this.window.GetSize()
	gl.Uniform3f(loc, float32(w), float32(h), 0)
	// iTime
	loc = gl.GetUniformLocation(this.program, gl.Str("iTime\x00"))
	t := float32(time.Since(this.startTime).Milliseconds()) / 1000.0
	gl.Uniform1f(loc, t)
	//
	if this.window.GetMouseButton(glfw.MouseButtonLeft) == glfw.Press {
		x, y := this.window.GetCursorPos()
		loc = gl.GetUniformLocation(this.program, gl.Str("iMouse\x00"))
		gl.Uniform4f(loc, float32(x), float32(y), 0, 0)
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

func newTexture(file string, filter int32, wrap int32) (uint32, error) {
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
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MIN_FILTER, filter)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_MAG_FILTER, filter)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_S, wrap)
	gl.TexParameteri(gl.TEXTURE_2D, gl.TEXTURE_WRAP_T, wrap)
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
