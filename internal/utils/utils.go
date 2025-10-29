package utils

import (
	"github.com/gomarkdown/markdown"
	"github.com/gomarkdown/markdown/html"
	"github.com/gomarkdown/markdown/parser"
	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"
	//"golang.org/x/sync/errgroup"
)

func MDtoHTML(md []byte) []byte {
	extensions := parser.CommonExtensions | parser.AutoHeadingIDs | parser.NoEmptyLineBeforeBlock
	p := parser.NewWithExtensions(extensions)
	doc := p.Parse(md)
	htmlFlags := html.CommonFlags | html.HrefTargetBlank
	opts := html.RendererOptions{Flags: htmlFlags}
	renderer := html.NewRenderer(opts)
	return markdown.Render(doc, renderer)
}

func streamMDtoHTML(c chan []byte, astStream chan ast.Node) error {
	streamContent := []byte{}
	defer close(astStream)
	var lastNode ast.Node
	var lastStart int
	var lastStop int
	chunkCount := 0

	for chunk := range c {
		chunkCount++
		streamContent = append(streamContent, chunk...)
		document := goldmark.DefaultParser().Parse(text.NewReader(streamContent),)
		err := ast.Walk(document, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
			if entering {
				nodeType := ""
				var segments *text.Segments
				switch v := n.(type) {
				case *ast.Heading:
					nodeType = "Heading"
					segments = v.Lines()
				case *ast.FencedCodeBlock:
					nodeType = "Fenced Code block"
					segments = v.Lines()
				case *ast.CodeBlock:
					nodeType = "Code block"
					segments = v.Lines()
				case *ast.Paragraph:
					nodeType = "Paragraph"
					segments = v.Lines()
				case *ast.List:
					nodeType = "List"
					segments = v.Lines()
				case *ast.ListItem:
					nodeType = "List item"
					cur := v.FirstChild()
					segments = &text.Segments{}
					for cur != nil {
						lines_ := cur.Lines()
						for i := 0; i < lines_.Len(); i++ {
							segments.Append(lines_.At(i))
						}
						cur = cur.NextSibling()
					}
				case *ast.Blockquote:
					nodeType = "Blockquote"
					segments = v.Lines()
				default:
					return ast.WalkContinue, nil
				}
				_ = nodeType
				if segments == nil {
					return ast.WalkContinue, nil
				}
				if segments.Len() == 0 {
					return ast.WalkContinue, nil
				}
				firstSegment := segments.At(0)
				lastSegment := segments.At(segments.Len() - 1)
				end := firstSegment.Start + 40
				if len(streamContent) < end {
					end = len(streamContent)
				}
				if end > lastSegment.Stop {
					end = lastSegment.Stop
				}
				if lastNode == nil {
					lastNode = n
					lastStart = firstSegment.Start
					lastStop = lastSegment.Stop
					return ast.WalkContinue, nil
				}
				if lastSegment.Stop <= lastStop {
					return ast.WalkSkipChildren, nil
				}
				if firstSegment.Start == lastStart {
					lastNode = n
					lastStart = firstSegment.Start
					lastStop = lastSegment.Stop
					return ast.WalkContinue, nil
				}
				if firstSegment.Start >= lastStop {
					astStream <- lastNode
					lastNode = n
					lastStart = firstSegment.Start
					lastStop = lastSegment.Stop
					return ast.WalkContinue, nil
				}
			}
			return ast.WalkContinue, nil
		})
		if err != nil {
			return err
		}
	}
	if lastNode != nil {
		astStream <- lastNode
	}
	return nil

}
