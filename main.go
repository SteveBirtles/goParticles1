package main

import (
	"fmt"
	"github.com/go-gl/gl/v4.3-core/gl"
	"github.com/go-gl/glfw/v3.2/glfw"
	"github.com/go-gl/mathgl/mgl32"
	_ "image/png"
	"io/ioutil"
	"log"
	"math"
	"math/rand"
	"runtime"
	"strings"
	"time"
)

const (
	windowWidth  = 1280
	windowHeight = 720
	numParticles = 1000000
	threads      = 4
)

var (
	frames            = 0
	second            = time.Tick(time.Second)
	windowTitlePrefix = "Particles"
	vao               uint32
	particles         []mgl32.Vec4
	velocities        []mgl32.Vec4
	done              = make(chan bool, threads)
)

func init() {

	runtime.LockOSThread()

}

func LoadShader(path string, shaderType uint32) uint32 {
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		panic(err)
	}
	source := string(bytes) + "\x00"

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

		panic(fmt.Errorf("failed to compile %v: %v", source, log))
	}

	return shader
}

func updateParticles(start, end int) {

	if end > len(particles) {
		end = len(particles)
	}

	for i := start; i < end; i++ {

		t := float32(0.01)

		pPos := particles[i]
		vPos := velocities[i]

		d := float32(math.Hypot(float64(vPos.X()), math.Hypot(float64(vPos.Y()), float64(vPos.Z()))))

		var g mgl32.Vec3
		g[0] = float32(pPos.X()/d) * -9.0
		g[1] = float32(pPos.Y()/d) * -9.0
		g[2] = float32(pPos.Z()/d) * -9.0

		particles[i][0] = float32(pPos.X() + vPos.X()*t + 0.5*t*t*g.X())
		particles[i][1] = float32(pPos.Y() + vPos.Y()*t + 0.5*t*t*g.Y())
		particles[i][2] = float32(pPos.Z() + vPos.Z()*t + 0.5*t*t*g.Z())

		velocities[i][0] = vPos.X() + g.X()*t
		velocities[i][1] = vPos.Y() + g.Y()*t
		velocities[i][2] = vPos.Z() + g.Z()*t

	}

	done <- true

}

func main() {

	var err error
	if err = glfw.Init(); err != nil {
		log.Fatalln("failed to initialize glfw:", err)
	}

	glfw.WindowHint(glfw.Resizable, glfw.False)
	glfw.WindowHint(glfw.ContextVersionMajor, 4)
	glfw.WindowHint(glfw.ContextVersionMinor, 1)
	glfw.WindowHint(glfw.OpenGLProfile, glfw.OpenGLCoreProfile)
	glfw.WindowHint(glfw.OpenGLForwardCompatible, glfw.True)

	window, err := glfw.CreateWindow(windowWidth, windowHeight, windowTitlePrefix, nil, nil)
	if err != nil {
		panic(err)
	}
	window.MakeContextCurrent()

	if err = gl.Init(); err != nil {
		panic(err)
	}
	gl.ClearColor(0.0, 0.0, 0.0, 1.0)
	glfw.SwapInterval(0)

	vertexShader := LoadShader("shaders/vert.glsl", gl.VERTEX_SHADER)
	fragmentShader := LoadShader("shaders/frag.glsl", gl.FRAGMENT_SHADER)

	posSSBO := uint32(1)
	velSSBO := uint32(2)

	for i := 0; i < numParticles; i++ {
		x := (rand.Float32()*2 - 1) * float32(32)
		y := (rand.Float32()*2 - 1) * float32(32)
		z := (rand.Float32()*2 - 1) * float32(32)
		particles = append(particles, mgl32.Vec4{x, y, z, 1})
	}

	gl.GenBuffers(1, &posSSBO)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, posSSBO)
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, numParticles*16, gl.Ptr(particles), gl.DYNAMIC_DRAW)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 0, posSSBO)

	for i := 0; i < numParticles; i++ {
		x := (rand.Float32()*2 - 1) * float32(10.0)
		y := (rand.Float32()*2 - 1) * float32(10.0)
		z := (rand.Float32()*2 - 1) * float32(10.0)
		velocities = append(velocities, mgl32.Vec4{x, y, z, 0})
	}

	gl.GenBuffers(1, &velSSBO)
	gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, velSSBO)
	gl.BufferData(gl.SHADER_STORAGE_BUFFER, numParticles*16, gl.Ptr(velocities), gl.DYNAMIC_DRAW)
	gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 1, velSSBO)

	quadProg := gl.CreateProgram()
	gl.AttachShader(quadProg, vertexShader)
	gl.AttachShader(quadProg, fragmentShader)
	gl.LinkProgram(quadProg)

	gl.UseProgram(quadProg)

	gl.GenVertexArrays(1, &vao)
	gl.BindVertexArray(vao)
	gl.BindBuffer(gl.ARRAY_BUFFER, posSSBO)
	gl.VertexAttribPointer(0, 4, gl.FLOAT, false, 0, nil)
	gl.EnableVertexAttribArray(0)

	position := mgl32.Vec3{0, 0, 100}
	target := mgl32.Vec3{0, 0, 0}
	up := mgl32.Vec3{0, 1, 0}
	view := mgl32.LookAtV(position, target, up)
	projection := mgl32.Perspective(mgl32.DegToRad(60), float32(windowWidth)/float32(windowHeight), 0.1, 1000.0)

	projUniform := int32(1)
	gl.UniformMatrix4fv(projUniform, 1, false, &projection[0])

	viewUniform := int32(2)
	gl.UniformMatrix4fv(viewUniform, 1, false, &view[0])

	for !window.ShouldClose() {

		if window.GetKey(glfw.KeyEscape) == glfw.Press {
			window.SetShouldClose(true)
		}

		/* --------------------------- */

		batchSize := (len(particles) + threads) / 4

		for i := 0; i < threads; i++ {
			go updateParticles(i*batchSize, (i+1)*batchSize)
		}

		for {
			if len(done) == 4 {
				for len(done) > 0 {
					<-done
				}
				break
			}
		}

		gl.BindBuffer(gl.SHADER_STORAGE_BUFFER, posSSBO)
		gl.BufferData(gl.SHADER_STORAGE_BUFFER, numParticles*16, gl.Ptr(particles), gl.DYNAMIC_DRAW)
		gl.BindBufferBase(gl.SHADER_STORAGE_BUFFER, 0, posSSBO)

		gl.UseProgram(quadProg)
		gl.ClearColor(0, 0, 0, 1)
		gl.Clear(gl.COLOR_BUFFER_BIT | gl.DEPTH_BUFFER_BIT)
		gl.BindVertexArray(vao)
		gl.DrawArrays(gl.POINTS, 0, numParticles)

		/* --------------------------- */

		gl.UseProgram(0)
		window.SwapBuffers()

		glfw.PollEvents()
		frames++
		select {
		case <-second:
			window.SetTitle(fmt.Sprintf("%s | FPS: %d", windowTitlePrefix, frames))
			frames = 0
		default:
		}

	}

	glfw.Terminate()
}
