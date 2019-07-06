package main

import (
	"flag"
	"fmt"
	"log"

	"github.com/gdey/dxf/color"
	"github.com/gdey/dxf/insunit"
	"github.com/gdey/dxf/table"
	"github.com/jackc/pgx"

	"github.com/gdey/dxf"
	"github.com/gdey/dxf/drawing"
	"github.com/gdey/dxf/entity"
	"github.com/go-spatial/geom"
	"github.com/go-spatial/geom/encoding/wkb"
)

var (
	PgHost     = flag.String("host", "localhost", "hostname for the database")
	PgPort     = flag.Int("port", DefaultPort, "port for the database")
	PgDatabase = flag.String("db", "scoutred-edge", "the database name to connect to")
	PgUser     = flag.String("user", "postgres", "the database username to use for connection")
	PgPassword = flag.String("password", "", "the database password")
	// 327496
	parcelID = 327496
	VPHeight = flag.Float64("vp-height", 15000.0, "The view port height")
)

func init() {
	flag.IntVar(&parcelID, "parcel", 327496, "the parsel to query for.")
}

const (
	QueryFeatures = `
SELECT DISTINCT ON (parcel.geom_4326)
			parcel.id as ID,
			ST_AsBinary(
				ST_Scale(
					ST_Transform(parcel.geom_4326,2230),
					12.0, -- TODO: support for feet. needs to be dynamic
					12.0 -- TODO: support for feet. needs to be dynamic
				)
			) AS GEOM
		FROM
			gis.parcels parcel,
			(
				SELECT
					ST_Expand(p.geom_4326,.001) AS geom_4326
				FROM
					gis.parcels p
				WHERE 
					p.id=$1
			) AS buffer
		WHERE
			ST_Intersects(parcel.geom_4326, buffer.geom_4326)
			AND parcel.id != $1
`
	QueryMainFeature = `
SELECT 
	id, 
	ST_AsBinary(
		ST_SCALE(
			ST_Transform(p.geom_4326,2230),
			12.0, -- TODO: support for feet, needs to be dynamic
			12.0  -- TODO: support for feet. needs to be dynamic
		)
	) AS geom
FROM
	gis.parcels p
WHERE
    p.id=$1
`
)

type Feature struct {
	Id   int
	Geom geom.Geometry
}

func drawLine(d *drawing.Drawing, closed bool, line [][2]float64, asSurface bool) (*entity.LwPolyline, error) {
	if len(line) == 0 {
		return nil, nil
	}
	vtx := make([][]float64, len(line))
	for i := range line {
		vtx[i] = line[i][:]
	}
	// Start by just drawing the points with a lwpolygline
	return d.LwPolyline(closed, vtx...)
}
func drawPolygon(d *drawing.Drawing, polygon [][][2]float64, asSurface bool) ([]*entity.LwPolyline, error) {
	entities := make([]*entity.LwPolyline, 0, len(polygon))
	for _, l := range polygon {
		ent, err := drawLine(d, true, l, asSurface)
		if err != nil {
			return nil, err
		}
		if ent == nil {
			continue
		}
		entities = append(entities, ent)
	}
	return entities, nil
}

func drawGeometry(d *drawing.Drawing, geo geom.Geometry, asSurface bool) ([]entity.Entity, error) {
	switch g := geo.(type) {
	case geom.Polygon:
		ents, err := drawPolygon(d, g.LinearRings(), asSurface)
		if err != nil {
			return nil, err
		}
		rent := make([]entity.Entity, len(ents))
		for i := range ents {
			rent[i] = ents[i]
		}
		return rent, nil
	case geom.MultiPolygon:
		rent := []entity.Entity{}
		for _, ply := range g.Polygons() {
			ents, err := drawPolygon(d, ply, asSurface)
			if err != nil {
				return nil, err
			}
			for i := range ents {
				rent = append(rent, ents[i])
			}
		}
		return rent, nil
	case *geom.Polygon:
		if g == nil {
			return nil, nil
		}
		ents, err := drawPolygon(d, g.LinearRings(), asSurface)
		if err != nil {
			return nil, err
		}
		rent := make([]entity.Entity, len(ents))
		for i := range ents {
			rent[i] = ents[i]
		}
		return rent, nil
	case *geom.MultiPolygon:
		if g == nil {
			return nil, nil
		}
		rent := []entity.Entity{}
		for _, ply := range g.Polygons() {
			ents, err := drawPolygon(d, ply, asSurface)
			if err != nil {
				return nil, err
			}
			for i := range ents {
				rent = append(rent, ents[i])
			}
		}
		return rent, nil
	default:
		return nil, fmt.Errorf("unsupported: %T", g)
	}
}

func GetFeatures(pool *pgx.ConnPool, Query string, parcelID int) ([]Feature, error) {
	var features []Feature
	rows, err := pool.Query(Query, parcelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rowCount := 0
	for rows.Next() {
		rowCount++
		var ft Feature
		var geom []byte
		if err := rows.Scan(&ft.Id, &geom); err != nil {
			log.Printf("Got an error trying to get info for row(%v): %v", rowCount, err)
			continue
		}

		if len(geom) == 0 {
			continue
		}
		ft.Geom, err = wkb.DecodeBytes(geom)
		if err != nil {
			log.Printf("Failed to decode bytes for row(%v): %v", rowCount, err)
			continue
		}
		features = append(features, ft)
	}
	return features, nil
}

func main() {

	flag.Parse()
	pool, _ := NewConnection(*PgHost, *PgPort, *PgDatabase, *PgUser, *PgPassword)

	d := dxf.NewDrawing()
	d.Header().LtScale = 1
	d.Header().InsUnit = insunit.Inches
	d.Header().InsLUnit = insunit.Architectural

	var centerPoint []float64

	features, err := GetFeatures(pool, QueryFeatures, parcelID)
	if err != nil {
		log.Fatalf("Failed to get general features: %v", err)
	}
	if len(features) != 0 {
		d.AddLayer("C-PARCEL-ADJ", color.White, dxf.DefaultLineType, true)
		for i, f := range features {
			_, err := drawGeometry(d, f.Geom, false)
			if err != nil {
				log.Printf("got error drawing: %v: %v", i, err)
				continue
			}
		}
	}

	features, err = GetFeatures(pool, QueryMainFeature, parcelID)
	if err != nil {
		log.Fatalf("Failed to get main features: %v", err)
	}
	if len(features) != 0 {
		var ext *geom.Extent
		d.AddLayer("C-PARCEL", color.Red, dxf.DefaultLineType, true)
		for i, f := range features {
			if ext == nil {
				ext, err = geom.NewExtentFromGeometry(f.Geom)
				if err != nil {
					log.Printf("got error getting extent of geom: %v: %v", i, err)
					continue
				}
			} else {
				ext.AddGeometry(f.Geom)
			}
			_, err = drawGeometry(d, f.Geom, false)
		}
		centerPoint = []float64{
			ext.MinX() + ((ext.MaxX() - ext.MinX()) / 2),
			ext.MinY() + ((ext.MaxY() - ext.MinY()) / 2),
		}
	}

	d.SetExt()
	if tables, ok := d.Sections[2].(table.Tables); ok {
		// 0 is the view port
		vp := table.NewViewport("*ACTIVE")
		if len(centerPoint) != 0 {
			vp.ViewCenter = centerPoint
		}
		vp.UpperRight = []float64{1.0, 1.0}
		vp.Height = *VPHeight
		vp.SnapSpacing = []float64{1.0, 1.0}
		tables[0].Add(vp)
	} else {
		log.Printf("Did not get correct type: %t", d.Sections[2])
	}

	err = d.SaveAs(fmt.Sprintf("%v_features.dxf", parcelID))
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Wrote out %v_features.dxf", parcelID)
}
