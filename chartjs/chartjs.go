package chartjs

import (
	"bytes"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
)

// ChartJS allows building an HTML/Javascript Chart.js chart from a given dataset
type ChartJS struct {
	data dataset
}

// New constructs a new ChartJS instance
func New(
	chartType string,
	fss [][]float64,
	sss [][]string,
	tss [][]time.Time,
	minFSS []float64,
	maxFSS []float64,
	title string,
	scaleType string,
	xLabel string,
	yLabel string,
	zeroBased bool,
	colorType int) ChartJS {

	if chartType == "bar" {
		zeroBased = true // https://github.com/marianogappa/chart/issues/11
	}

	var d = dataset{
		ChartType: chartType,
		FSS:       fss,
		SSS:       sss,
		TSS:       tss,
		MinFSS:    minFSS,
		MaxFSS:    maxFSS,
		Title:     title,
		ScaleType: scaleType,
		XLabel:    xLabel,
		YLabel:    yLabel,
		ZeroBased: zeroBased,
		ColorType: colorType,
	}

	if chartType == "line" && d.canBeScatterLine() {
		sort.Sort(&d)
	}

	return ChartJS{d}
}

// MustBuild prepares the dataset and executes the text template with it. Fatals if there's a problem
// with executing the template.
func (c ChartJS) MustBuild() bytes.Buffer {
	b, err := c.Build()
	if err != nil {
		log.Fatal(err)
	}
	return b
}

// Build prepares the dataset and executes the text template with it. Returns an error if there's a problem
// with executing the template.
func (c ChartJS) Build() (bytes.Buffer, error) {
	var b bytes.Buffer
	if err := cjsTemplate.Execute(&b, c.getData()); err != nil {
		return b, fmt.Errorf("could't prepare ChartJS js code for chart: [%v]", err)
	}
	return b, nil
}

type cjsData struct {
	ChartType       string // for Chart.js
	ActualChartType string // for algorithm
	Title           string
	ScaleType       string
	XLabel          string
	YLabel          string
	ZeroBased       bool
	Labels          string // Need backticks; can't use array
	Datasets        []cjsDataset
	TooltipCallback string
	UsesTimeScale   bool
	ColorType       int
}

type cjsDataset struct {
	SimpleData      []string
	ComplexData     []cjsDataPoint
	BackgroundColor string
	Fill            bool
	Label           string
	BorderColor     string
}

type cjsDataPoint struct {
	X, Y, R string
	UsesR   bool
}

func (c ChartJS) getData() cjsData {
	d := c.labelsAndDatasets()
	d.Title = c.data.Title
	d.ScaleType = c.data.ScaleType
	d.ColorType = c.data.ColorType
	d.XLabel = c.data.XLabel
	d.YLabel = c.data.YLabel
	d.ZeroBased = c.data.ZeroBased
	d.TooltipCallback = c.tooltipCallback()

	return d
}

func (c ChartJS) labelsAndDatasets() cjsData {
	var usesTimeScale bool
	if c.data.ChartType == "line" && (!c.data.hasStrings() || c.data.hasTimes()) {
		c.data.ChartType = "scatterline"
		if c.data.hasStrings() && c.data.floatFieldLen()+c.data.timeFieldLen() >= 2 {
			c.data.ChartType = "denormalised-scatterline" // every line is one datapoint rather than a column
		}
	}
	switch c.data.ChartType {
	case "pie":
		return cjsData{
			ChartType:       "pie",
			ActualChartType: "pie",
			Labels:          c.marshalLabels(),
			Datasets: []cjsDataset{{
				Fill:            true,
				SimpleData:      c.marshalSimpleData(0),
				BackgroundColor: colorFirstN(c.data.ColorType, len(c.data.FSS)),
			}},
		}
	case "bar":
		if len(c.data.FSS[0]) == 1 {
			return cjsData{
				ChartType:       "bar",
				ActualChartType: "bar",
				Labels:          c.marshalLabels(),
				Datasets: []cjsDataset{{
					Fill:            true,
					SimpleData:      c.marshalSimpleData(0),
					BackgroundColor: colorFirstN(c.data.ColorType, len(c.data.FSS)),
				}},
			}
		}
		ds := []cjsDataset{}
		for i := range c.data.FSS[0] {
			ds = append(ds, cjsDataset{
				Fill:            true,
				Label:           fmt.Sprintf("category %v", i),
				SimpleData:      c.marshalSimpleData(i),
				BackgroundColor: colorRepeat(c.data.ColorType, i, len(c.data.FSS)),
			})
		}
		return cjsData{
			ChartType:       "bar",
			ActualChartType: "bar",
			Labels:          c.marshalLabels(),
			Datasets:        ds,
		}
	case "line":
		ds := []cjsDataset{}
		for i := range c.data.FSS[0] {
			ds = append(ds, cjsDataset{
				Fill:            false,
				Label:           fmt.Sprintf("category %v", i),
				SimpleData:      c.marshalSimpleData(i),
				BorderColor:     colorIndex(c.data.ColorType, i),
				BackgroundColor: colorIndex(c.data.ColorType, i),
			})
		}
		return cjsData{
			ChartType:       "line",
			ActualChartType: "line",
			Labels:          c.marshalLabels(),
			Datasets:        ds,
		}
	case "scatterline":
		dss := []cjsDataset{}
	outerLoop:
		for n := range c.data.FSS[0] {
			ds := []cjsDataPoint{}
			for i := range c.data.FSS {
				d := cjsDataPoint{}
				if c.data.hasTimes() {
					usesTimeScale = true
					d.X = "'" + c.data.TSS[i][0].Format("2006-01-02T15:04:05.999999999") + "'"
					d.Y = fmt.Sprintf("%g", c.data.FSS[i][n])
				} else {
					if n == len(c.data.FSS[0])-1 {
						break outerLoop
					}
					d.X = fmt.Sprintf("%g", c.data.FSS[i][0])
					d.Y = fmt.Sprintf("%g", c.data.FSS[i][n+1])
				}
				ds = append(ds, d)
			}
			dss = append(dss, cjsDataset{
				Fill:            false,
				Label:           fmt.Sprintf("category %v", n),
				ComplexData:     ds,
				BorderColor:     colorIndex(c.data.ColorType, n),
				BackgroundColor: colorIndex(c.data.ColorType, n),
			})
		}
		return cjsData{
			ChartType:       "line",
			ActualChartType: "scatterline",
			Datasets:        dss,
			UsesTimeScale:   usesTimeScale,
		}
	case "denormalised-scatterline":
		mdss := map[string]cjsDataset{}
		for i := range c.data.FSS {
			d := cjsDataPoint{}
			if c.data.hasTimes() {
				usesTimeScale = true
				d.X = "'" + c.data.TSS[i][0].Format("2006-01-02T15:04:05.999999999") + "'"
				d.Y = fmt.Sprintf("%g", c.data.FSS[i][0])
			} else {
				d.X = fmt.Sprintf("%g", c.data.FSS[i][0])
				d.Y = fmt.Sprintf("%g", c.data.FSS[i][1])
			}
			ds := c.data.SSS[i][0]
			if _, ok := mdss[ds]; !ok {
				mdss[ds] = cjsDataset{
					Fill:            false,
					Label:           ds,
					ComplexData:     []cjsDataPoint{d},
					BorderColor:     colorIndex(c.data.ColorType, len(mdss)),
					BackgroundColor: colorIndex(c.data.ColorType, len(mdss)),
				}
			} else {
				m := mdss[ds]
				m.ComplexData = append(m.ComplexData, d)
				mdss[ds] = m
			}
		}

		var (
			dss = make([]cjsDataset, len(mdss))
			i   = 0
		)
		for _, v := range mdss {
			dss[i] = v
			i++
		}
		sort.Slice(dss, func(i, j int) bool { // https://github.com/marianogappa/chart/issues/33
			return dss[i].Label < dss[j].Label
		})
		return cjsData{
			ChartType:       "line",
			ActualChartType: "scatterline",
			Datasets:        dss,
			UsesTimeScale:   usesTimeScale,
		}
	case "scatter":
		css := map[string]int{}
		ils := map[int]string{}
		i := 0
		for _, ss := range c.data.SSS {
			if len(ss) == 0 {
				break
			}
			if _, ok := css[ss[0]]; !ok {
				css[ss[0]] = i
				ils[i] = ss[0]
				i++
			}
		}
		dss := make([]cjsDataset, i)
		if i == 0 {
			dss = append(dss, cjsDataset{
				Fill:            true,
				Label:           "category 0",
				ComplexData:     []cjsDataPoint{},
				BackgroundColor: colorIndex(c.data.ColorType, 0),
				BorderColor:     colorIndex(c.data.ColorType, 0),
			})
		}
		for j := 0; j < i; j++ {
			dss[j] = cjsDataset{
				Fill:            true,
				Label:           ils[j],
				ComplexData:     []cjsDataPoint{},
				BackgroundColor: colorIndex(c.data.ColorType, j),
				BorderColor:     colorIndex(c.data.ColorType, j),
			}
		}

		for i := range c.data.FSS {
			d := cjsDataPoint{UsesR: true}
			if c.data.hasTimes() {
				usesTimeScale = true
				d.X = "'" + c.data.TSS[i][0].Format("2006-01-02T15:04:05.999999999") + "'"
				d.Y = fmt.Sprintf("%g", c.data.FSS[i][0])
				if len(c.data.FSS[i]) >= 2 {
					d.R = fmt.Sprintf("%v", scatterRadius(c.data.FSS[i][1], c.data.MinFSS[1], c.data.MaxFSS[1]))
				} else {
					d.R = fmt.Sprintf("%v", 4)
				}
			} else {
				d.X = fmt.Sprintf("%g", c.data.FSS[i][0])
				d.Y = "0"
				if len(c.data.FSS[i]) >= 2 {
					d.Y = fmt.Sprintf("%g", c.data.FSS[i][1])
				}
				if len(c.data.FSS[i]) >= 3 {
					d.R = fmt.Sprintf("%v", scatterRadius(c.data.FSS[i][2], c.data.MinFSS[2], c.data.MaxFSS[2]))
				} else {
					d.R = fmt.Sprintf("%v", 4)
				}
			}
			j := 0
			if c.data.hasStrings() {
				j = css[c.data.SSS[i][0]]
			}
			cd := dss[j].ComplexData
			cd = append(cd, d)
			dss[j].ComplexData = cd
		}
		return cjsData{
			ChartType:       "bubble",
			ActualChartType: "scatter",
			Datasets:        dss,
			UsesTimeScale:   usesTimeScale,
		}
	default:
		log.Fatalf("Unknown chart type: %v", c.data.ChartType)
		return cjsData{}
	}
}

func (c ChartJS) marshalLabels() string {
	if !c.data.hasStrings() && c.data.hasTimes() {
		ls := make([]string, len(c.data.TSS))
		for i, ts := range c.data.TSS {
			ls[i] = ts[0].Format("2006-01-02T15:04:05.999999999")
		}
		return "`" + strings.Join(ls, "`,`") + "`"
	}

	if !c.data.hasStrings() {
		ls := make([]string, len(c.data.FSS))
		for i := range c.data.FSS {
			ls[i] = fmt.Sprintf("slice %v", i)
		}
		return strings.Join(ls, ",")
	}

	ls := make([]string, len(c.data.SSS))
	for i, l := range c.data.SSS {
		ls[i] = preprocessLabel(l[0])
	}
	return strings.Join(ls, ",")
}

func (c ChartJS) marshalSimpleData(col int) []string {
	ds := make([]string, len(c.data.FSS))
	for i, f := range c.data.FSS {
		ds[i] = fmt.Sprintf("%g", f[col])
	}
	return ds
}

func (c ChartJS) tooltipCallback() string {
	switch c.data.ChartType {
	case "pie":
		return `
                    var value = data.datasets[0].data[tti.index];
                    var total = data.datasets[0].data.reduce((a, b) => a + b, 0)
                    var label = data.labels[tti.index];
                    var percentage = Math.round(value / total * 100);
                    return label + ': ' + percentage + '%';
    `
	case "line", "scatterline":
		return `
                    var value = data.datasets[tti.datasetIndex].data[tti.index];
                    if (value.y) {
                        value = value.y
                    }
                    return value;
    `
	case "scatter":
		return `
                    var value = data.datasets[tti.datasetIndex].data[tti.index];
                    var label = data.datasets[tti.datasetIndex].label;
                    return (label ? label + ': ' : '') + '(' + value.x + ', ' + value.y + ')';
    `
	case "bar":
		return `
                    var value = data.datasets[tti.datasetIndex].data[tti.index];
                    var label = data.labels[tti.index];
                    return value;
    `
	default:
		return ``
	}
}

func scatterRadius(x, min, max float64) float64 {
	if max-min < 50 {
		return x - min + 4
	}
	return float64(4) + (x-min)/(max-min)*50
}

func cropLongLabels(s string) string {
	if len(s) > 50 {
		return s[:47] + "..."
	}
	return s
}

func preprocessLabel(s string) string {
	if s == "" {
		return ""
	}
	s = cropLongLabels(s)
	if string(s[len(s)-1]) == "\\" {
		s += `\`
	}
	s = strings.Replace(s, `${`, `\${`, -1)
	s = strings.Replace(s, "`", "\\`", -1)
	return "`" + s + "`"
}
