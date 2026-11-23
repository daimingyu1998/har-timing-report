package report

import (
	_ "embed"
	"html/template"
	"io"
	"strconv"
)

//go:embed waterfall.html
var waterfallHTML string

var waterfall = template.Must(template.New("waterfall").Funcs(template.FuncMap{"ms": ms}).Parse(waterfallHTML))

type waterfallRow struct {
	Request
	StatusLabel string
	Kind        string
	Plottable   bool
	Left        float64
	Width       float64
}

func (r Report) WriteHTML(w io.Writer) error {
	var extent int64
	for _, row := range r.Requests {
		if row.StartMS != nil && row.TotalMS != nil {
			extent = max(extent, *row.StartMS+*row.TotalMS)
		}
	}
	rows := make([]waterfallRow, 0, len(r.Requests))
	for _, req := range r.Requests {
		row := waterfallRow{Request: req, StatusLabel: "—", Kind: "missing"}
		if req.Status != nil {
			row.StatusLabel = strconv.Itoa(*req.Status)
			row.Kind = "success"
			if *req.Status >= 400 {
				row.Kind = "error"
			}
		}
		if req.StartMS != nil && req.TotalMS != nil {
			row.Plottable = true
			if extent > 0 {
				row.Left = 100 * float64(*req.StartMS) / float64(extent)
				row.Width = 100 * float64(*req.TotalMS) / float64(extent)
			}
		}
		rows = append(rows, row)
	}
	return waterfall.Execute(w, struct {
		Report
		Rows  []waterfallRow
		EndMS int64
	}{r, rows, extent})
}
