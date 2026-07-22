# 圆角边框设计

## 目标

为现有模板边框 API 增加可选圆角能力，覆盖表格外框、图片、Box 和文本段落。半径为 0 时保持现有直角边框和 PDF 输出行为。

## API

在 `template.BorderSpec` 上增加 `BorderRadius(document.Value)` 选项。示例：

```go
template.Border(
	template.BorderWidth(document.Pt(1)),
	template.BorderColor(pdf.RGBHex(0x1A237E)),
	template.BorderRadius(document.Mm(3)),
)
```

该选项通过 `document.BorderEdges` 传递到渲染层，现有 `WithTableBorder`、`WithImageBorder`、`WithBoxBorder` 和 `WithTextBorder` 自动支持。未设置时半径为零。

## 渲染行为

- 使用 PDF 路径和四段三次贝塞尔曲线绘制圆角矩形。
- 半径按布局后的宽高限制为不超过短边的一半。
- 有背景色时，背景填充使用同一圆角轮廓。
- 模板生成的统一边框使用圆角路径；直接构造四边不同宽度、颜色或样式的边框时保留现有直角逐边绘制逻辑。
- 半径为零时继续使用原有 `re` 矩形路径，降低兼容风险。

## 测试

- 渲染层验证圆角路径包含 `c` 曲线指令，并验证半径限制。
- 模板层验证 `BorderRadius` 能写入边框配置。
- 运行 `go test ./...`，确认现有边框和 golden 测试不回归。
