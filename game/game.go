package game

import (
	"korok.io/korok/anim"
	"korok.io/korok/anim/frame"
	"korok.io/korok/asset"
	"korok.io/korok/audio"
	"korok.io/korok/effect"
	"korok.io/korok/engi"
	"korok.io/korok/gfx"
	"korok.io/korok/gfx/dbg"
	"korok.io/korok/gui"
	"korok.io/korok/hid/input"

	"log"
	"reflect"
	"time"
)

const (
	MaxScriptSize = 1024

	MaxSpriteSize = 64 << 10
	MaxTransformSize = 64 << 10
	MaxTextSize = 64 << 10
	MaxMeshSize = 64 << 10

	MaxParticleSize = 1024
)


type Options struct {
	W, H int
}

type Table interface{}

type DB struct {
	EntityM *engi.EntityManager
	Tables  []interface{}
}

type appState struct {
	old struct{
		paused bool
		lostFocus bool
	}
	now struct{
		paused bool
		lostFocus bool
	}
	PauseCallback func(paused bool)
	FocusCallback func(focused bool)
}

//func (app appState) Paused() (paused bool, old bool){
//	paused = app.now.paused
//	old = app.old.paused
//	return
//}
//
//func (app appState) Focused() (focused bool, old bool) {
//	focused = !app.now.lostFocus
//	old = !app.old.lostFocus
//	return
//}

func (app *appState) setPaused(paused bool) {
	app.old.paused = app.now.paused
	app.now.paused = paused
}

func (app *appState) setFocused(focused bool) {
	app.old.lostFocus = app.now.lostFocus
	app.now.lostFocus = !focused
}

// 统一管理游戏各个子系统的创建和销毁的地方
var G *Game

type Game struct {
	Options
	FPS
	DB

	// scene manager
	SceneManager

	// system
	*gfx.RenderSystem
	*input.InputSystem
	*effect.ParticleSimulateSystem
	*ScriptSystem
	*anim.AnimationSystem

	// game state
	appState
	spriteTable     *gfx.SpriteTable
	meshTable       *gfx.MeshTable
	xfTable         *gfx.TransformTable
	textTable       *gfx.TextTable
	psTable         *effect.ParticleSystemTable
	spriteAnimTable *frame.FlipbookTable
}

func (g *Game) Camera() *gfx.Camera {
	return &g.RenderSystem.MainCamera
}

/// window callback
func (g *Game) OnCreate(w, h float32, ratio float32) {
	g.Create(w, h, ratio)
}

func (g *Game) OnLoop() {
	g.Update()
}

func (g *Game) OnDestroy() {
	g.Destroy()
}

func (g *Game) OnPause() {
	g.notifyPause()
}

func (g *Game) OnResume() {
	g.notifyResume()
}

func (g *Game) OnFocusChanged(focused bool) {
	g.appState.setFocused(focused)
	if fn := g.FocusCallback; fn != nil {
		fn(focused)
	}
	log.Println("window focuse changed !!", focused)
}

/// input callback
func (g *Game) OnKeyEvent(key int, pressed bool) {
	g.InputSystem.SetKeyEvent(key, pressed)
}

func (g *Game) OnPointEvent(key int, pressed bool, x, y float32) {
	g.InputSystem.SetPointerEvent(key, pressed, x, y)
}

func (g *Game) OnResize(w, h int32) {
	g.setGameSize(float32(w), float32(h))
}

func (g *Game) setGameSize(w, h float32) {
	// setup camera
	if rs := g.RenderSystem; rs != nil {
		rs.MainCamera.SetViewPort(w, h)
	}

	// gui real screen size
	gui.SetScreenSize(w, h)
}

// init subsystem
func (g *Game) Create(w, h float32, ratio float32) {
	g.FPS.initialize()
	gfx.Init(ratio)
	audio.Init()

	// render system
	rs := gfx.NewRenderSystem()
	g.RenderSystem = rs

	// init game window size
	g.setGameSize(w, h); g.MainCamera.MoveTo(w/2, h/2)

	// set table
	rs.RequireTable(g.DB.Tables)
	// set render
	var vertex, color string

	vertex, color = asset.Shader.GetShaderStr("batch")
	batchRender := gfx.NewBatchRender(vertex, color)
	rs.RegisterRender(gfx.RenderType(0), batchRender)

	vertex, color = asset.Shader.GetShaderStr("mesh")
	meshRender := gfx.NewMeshRender(vertex, color)
	rs.RegisterRender(gfx.RenderType(1), meshRender)

	// set feature
	srf := &gfx.SpriteRenderFeature{}
	srf.Register(rs)
	mrf := &gfx.MeshRenderFeature{}
	mrf.Register(rs)
	trf := &gfx.TextRenderFeature{}
	trf.Register(rs)

	// gui system
	ui := &gui.UIRenderFeature{}
	ui.Register(rs)

	/// init debug
	dbg.Init(g.Options.W, g.Options.H)

	/// input system
	g.InputSystem = input.NewInputSystem()

	/// particle-simulation system
	pss := effect.NewSimulationSystem()
	pss.RequireTable(g.DB.Tables)
	g.ParticleSimulateSystem = pss
	// set feature
	prf := &effect.ParticleRenderFeature{}
	prf.Register(rs)

	/// script system
	g.ScriptSystem = NewScriptSystem()
	g.ScriptSystem.RequireTable(g.DB.Tables)

	/// Tex2D animation system
	g.AnimationSystem = anim.NewAnimationSystem()
	g.AnimationSystem.RequireTable(g.DB.Tables)
	anim.SetDefaultAnimationSystem(g.AnimationSystem)

	// audio system

	/// setup scene manager
	g.SceneManager.Setup(g)

	log.Println("Load Feature:", len(rs.FeatureList))
	for i, v := range rs.FeatureList {
		log.Println(i, " feature - ", reflect.TypeOf(v))
	}
}

// destroy subsystem
func (g *Game) Destroy() {
	// clear scene stack
	g.SceneManager.Clear()

	// destroy other system
	g.RenderSystem.Destroy()

	// dbg system
	dbg.Destroy()

	// audio system
	audio.Destroy()
}

func (g *Game) notifyPause() {
	g.setPaused(true)
	if fn := g.PauseCallback; fn != nil {
		fn(true)
	}
	log.Println("game paused..")
}

func (g *Game) notifyResume() {
	g.setPaused(false)
	if fn := g.PauseCallback; fn != nil {
		fn(false)
	}
	log.Println("game resumed..")
}

func (g *Game) Init() {
	g.loadTables()
}

func (g *Game) loadTables() {
	g.DB.EntityM = engi.NewEntityManager()

	// init tables
	scriptTable := NewScriptTable(MaxScriptSize)
	tagTable := &TagTable{}

	g.DB.Tables = append(g.DB.Tables, scriptTable, tagTable)

	g.spriteTable = gfx.NewSpriteTable(MaxSpriteSize)
	g.meshTable = gfx.NewMeshTable(MaxMeshSize)
	g.xfTable = gfx.NewTransformTable(MaxTransformSize)
	g.textTable = gfx.NewTextTable(MaxTextSize)

	g.DB.Tables = append(g.DB.Tables, g.spriteTable, g.meshTable, g.xfTable, g.textTable)

	g.psTable = effect.NewParticleSystemTable(MaxParticleSize)
	g.DB.Tables = append(g.DB.Tables, g.psTable)

	g.spriteAnimTable = frame.NewFlipbookTable(MaxSpriteSize)
	g.DB.Tables = append(g.DB.Tables, g.spriteAnimTable)
}

func (g *Game) GetSpriteTable()*gfx.SpriteTable  {
	return g.spriteTable
}

func (g *Game) GetTransFormTable()*gfx.TransformTable  {
	return g.xfTable
}

//get mesh table
func (g *Game) GetMeshTable()*gfx.MeshTable  {
	return g.meshTable
}

//get floobook table
func (g *Game) GetFlipbookTable()*frame.FlipbookTable  {
	return g.spriteAnimTable
}

//get text table
func (g *Game) GetTextTable()*gfx.TextTable  {
	return g.textTable
}

//get particle system table
func (g *Game) GetParticleSystemTable()*effect.ParticleSystemTable  {
	return g.psTable
}

func (g *Game) Input(dt float32) {

}

func (g *Game) Update() {
	// update
	dt := g.FPS.Smooth()

	// ease cpu usage TODO
	if g.now.paused || (g.now.lostFocus && dt < 0.016) {
		time.Sleep(time.Duration((0.016-dt)*1000)*time.Millisecond)
	}

	// update input-system
	g.InputSystem.AdvanceFrame()

	// update scene
	g.SceneManager.Update(dt)

	// update script
	g.ScriptSystem.Update(dt)

	g.InputSystem.Reset()

	//// simulation....

	// update sprite animation
	g.AnimationSystem.Update(dt)

	/// 动画更新，骨骼数据
	///g.AnimationSystem.Update(dt)

	// g.CollisionSystem.Update(dt)

	// 粒子系统更新
	g.ParticleSimulateSystem.Update(dt)

	// Render
	g.RenderSystem.Update(dt)

	// fps & profile
	g.DrawProfile()

	//bk.Dump()
	audio.AdvanceFrame()

	// flush drawCall
	num := gfx.Flush()

	// drawCall = all-drawCall - camera-drawCall
	dc := num - len(g.RenderSystem.RenderList)
	dbg.LogFPS(int(g.fps), dc)
}

func (g *Game) DrawProfile() {
	// Advance frame
	dbg.AdvanceFrame()
}

func (g *Game) Draw(dt float32) {
}
