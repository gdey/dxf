package entity

import (
	"github.com/yofu/dxf/format"
)

type Vertex struct {
	*entity
	Flag  int
	Coord []float64
}

func (p *Vertex) IsEntity() bool {
	return true
}

func NewVertex(x, y, z float64) *Vertex {
	v := &Vertex{
		NewEntity(VERTEX),
		32,
		[]float64{x, y, z},
	}
	return v
}

func (v *Vertex) Format(f *format.Formatter) {
	v.entity.Format(f)
	f.WriteString(100, "AcDbVertex")
	f.WriteString(100, "AcDb3DPolylineVertex")
	f.WriteInt(70, v.Flag)
	for i := 0; i < 3; i++ {
		f.WriteFloat((i+1)*10, v.Coord[i])
	}
}

func (v *Vertex) String() string {
	f := format.New()
	return v.FormatString(f)
}

func (v *Vertex) FormatString(f *format.Formatter) string {
	v.Format(f)
	return f.Output()
}