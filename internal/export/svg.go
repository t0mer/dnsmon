package export

import (
	"fmt"
	"io"
	"strings"

	"github.com/t0mer/dnsmon/internal/dnsclient"
)

const (
	svgWidth  = 1000.0
	svgHeight = 500.0
)

// SVG writes a world-map SVG with resolver markers to w.
// Markers are colored green (ok/consensus), orange (different answer), red (error/nxdomain/timeout).
func SVG(w io.Writer, check *dnsclient.Check) error {
	var b strings.Builder

	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteString(fmt.Sprintf(
		`<svg xmlns="http://www.w3.org/2000/svg" width="%d" height="%d" viewBox="0 0 %d %d">`,
		int(svgWidth), int(svgHeight)+80, int(svgWidth), int(svgHeight)+80,
	))

	// Background
	b.WriteString(fmt.Sprintf(
		`<rect width="%d" height="%d" fill="#1a1a2e"/>`,
		int(svgWidth), int(svgHeight)+80,
	))

	// Simple world map outline (approximate continent rectangles)
	b.WriteString(`<g opacity="0.3" fill="#4a5568" stroke="#718096" stroke-width="0.5">`)
	// North America
	b.WriteString(`<rect x="60" y="60" width="180" height="160" rx="4"/>`)
	// South America
	b.WriteString(`<rect x="130" y="240" width="100" height="150" rx="4"/>`)
	// Europe
	b.WriteString(`<rect x="420" y="55" width="100" height="110" rx="4"/>`)
	// Africa
	b.WriteString(`<rect x="430" y="185" width="110" height="170" rx="4"/>`)
	// Asia
	b.WriteString(`<rect x="530" y="50" width="280" height="200" rx="4"/>`)
	// Australia
	b.WriteString(`<rect x="720" y="310" width="120" height="90" rx="4"/>`)
	b.WriteString(`</g>`)

	// Determine consensus answer
	consensusAnswer := ""
	maxCount := 0
	for ans, count := range check.Summary.Consensus {
		if count > maxCount {
			maxCount = count
			consensusAnswer = ans
		}
	}

	// Draw resolver markers
	for _, r := range check.Results {
		x, y := mercatorProject(r.Resolver.Lat, r.Resolver.Lng)
		color := markerColor(r, consensusAnswer)

		ans := ""
		if len(r.Answers) > 0 {
			ans = r.Answers[0].Value
		}
		title := fmt.Sprintf("%s (%s) - %s: %s", r.Resolver.Name, r.Resolver.Country, r.Status, ans)

		b.WriteString(fmt.Sprintf(
			`<circle cx="%.1f" cy="%.1f" r="5" fill="%s" stroke="#fff" stroke-width="0.5" opacity="0.9"><title>%s</title></circle>`,
			x, y, color, escapeXML(title),
		))
	}

	// Title
	b.WriteString(fmt.Sprintf(
		`<text x="10" y="20" fill="#e2e8f0" font-family="sans-serif" font-size="14" font-weight="bold">dnsmon: %s %s</text>`,
		escapeXML(check.Name), escapeXML(check.Type),
	))

	// Legend
	legendY := int(svgHeight) + 20
	b.WriteString(fmt.Sprintf(
		`<circle cx="20" cy="%d" r="5" fill="#48bb78"/><text x="30" y="%d" fill="#a0aec0" font-family="sans-serif" font-size="11">OK (consensus)</text>`,
		legendY, legendY+4,
	))
	b.WriteString(fmt.Sprintf(
		`<circle cx="170" cy="%d" r="5" fill="#ed8936"/><text x="180" y="%d" fill="#a0aec0" font-family="sans-serif" font-size="11">Different answer</text>`,
		legendY, legendY+4,
	))
	b.WriteString(fmt.Sprintf(
		`<circle cx="330" cy="%d" r="5" fill="#fc8181"/><text x="340" y="%d" fill="#a0aec0" font-family="sans-serif" font-size="11">Error / Timeout / NXDOMAIN</text>`,
		legendY, legendY+4,
	))

	b.WriteString(`</svg>`)

	_, err := io.WriteString(w, b.String())
	return err
}

// mercatorProject converts lat/lng to SVG x/y coordinates using a simple Mercator projection.
func mercatorProject(lat, lng float64) (float64, float64) {
	x := (lng + 180.0) / 360.0 * svgWidth
	latRad := lat * 3.14159265358979 / 180.0
	mercN := 3.14159265358979/4 + latRad/2
	y := (svgHeight / 2) - (svgHeight * 0.3 * (func(x float64) float64 {
		if x > 0 {
			return logApprox(x)
		}
		return -logApprox(-x)
	})(tanApprox(mercN)) / 3.14159265358979)
	return x, y
}

// tanApprox approximates tan without importing math.
func tanApprox(x float64) float64 {
	s := sinApprox(x)
	c := cosApprox(x)
	if c == 0 {
		return 0
	}
	return s / c
}

func sinApprox(x float64) float64 {
	// Normalize x to [-π, π]
	for x > 3.14159265358979 {
		x -= 2 * 3.14159265358979
	}
	for x < -3.14159265358979 {
		x += 2 * 3.14159265358979
	}
	// Taylor series approximation
	x2 := x * x
	return x * (1 - x2/6*(1-x2/20*(1-x2/42)))
}

func cosApprox(x float64) float64 {
	return sinApprox(x + 3.14159265358979/2)
}

func logApprox(x float64) float64 {
	if x <= 0 {
		return 0
	}
	// ln(x) approximation using ln(x) = 2*atanh((x-1)/(x+1))
	y := (x - 1) / (x + 1)
	y2 := y * y
	return 2 * y * (1 + y2/3 + y2*y2/5 + y2*y2*y2/7)
}

func markerColor(r dnsclient.ResolverResult, consensusAnswer string) string {
	switch r.Status {
	case dnsclient.StatusOK:
		ans := ""
		if len(r.Answers) > 0 {
			ans = r.Answers[0].Value
		}
		if consensusAnswer != "" && ans == consensusAnswer {
			return "#48bb78" // green
		}
		return "#ed8936" // orange - different answer
	default:
		return "#fc8181" // red
	}
}

func escapeXML(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	return s
}
