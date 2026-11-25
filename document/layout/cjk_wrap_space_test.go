package layout

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"github.com/gpdf-dev/gpdf/document"
	"github.com/gpdf-dev/gpdf/pdf/font"
)

// cjkTestFontPath 指向主项目内置的报告字体；缺失时跳过用例。
const cjkTestFontPath = "/Volumes/space/work/mainbo/xinli-ai/gether/xlai-api/deploy/resource/AlibabaPuHuiTi-3-55-Regular.ttf"

// cjkParagraph 取自真实测评报告段落：首行缩进 + 中英数混排，断行点既可能
// 落在汉字之间，也可能落在汉字与数字/百分号之间。
const cjkParagraph = "如表7所示，在本次测评中，预警占比较高的学校分别为广汉中学钟声学校、广汉中学实验学校高新分校、四川省广汉中学。广汉中学钟声学校有生活满意度问题倾向的人数有378人，占有效测评人数的15.68%；广汉中学实验学校高新分校有生活满意度问题倾向的人数有104人，占有效测评人数的15.64%；该群体在生活满意度方面可能存在一定风险，建议结合学生日常表现进一步评估确认，并给予针对性的关注与指导。"

type ttfFontResolver struct {
	ttf font.Font
}

func (r *ttfFontResolver) Resolve(_ string, _ document.FontWeight, _ bool) ResolvedFont {
	return ResolvedFont{ID: "test"}
}

func (r *ttfFontResolver) MeasureString(_ ResolvedFont, text string, size float64) float64 {
	return font.MeasureString(r.ttf, text, size)
}

func (r *ttfFontResolver) LineBreak(_ ResolvedFont, text string, size, maxWidth float64) []string {
	return font.LineBreak(r.ttf, text, size, maxWidth)
}

func cjkResolver(t *testing.T) *ttfFontResolver {
	t.Helper()
	data, err := os.ReadFile(cjkTestFontPath)
	if err != nil {
		t.Skipf("report font not available: %v", err)
	}
	ttf, err := font.ParseTrueType(data)
	if err != nil {
		t.Fatalf("parse font: %v", err)
	}
	return &ttfFontResolver{ttf: ttf}
}

// cjkSpacePattern 匹配两个中日韩文字或标点之间夹着的空格。
var cjkSpacePattern = regexp.MustCompile(`[\p{Han}，。；：、）】] [\p{Han}（]`)

// mmToPt 把毫米换算为 PDF 磅，供 Constraints 使用。
func mmToPt(v float64) float64 { return document.Mm(v).Resolve(0, 0) }

func cjkParagraphStyle() document.Style {
	return document.Style{
		FontSize:   11,
		LineHeight: 1.6,
		TextIndent: document.Mm(7),
		TextAlign:  document.AlignLeft,
		FontFamily: "test",
	}
}

func placedLineContents(result Result) []string {
	var lines []string
	for _, pn := range result.Children {
		if txt, ok := pn.Node.(*document.Text); ok {
			lines = append(lines, txt.Content)
		}
	}
	return lines
}

// assertExactLines 校验各行都能按顺序回溯到原文：既不能多出空格，也不能
// 丢掉原文里的空格或字符，回溯结束后剩余内容必须为空。
func assertExactLines(t *testing.T, text string, lines []string) {
	t.Helper()
	rest := text
	for i, line := range lines {
		next, ok := sourceAfterLine(rest, line)
		if !ok {
			t.Fatalf("line %d %q is not a slice of the remaining source %q", i, line, rest)
		}
		rest = next
	}
	if rest != "" {
		t.Errorf("lines do not cover the source, leftover %q", rest)
	}
}

// TestLayoutTextIndentCJKNoInsertedSpaces 中文段落带首行缩进时，换行重排
// 不得在中文字符之间引入空格（历史缺陷：joinLines 用空格拼接 CJK 断行）。
func TestLayoutTextIndentCJKNoInsertedSpaces(t *testing.T) {
	resolver := cjkResolver(t)
	constraints := Constraints{
		AvailableWidth:  mmToPt(160),
		AvailableHeight: mmToPt(200),
		FontResolver:    resolver,
	}
	result := (&FlowLayout{}).LayoutText(cjkParagraph, cjkParagraphStyle(), constraints)
	lines := placedLineContents(result)
	if len(lines) < 2 {
		t.Fatalf("expected multiple wrapped lines, got %d", len(lines))
	}
	for i, line := range lines {
		if loc := cjkSpacePattern.FindStringIndex(line); loc != nil {
			t.Errorf("line %d contains inserted space between CJK chars: %q", i, line)
		}
	}
	assertExactLines(t, cjkParagraph, lines)
}

// TestLayoutTextOverflowKeepsSourceText 跨页溢出的文本必须是原文的后缀，
// 不能靠重新拼接得到（同一缺陷也会在中文之间插入空格）。
func TestLayoutTextOverflowKeepsSourceText(t *testing.T) {
	resolver := cjkResolver(t)
	constraints := Constraints{
		AvailableWidth:  mmToPt(160),
		AvailableHeight: mmToPt(10),
		FontResolver:    resolver,
	}
	result := (&FlowLayout{}).LayoutText(cjkParagraph, cjkParagraphStyle(), constraints)
	overflow, ok := result.Overflow.(*document.Text)
	if !ok {
		t.Fatalf("expected text overflow, got %#v", result.Overflow)
	}
	if loc := cjkSpacePattern.FindStringIndex(overflow.Content); loc != nil {
		t.Errorf("overflow contains inserted space between CJK chars: %q", overflow.Content)
	}
	if !strings.HasSuffix(cjkParagraph, overflow.Content) {
		t.Errorf("overflow %q is not a suffix of the source", overflow.Content)
	}
}

// TestJoinLinesCJKNoSpace 拼接兜底路径：CJK 断行不得引入空格，拉丁词边界
// 仍要恢复空格。
func TestJoinLinesCJKNoSpace(t *testing.T) {
	tests := []struct {
		name  string
		input []string
		want  string
	}{
		{"cjk", []string{"生活满意度", "问题倾向的人数"}, "生活满意度问题倾向的人数"},
		{"cjk_punctuation", []string{"存在一定风险，", "建议结合学生"}, "存在一定风险，建议结合学生"},
		{"latin", []string{"hello", "world"}, "hello world"},
		{"already_spaced", []string{"满意度 ", "方面"}, "满意度 方面"},
		{"leading_space", []string{"满意度", " 方面"}, "满意度 方面"},
		{"empty_line", []string{"满意度", ""}, "满意度"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := joinLines(tt.input); got != tt.want {
				t.Errorf("joinLines(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestSourceAfterLine 回溯原文：断行处被吃掉的一个空格或换行不应保留，
// 其余内容原样返回。
func TestSourceAfterLine(t *testing.T) {
	tests := []struct {
		name     string
		text     string
		line     string
		wantRest string
		wantOK   bool
	}{
		{"cjk_break", "生活满意度问题倾向", "生活满意度", "问题倾向", true},
		{"word_break", "hello world again", "hello", "world again", true},
		{"newline", "第一段\n第二段", "第一段", "第二段", true},
		{"double_space_keeps_one", "满意度  方面", "满意度", " 方面", true},
		{"not_a_prefix", "满意度方面", "风险", "", false},
		{"empty_line_skips_leading_newline", "\n第二段\n第三段", "", "第二段\n第三段", true},
		{"empty_line_no_separator", "第二段", "", "第二段", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rest, ok := sourceAfterLine(tt.text, tt.line)
			if ok != tt.wantOK || rest != tt.wantRest {
				t.Errorf("sourceAfterLine(%q, %q) = (%q, %v), want (%q, %v)",
					tt.text, tt.line, rest, ok, tt.wantRest, tt.wantOK)
			}
		})
	}
}

// TestLayoutTextNewlinesRoundTrip 段内含换行时，逐行回溯原文必须无损，
// 空行对应被断行器吃掉的换行符。
func TestLayoutTextNewlinesRoundTrip(t *testing.T) {
	resolver := cjkResolver(t)
	text := "第一段内容，描述本次测评的总体风险情况与统计口径。\n\n第二段内容，给出针对性建议。\n第三段内容，说明后续跟踪安排。"
	constraints := Constraints{
		AvailableWidth:  mmToPt(60),
		AvailableHeight: mmToPt(200),
		FontResolver:    resolver,
	}
	style := cjkParagraphStyle()
	style.TextIndent = document.Value{}
	result := (&FlowLayout{}).LayoutText(text, style, constraints)
	lines := placedLineContents(result)
	if len(lines) < 3 {
		t.Fatalf("expected wrapped lines, got %d: %q", len(lines), lines)
	}
	assertExactLines(t, text, lines)
}
