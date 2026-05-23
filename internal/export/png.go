package export

import (
	"fmt"
	"image/color"
	"io"
	"strings"

	"github.com/fogleman/gg"
	"github.com/t0mer/dnsmon/internal/dnsclient"
)

const (
	pngWidth      = 1200.0
	pngHeight     = 800.0
	mapHeight     = 500.0
	tableStartY   = mapHeight + 20
	tableRowH     = 18.0
	tableFontSize = 11.0
)

// PNG renders a check's world-map and results table as PNG to w.
func PNG(w io.Writer, check *dnsclient.Check) error {
	dc := gg.NewContext(pngWidth, int(pngHeight))

	// Dark background
	dc.SetColor(color.RGBA{R: 15, G: 15, B: 35, A: 255})
	dc.Clear()

	drawMap(dc, check)
	drawTable(dc, check)

	return dc.EncodePNG(w)
}

func drawMap(dc *gg.Context, check *dnsclient.Check) {
	// Map background
	dc.SetColor(color.RGBA{R: 20, G: 30, B: 60, A: 255})
	dc.DrawRectangle(0, 0, pngWidth, mapHeight)
	dc.Fill()

	// Simple continent rectangles
	dc.SetColor(color.RGBA{R: 60, G: 80, B: 100, A: 200})
	// North America
	dc.DrawRoundedRectangle(70, 70, 200, 170, 5)
	dc.Fill()
	// South America
	dc.DrawRoundedRectangle(150, 260, 110, 160, 5)
	dc.Fill()
	// Europe
	dc.DrawRoundedRectangle(490, 60, 120, 120, 5)
	dc.Fill()
	// Africa
	dc.DrawRoundedRectangle(500, 200, 130, 180, 5)
	dc.Fill()
	// Asia
	dc.DrawRoundedRectangle(630, 55, 330, 220, 5)
	dc.Fill()
	// Australia
	dc.DrawRoundedRectangle(840, 330, 140, 100, 5)
	dc.Fill()

	// Title
	dc.SetColor(color.RGBA{R: 220, G: 230, B: 240, A: 255})
	dc.DrawStringAnchored(fmt.Sprintf("DNS Propagation: %s %s", check.Name, check.Type), 20, 22, 0, 0.5)

	// Consensus
	consensusAnswer := ""
	maxCount := 0
	for ans, count := range check.Summary.Consensus {
		if count > maxCount {
			maxCount = count
			consensusAnswer = ans
		}
	}

	// Resolver markers
	for _, r := range check.Results {
		x, y := pngMercatorProject(r.Resolver.Lat, r.Resolver.Lng)
		c := pngMarkerColor(r, consensusAnswer)
		dc.SetColor(c)
		dc.DrawCircle(x, y, 5)
		dc.Fill()

		// White border
		dc.SetColor(color.RGBA{R: 255, G: 255, B: 255, A: 100})
		dc.DrawCircle(x, y, 5)
		dc.SetLineWidth(0.5)
		dc.Stroke()
	}

	// Legend
	legendY := mapHeight - 25
	drawLegendItem(dc, 20, legendY, color.RGBA{R: 72, G: 187, B: 120, A: 255}, "OK (consensus)")
	drawLegendItem(dc, 200, legendY, color.RGBA{R: 237, G: 137, B: 54, A: 255}, "Different answer")
	drawLegendItem(dc, 380, legendY, color.RGBA{R: 252, G: 129, B: 129, A: 255}, "Error / Timeout / NXDOMAIN")
}

func drawLegendItem(dc *gg.Context, x, y float64, c color.RGBA, label string) {
	dc.SetColor(c)
	dc.DrawCircle(x, y, 5)
	dc.Fill()
	dc.SetColor(color.RGBA{R: 180, G: 190, B: 200, A: 255})
	dc.DrawStringAnchored(label, x+12, y, 0, 0.5)
}

func drawTable(dc *gg.Context, check *dnsclient.Check) {
	// Table header background
	dc.SetColor(color.RGBA{R: 30, G: 40, B: 80, A: 255})
	dc.DrawRectangle(0, tableStartY, pngWidth, tableRowH)
	dc.Fill()

	// Header text
	dc.SetColor(color.RGBA{R: 220, G: 230, B: 240, A: 255})
	headers := []string{"Resolver", "IP", "Country", "Status", "Answer", "TTL", "ms"}
	colX := []float64{10, 230, 340, 400, 480, 820, 900}

	for i, h := range headers {
		dc.DrawStringAnchored(h, colX[i], tableStartY+tableRowH/2, 0, 0.5)
	}

	// Rows
	availH := pngHeight - tableStartY - tableRowH
	maxRows := int(availH / tableRowH)
	if maxRows > len(check.Results) {
		maxRows = len(check.Results)
	}

	for i := 0; i < maxRows; i++ {
		r := check.Results[i]
		rowY := tableStartY + tableRowH*float64(i+1)

		// Alternating row background
		if i%2 == 0 {
			dc.SetColor(color.RGBA{R: 20, G: 25, B: 50, A: 255})
		} else {
			dc.SetColor(color.RGBA{R: 25, G: 35, B: 65, A: 255})
		}
		dc.DrawRectangle(0, rowY, pngWidth, tableRowH)
		dc.Fill()

		ans := ""
		ttl := ""
		if len(r.Answers) > 0 {
			ans = r.Answers[0].Value
			if len(ans) > 35 {
				ans = ans[:32] + "..."
			}
			ttl = fmt.Sprintf("%d", r.Answers[0].TTL)
		}

		statusColor := pngStatusColor(r.Status)
		textColor := color.RGBA{R: 200, G: 210, B: 220, A: 255}

		name := r.Resolver.Name
		if len(name) > 22 {
			name = name[:19] + "..."
		}

		cells := []struct {
			text  string
			color color.RGBA
		}{
			{name, textColor},
			{r.Resolver.IP, textColor},
			{r.Resolver.Country, textColor},
			{r.Status, statusColor},
			{ans, textColor},
			{ttl, textColor},
			{fmt.Sprintf("%d", r.DurationMS), textColor},
		}

		for j, cell := range cells {
			dc.SetColor(cell.color)
			dc.DrawStringAnchored(cell.text, colX[j], rowY+tableRowH/2, 0, 0.5)
		}
	}

	if len(check.Results) > maxRows {
		dc.SetColor(color.RGBA{R: 150, G: 160, B: 180, A: 255})
		dc.DrawStringAnchored(
			fmt.Sprintf("... and %d more results", len(check.Results)-maxRows),
			10, tableStartY+tableRowH*float64(maxRows+1), 0, 0.5,
		)
	}
}

func pngMercatorProject(lat, lng float64) (float64, float64) {
	const margin = 20.0
	x := margin + (lng+180.0)/360.0*(pngWidth-2*margin)

	latRad := lat * 3.14159265358979 / 180.0
	mercN := 3.14159265358979/4 + latRad/2
	sinVal := sinApprox(mercN)
	cosVal := cosApprox(mercN)
	var tanVal float64
	if cosVal != 0 {
		tanVal = sinVal / cosVal
	}
	var logVal float64
	if tanVal > 0 {
		logVal = logApprox(tanVal)
	} else if tanVal < 0 {
		logVal = -logApprox(-tanVal)
	}

	y := (mapHeight / 2) - (mapHeight*0.3*logVal/3.14159265358979)
	if y < margin {
		y = margin
	}
	if y > mapHeight-margin {
		y = mapHeight - margin
	}
	return x, y
}

func pngMarkerColor(r dnsclient.ResolverResult, consensusAnswer string) color.RGBA {
	switch r.Status {
	case dnsclient.StatusOK:
		ans := ""
		if len(r.Answers) > 0 {
			ans = r.Answers[0].Value
		}
		if consensusAnswer != "" && ans == consensusAnswer {
			return color.RGBA{R: 72, G: 187, B: 120, A: 255} // green
		}
		return color.RGBA{R: 237, G: 137, B: 54, A: 255} // orange
	default:
		return color.RGBA{R: 252, G: 129, B: 129, A: 255} // red
	}
}

func pngStatusColor(status string) color.RGBA {
	switch status {
	case dnsclient.StatusOK:
		return color.RGBA{R: 72, G: 187, B: 120, A: 255}
	case dnsclient.StatusNXDomain:
		return color.RGBA{R: 252, G: 129, B: 129, A: 255}
	case dnsclient.StatusTimeout:
		return color.RGBA{R: 237, G: 137, B: 54, A: 255}
	default:
		return color.RGBA{R: 252, G: 129, B: 129, A: 255}
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-3] + "..."
}

var _ = strings.TrimSpace // keep import
var _ = truncate
