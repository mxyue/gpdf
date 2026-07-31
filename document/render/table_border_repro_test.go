package render_test

import (
	"bytes"
	"strings"
	"testing"

	"github.com/gpdf-dev/gpdf/document"
	"github.com/gpdf-dev/gpdf/pdf"
	"github.com/gpdf-dev/gpdf/template"
)

func TestTableBorderRendersOuterFrameAfterCells(t *testing.T) {
	doc := template.New(
		template.WithPageSize(document.Size{Width: 220, Height: 180}),
		template.WithMargins(document.UniformEdges(document.Pt(20))),
		template.WithWriterSetup(func(writer *pdf.Writer) { writer.SetCompression(false) }),
	)
	pageBuilder := doc.AddPage()
	pageBuilder.AutoRow(func(row *template.RowBuilder) {
		row.Col(12, func(col *template.ColBuilder) {
			col.Table(
				[]string{"A", "B"},
				[][]string{{"C", "D"}},
				template.ColumnWidths(50, 50),
				template.WithTableBorder(template.Border(template.BorderWidth(document.Pt(0.8)), template.BorderColor(pdf.RGBHex(0x008080)))),
				template.WithTableCellBorder(template.Border(template.BorderWidth(document.Pt(0.4)), template.BorderColor(pdf.RGBHex(0xB0D0D0)))),
				template.WithTableBorderCollapse(true),
			)
		})
	})
	data, err := doc.Generate()
	if err != nil {
		t.Fatal(err)
	}
	reader, err := pdf.NewReader(data)
	if err != nil {
		t.Fatal(err)
	}
	pageInfo, err := reader.Page(0)
	if err != nil {
		t.Fatal(err)
	}
	pageObj, err := reader.GetObject(pageInfo.Ref.Number)
	if err != nil {
		t.Fatal(err)
	}
	pageDict, ok := pageObj.(pdf.Dict)
	if !ok {
		t.Fatalf("page object type = %T, want pdf.Dict", pageObj)
	}
	contentRef, ok := pageDict[pdf.Name("Contents")].(pdf.ObjectRef)
	if !ok {
		t.Fatal("page contents should be a single object reference")
	}
	contentObj, err := reader.GetObject(contentRef.Number)
	if err != nil {
		t.Fatal(err)
	}
	stream, ok := contentObj.(pdf.Stream)
	if !ok {
		t.Fatalf("content object type = %T, want pdf.Stream", contentObj)
	}
	content := string(stream.Content)
	outerBottom := "20 128 180 0.8 re"
	cellBottom := "110 128.79999999999998 89.2 0.4 re"
	if !strings.Contains(content, outerBottom) || !strings.Contains(content, cellBottom) {
		t.Fatalf("expected table and cell bottom borders in content stream: %q", content)
	}
	if strings.LastIndex(content, outerBottom) < strings.LastIndex(content, cellBottom) {
		t.Fatal("table outer bottom border must be rendered after the bottom-right cell border")
	}
	if bytes.Contains(data, []byte("/FlateDecode")) {
		t.Fatal("test PDF should be uncompressed for content-order assertions")
	}
}
