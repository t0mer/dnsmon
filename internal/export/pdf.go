package export

import (
	"fmt"
	"io"

	"github.com/jung-kurt/gofpdf"
	"github.com/t0mer/dnsmon/internal/dnsclient"
)

// PDF writes a Check's results as PDF to w.
func PDF(w io.Writer, check *dnsclient.Check) error {
	pdf := gofpdf.New("L", "mm", "A4", "")
	pdf.AddPage()
	pdf.SetFont("Helvetica", "B", 16)

	// Title
	pdf.CellFormat(0, 10, "dnsmon DNS Propagation Report", "", 1, "C", false, 0, "")
	pdf.Ln(4)

	// Metadata
	pdf.SetFont("Helvetica", "", 11)
	pdf.CellFormat(40, 7, "Domain:", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 7, check.Name, "", 1, "L", false, 0, "")

	pdf.CellFormat(40, 7, "Record Type:", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 7, check.Type, "", 1, "L", false, 0, "")

	pdf.CellFormat(40, 7, "Check ID:", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 7, check.ID, "", 1, "L", false, 0, "")

	pdf.CellFormat(40, 7, "Date:", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 7, check.CreatedAt.UTC().Format("2006-01-02 15:04:05 UTC"), "", 1, "L", false, 0, "")

	pdf.CellFormat(40, 7, "Resolvers:", "", 0, "L", false, 0, "")
	pdf.CellFormat(0, 7, fmt.Sprintf("%d total, %d responded, %d timeouts, %d errors",
		check.Summary.TotalResolvers, check.Summary.Responded, check.Summary.Timeouts, check.Summary.Errors), "", 1, "L", false, 0, "")

	pdf.Ln(6)

	// Table header
	pdf.SetFont("Helvetica", "B", 9)
	pdf.SetFillColor(50, 50, 80)
	pdf.SetTextColor(255, 255, 255)

	colWidths := []float64{50, 30, 20, 20, 90, 25}
	headers := []string{"Resolver", "IP", "Country", "Status", "Answer", "Duration (ms)"}
	for i, h := range headers {
		pdf.CellFormat(colWidths[i], 7, h, "1", 0, "C", true, 0, "")
	}
	pdf.Ln(-1)

	// Table rows
	pdf.SetFont("Helvetica", "", 8)
	pdf.SetTextColor(0, 0, 0)

	fill := false
	for _, r := range check.Results {
		pdf.SetFillColor(240, 240, 255)
		if !fill {
			pdf.SetFillColor(255, 255, 255)
		}
		fill = !fill

		ans := ""
		if len(r.Answers) > 0 {
			ans = r.Answers[0].Value
			if len(ans) > 40 {
				ans = ans[:37] + "..."
			}
		}

		name := r.Resolver.Name
		if len(name) > 22 {
			name = name[:19] + "..."
		}

		cells := []string{
			name,
			r.Resolver.IP,
			r.Resolver.Country,
			r.Status,
			ans,
			fmt.Sprintf("%d", r.DurationMS),
		}
		for i, cell := range cells {
			pdf.CellFormat(colWidths[i], 6, cell, "1", 0, "L", fill, 0, "")
		}
		pdf.Ln(-1)
	}

	return pdf.Output(w)
}
