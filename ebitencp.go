package ebitencp

import (
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
	"github.com/jakecoffman/cp/v2"
)

const DrawPointLineScale = 1.0

type Drawer struct {
	Screen       *ebiten.Image
	ScreenWidth  int
	ScreenHeight int
	StrokeWidth  float32
	FlipYAxis    bool
	// Drawing colors
	Theme *Theme
	// GeoM for drawing vertices. Useful for cameras.
	// Apply GeoM to shift the drawing.
	GeoM      *ebiten.GeoM
	OptStroke *ebiten.DrawTrianglesOptions
	OptFill   *ebiten.DrawTrianglesOptions

	// Deprecated: Use GeoM instead of Camera
	Camera Camera
	// Deprecated: Use OptStroke and OptFill instead of AntiAlias
	AntiAlias bool

	handler    mouseEventHandler
	whiteImage *ebiten.Image
}

type Camera struct {
	Offset cp.Vector
}

func NewDrawer(screenWidth, screenHeight int) *Drawer {
	whiteImage := ebiten.NewImage(3, 3)
	whiteImage.Fill(color.White)
	antiAlias := true
	return &Drawer{
		whiteImage:   whiteImage,
		ScreenWidth:  screenWidth,
		ScreenHeight: screenHeight,
		AntiAlias:    antiAlias,
		StrokeWidth:  1,
		FlipYAxis:    false,
		Theme:        DefaultTheme(),
		GeoM:         &ebiten.GeoM{},
		Camera: Camera{
			Offset: cp.Vector{X: 0, Y: 0},
		},
		OptStroke: &ebiten.DrawTrianglesOptions{
			FillRule:  ebiten.FillRuleFillAll,
			AntiAlias: antiAlias,
		},
		OptFill: &ebiten.DrawTrianglesOptions{
			FillRule:  ebiten.FillRuleFillAll,
			AntiAlias: antiAlias,
		},
	}
}

func (d *Drawer) WithScreen(screen *ebiten.Image) *Drawer {
	d.Screen = screen
	return d
}

func (d *Drawer) DrawCircle(pos cp.Vector, angle, radius float64, outline, fill cp.FColor, data interface{}) {

	path := &vector.Path{}
	path.Arc(
		float32(pos.X),
		float32(pos.Y),
		float32(radius),
		0, 2*math.Pi, vector.Clockwise)
	d.drawFill(d.Screen, *path, fill.R, fill.G, fill.B, fill.A)

	path.MoveTo(
		float32(pos.X),
		float32(pos.Y))
	path.LineTo(
		float32(pos.X+math.Cos(angle)*radius),
		float32(pos.Y+math.Sin(angle)*radius))
	path.Close()

	d.drawOutline(d.Screen, *path, outline.R, outline.G, outline.B, outline.A)
}

func (d *Drawer) DrawSegment(a, b cp.Vector, fill cp.FColor, data interface{}) {
	var path *vector.Path = &vector.Path{}
	path.MoveTo(
		float32(a.X),
		float32(a.Y))
	path.LineTo(
		float32(b.X),
		float32(b.Y))
	path.Close()
	d.drawOutline(d.Screen, *path, fill.R, fill.G, fill.B, fill.A)
}

func (d *Drawer) DrawFatSegment(a, b cp.Vector, radius float64, outline, fill cp.FColor, data interface{}) {
	var path vector.Path = vector.Path{}
	t1 := float32(math.Atan2(b.Y-a.Y, b.X-a.X)) + math.Pi/2
	t2 := t1 + math.Pi
	path.Arc(
		float32(a.X),
		float32(a.Y),
		float32(radius),
		t1, t1+math.Pi, vector.Clockwise)
	path.Arc(
		float32(b.X),
		float32(b.Y),
		float32(radius),
		t2, t2+math.Pi, vector.Clockwise)
	path.Close()
	d.drawFill(d.Screen, path, fill.R, fill.G, fill.B, fill.A)
	d.drawOutline(d.Screen, path, outline.R, outline.G, outline.B, outline.A)
}

func (d *Drawer) DrawPolygon(count int, verts []cp.Vector, radius float64, outline, fill cp.FColor, data interface{}) {
	type ExtrudeVerts struct {
		offset, n cp.Vector
	}
	extrude := make([]ExtrudeVerts, count)

	for i := 0; i < count; i++ {
		v0 := verts[(i-1+count)%count]
		v1 := verts[i]
		v2 := verts[(i+1)%count]

		n1 := v1.Sub(v0).ReversePerp().Normalize()
		n2 := v2.Sub(v1).ReversePerp().Normalize()

		offset := n1.Add(n2).Mult(1.0 / (n1.Dot(n2) + 1.0))
		extrude[i] = ExtrudeVerts{offset, n2}
	}

	path := vector.Path{}
	inset := -math.Max(0, 1.0/DrawPointLineScale-radius)
	outset := 1.0/DrawPointLineScale + radius - inset
	outset2 := 1.0/DrawPointLineScale + radius - inset
	j := count - 1
	for i := 0; i < count; {
		vA := verts[i]
		vB := verts[j]

		nA := extrude[i].n
		nB := extrude[j].n

		offsetA := extrude[i].offset
		offsetB := extrude[j].offset

		innerA := vA.Add(offsetA.Mult(inset))
		innerB := vB.Add(offsetB.Mult(inset))

		outer0 := innerA.Add(nB.Mult(outset))
		outer1 := innerB.Add(nB.Mult(outset))
		outer2 := innerA.Add(offsetA.Mult(outset))
		outer3 := innerA.Add(offsetA.Mult(outset2))
		outer4 := innerA.Add(nA.Mult(outset))

		path.LineTo(
			float32(outer1.X),
			float32(outer1.Y))
		path.LineTo(
			float32(outer0.X),
			float32(outer0.Y))
		if radius != 0 {
			path.ArcTo(
				float32(outer3.X),
				float32(outer3.Y),
				float32(outer4.X),
				float32(outer4.Y),
				float32(radius),
			)
		} else {
			// ArcTo() and Arc() are very computationally expensive, so use LineTo()
			path.LineTo(
				float32(outer2.X),
				float32(outer2.Y))
		}

		j = i
		i++
	}
	path.Close()

	d.drawFill(d.Screen, path, fill.R, fill.G, fill.B, fill.A)
	d.drawOutline(d.Screen, path, outline.R, outline.G, outline.B, outline.A)
}
func (d *Drawer) DrawDot(size float64, pos cp.Vector, fill cp.FColor, data interface{}) {
	var path *vector.Path = &vector.Path{}
	path.Arc(
		float32(pos.X),
		float32(pos.Y),
		float32(2),
		0, 2*math.Pi, vector.Clockwise)
	path.Close()

	d.drawFill(d.Screen, *path, fill.R, fill.G, fill.B, fill.A)
}

func (d *Drawer) Flags() uint {
	return 0
}

func (d *Drawer) OutlineColor() cp.FColor {
	return toFColor(d.Theme.Outline)
}

func (d *Drawer) ShapeColor(shape *cp.Shape, data interface{}) cp.FColor {
	body := shape.Body()
	if body.IsSleeping() {
		return toFColor(d.Theme.ShapeSleeping)
	}

	if body.IdleTime() > shape.Space().SleepTimeThreshold {
		return toFColor(d.Theme.ShapeIdle)
	}
	return toFColor(d.Theme.Shape)
}

func (d *Drawer) ConstraintColor() cp.FColor {
	return toFColor(d.Theme.Constraint)
}

func (d *Drawer) CollisionPointColor() cp.FColor {
	return toFColor(d.Theme.CollisionPoint)
}

func (d *Drawer) Data() interface{} {
	return nil
}

func (d *Drawer) drawOutline(
	screen *ebiten.Image,
	path vector.Path,
	r, g, b, a float32,
) {
	sop := &vector.StrokeOptions{}
	sop.Width = d.StrokeWidth
	sop.LineJoin = vector.LineJoinRound
	vs, is := path.AppendVerticesAndIndicesForStroke(nil, nil, sop)
	applyMatrixToVertices(vs, *d.GeoM, &d.Camera, d.FlipYAxis, d.ScreenWidth, d.ScreenHeight, r, g, b, a)
	op := d.OptStroke
	screen.DrawTriangles(vs, is, d.whiteImage, op)
}

func (d *Drawer) drawFill(
	screen *ebiten.Image,
	path vector.Path,
	r, g, b, a float32,
) {
	vs, is := path.AppendVerticesAndIndicesForFilling(nil, nil)
	applyMatrixToVertices(vs, *d.GeoM, &d.Camera, d.FlipYAxis, d.ScreenWidth, d.ScreenHeight, r, g, b, a)
	op := d.OptFill
	screen.DrawTriangles(vs, is, d.whiteImage, op)
}

func applyMatrixToVertices(vs []ebiten.Vertex, matrix ebiten.GeoM, camera *Camera, flipYAxis bool, screenWidth, screenHeight int, r, g, b, a float32) {
	var f float64 = -1
	if flipYAxis {
		f = 1
	}

	matrix.Scale(1, f)
	matrix.Translate(-camera.Offset.X, -camera.Offset.Y*f)
	matrix.Translate(float64(screenWidth)/2.0, float64(screenHeight)/2.0)
	for i := range vs {
		x, y := matrix.Apply(float64(vs[i].DstX), float64(vs[i].DstY))
		vs[i].DstX, vs[i].DstY = float32(x), float32(y)
		vs[i].SrcX, vs[i].SrcY = 1, 1
		vs[i].ColorR, vs[i].ColorG, vs[i].ColorB, vs[i].ColorA = r, g, b, a
	}
}

// ScreenToWorld converts screen-space coordinates to world-space
func ScreenToWorld(screenPoint cp.Vector, cameraGeoM ebiten.GeoM, camera Camera, flipYAxis bool, screenWidth, screenHeight int) cp.Vector {
	if cameraGeoM.IsInvertible() {
		cameraGeoM.Invert()
		var f float64 = -1
		if flipYAxis {
			f = 1
		}
		matrix := &ebiten.GeoM{}
		matrix.Translate(-float64(screenWidth)/2.0, -float64(screenHeight)/2.0)
		matrix.Translate(camera.Offset.X, camera.Offset.Y*f)
		matrix.Scale(1, f)
		matrix.Concat(cameraGeoM)
		worldX, worldY := matrix.Apply(screenPoint.X, screenPoint.Y)
		return cp.Vector{X: worldX, Y: worldY}
	} else {
		// When scaling it can happened that matrix is not invertable
		return cp.Vector{X: math.NaN(), Y: math.NaN()}
	}
}

func (d *Drawer) HandleMouseEvent(space *cp.Space) {
	d.handler.handleMouseEvent(
		d,
		space,
		d.ScreenWidth,
		d.ScreenHeight,
	)
}

// event handling

const GRABBABLE_MASK_BIT uint = 1 << 31

var grabFilter cp.ShapeFilter = cp.ShapeFilter{
	Group:      cp.NO_GROUP,
	Categories: GRABBABLE_MASK_BIT,
	Mask:       GRABBABLE_MASK_BIT,
}

type mouseEventHandler struct {
	mouseJoint *cp.Constraint
	mouseBody  *cp.Body
	touchIDs   []ebiten.TouchID
}

func (h *mouseEventHandler) handleMouseEvent(d *Drawer, space *cp.Space, screenWidth, screenHeight int) {
	if h.mouseBody == nil {
		h.mouseBody = cp.NewKinematicBody()
	}

	var x, y int

	// touch position
	for _, id := range h.touchIDs {
		x, y = ebiten.TouchPosition(id)
		if x == 0 && y == 0 || inpututil.IsTouchJustReleased(id) {
			h.onMouseUp(space)
			h.touchIDs = []ebiten.TouchID{}
			break
		}
	}
	isJuestTouched := false
	touchIDs := inpututil.AppendJustPressedTouchIDs(h.touchIDs[:0])
	for _, id := range touchIDs {
		isJuestTouched = true
		h.touchIDs = []ebiten.TouchID{id}
		x, y = ebiten.TouchPosition(id)
		break
	}
	// mouse position
	if len(h.touchIDs) == 0 {
		x, y = ebiten.CursorPosition()
	}

	cursorPosition := cp.Vector{X: float64(x), Y: float64(y)}
	cursorPosition = ScreenToWorld(cursorPosition, *d.GeoM, d.Camera, d.FlipYAxis, screenWidth, screenHeight)

	if isJuestTouched {
		h.mouseBody.SetVelocityVector(cp.Vector{})
		h.mouseBody.SetPosition(cursorPosition)
	} else {
		newPoint := h.mouseBody.Position().Lerp(cursorPosition, 0.25)
		h.mouseBody.SetVelocityVector(newPoint.Sub(h.mouseBody.Position()).Mult(60.0))
		h.mouseBody.SetPosition(newPoint)
	}

	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || isJuestTouched {
		h.onMouseDown(space, cursorPosition)
	}
	if inpututil.IsMouseButtonJustReleased(ebiten.MouseButtonLeft) {
		h.onMouseUp(space)
	}
}

func (h *mouseEventHandler) onMouseDown(space *cp.Space, cursorPosition cp.Vector) {
	// give the mouse click a little radius to make it easier to click small shapes.
	radius := 5.0

	info := space.PointQueryNearest(cursorPosition, radius, grabFilter)

	if info.Shape != nil && info.Shape.Body().Mass() < cp.INFINITY {
		var nearest cp.Vector
		if info.Distance > 0 {
			nearest = info.Point
		} else {
			nearest = cursorPosition
		}

		body := info.Shape.Body()
		h.mouseJoint = cp.NewPivotJoint2(h.mouseBody, body, cp.Vector{}, body.WorldToLocal(nearest))
		h.mouseJoint.SetMaxForce(50000)
		h.mouseJoint.SetErrorBias(math.Pow(1.0-0.15, 60.0))
		space.AddConstraint(h.mouseJoint)
	}
}

func (h *mouseEventHandler) onMouseUp(space *cp.Space) {
	if h.mouseJoint == nil {
		return
	}
	space.RemoveConstraint(h.mouseJoint)
	h.mouseJoint = nil
}
