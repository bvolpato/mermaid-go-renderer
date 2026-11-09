// Ported from mermaid 12.2.0, diagrams/usecase/parser/usecase.parser.ts (MIT).
//
// Each rule is a method that mirrors the rule of the Chevrotain grammar call
// for call: CONSUME is consume, SUBRULE is subrule, OPTION2 is option(2, ...)
// and so on. The occurrence index of an OR, an OPTION, a MANY and an
// AT_LEAST_ONE selects its lookahead paths in grammar_gen.go.

package usecase

import "strings"

// usecaseParser is UsecaseParser.
type usecaseParser struct {
	recognizer
}

// isLabelToken is isLabelToken(token).
func isLabelToken(tok *token) bool {
	return tokenMatcher(tok, tokLabelText) ||
		tok.typ == tokPlainString ||
		tok.typ == tokMarkdownString
}

var forbiddenPlantUmlStatements = map[string]bool{
	"allowmixing": true,
	"newpage":     true,
	"package":     true,
	"rectangle":   true,
	"skinparam":   true,
}

func (p *usecaseParser) start() *cstNode {
	return p.rule("start", func() {
		p.consume(tokUsecase)
		p.subrule(p.lineEnd)
		p.many(0, func() { p.subrule(p.line) })
	})
}

func (p *usecaseParser) line() *cstNode {
	return p.rule("line", func() {
		p.or(0, []alternative{
			{alt: func() { p.subrule(p.blankLine) }},
			{alt: func() { p.subrule(p.commentLine) }},
			{
				gate: p.isStatementStart,
				alt:  func() { p.subrule(p.statement) },
			},
		})
	})
}

func (p *usecaseParser) statement() *cstNode {
	return p.rule("statement", func() {
		p.or(0, []alternative{
			{alt: func() { p.subrule(p.accTitleStatement) }},
			{alt: func() { p.subrule(p.accDescrStatement) }},
			{alt: func() { p.subrule(p.directionStatement) }},
			{alt: func() { p.subrule(p.actorStatement) }},
			{alt: func() { p.subrule(p.systemBoundaryStatement) }},
			{alt: func() { p.subrule(p.noteStatement) }},
			{alt: func() { p.subrule(p.jsonStatement) }},
			{alt: func() { p.subrule(p.classDefStatement) }},
			{alt: func() { p.subrule(p.classStatement) }},
			{alt: func() { p.subrule(p.styleStatement) }},
			{
				gate: p.isMetadataAssignment,
				alt:  func() { p.subrule(p.metadataAssignmentStatement) },
			},
			{
				gate: func() bool { return !p.isForbiddenPlantUmlStatement() },
				alt:  func() { p.subrule(p.entityStatement) },
			},
		})
	})
}

func (p *usecaseParser) lineEnd() *cstNode {
	return p.rule("lineEnd", func() {
		p.or(0, []alternative{
			{alt: func() { p.consume(tokNewline) }},
			{alt: func() { p.consume(tokEof) }},
		})
	})
}

func (p *usecaseParser) blankLine() *cstNode {
	return p.rule("blankLine", func() {
		p.consume(tokNewline)
	})
}

func (p *usecaseParser) commentLine() *cstNode {
	return p.rule("commentLine", func() {
		p.consume(tokComment)
		p.subrule(p.lineEnd)
	})
}

func (p *usecaseParser) accTitleStatement() *cstNode {
	return p.rule("accTitleStatement", func() {
		p.consume(tokAccTitleLine)
		p.subrule(p.lineEnd)
	})
}

func (p *usecaseParser) accDescrStatement() *cstNode {
	return p.rule("accDescrStatement", func() {
		p.or(0, []alternative{
			{alt: func() { p.consume(tokAccDescrLine) }},
			{alt: func() { p.consume(tokAccDescrBlock) }},
		})
		p.subrule(p.lineEnd)
	})
}

func (p *usecaseParser) actorStatement() *cstNode {
	return p.rule("actorStatement", func() {
		p.consume(tokActor)
		p.subrule(p.actorItem)
		p.option(0, nil, func() {
			p.or(0, []alternative{
				{
					alt: func() {
						p.atLeastOne(0, func() {
							p.consume(tokComma)
							p.subrule(p.actorItem)
						})
					},
				},
				{alt: func() { p.subrule(p.relationTail) }},
			})
		})
		p.subrule(p.lineEnd)
	})
}

func (p *usecaseParser) actorItem() *cstNode {
	return p.rule("actorItem", func() {
		p.subrule(p.actorName)
		p.option(0, nil, func() { p.subrule(p.metadata) })
		p.option(2, nil, func() { p.subrule(p.stereotype) })
		p.option(3, nil, func() { p.subrule(p.classSuffix) })
	})
}

func (p *usecaseParser) actorName() *cstNode {
	return p.rule("actorName", func() {
		p.or(0, []alternative{
			{
				alt: func() {
					p.consume(tokIdentifier)
					p.option(0, nil, func() {
						p.consume(tokLparen)
						p.subrule(p.nodeLabel)
						p.consume(tokRparen)
					})
				},
			},
			{alt: func() { p.consume(tokPlainString) }},
			{alt: func() { p.consume(tokMarkdownString) }},
		})
	})
}

func (p *usecaseParser) actorDeclarationOnly() *cstNode {
	return p.rule("actorDeclarationOnly", func() {
		p.consume(tokActor)
		p.subrule(p.actorItem)
		p.many(0, func() {
			p.consume(tokComma)
			p.subrule(p.actorItem)
		})
		p.subrule(p.lineEnd)
	})
}

func (p *usecaseParser) entityStatement() *cstNode {
	return p.rule("entityStatement", func() {
		p.subrule(p.entityName)
		p.option(0, nil, func() { p.subrule(p.relationTail) })
		p.subrule(p.lineEnd)
	})
}

func (p *usecaseParser) entityName() *cstNode {
	return p.rule("entityName", func() {
		p.or(0, []alternative{
			{
				alt: func() {
					p.consume(tokIdentifier)
					p.option(0, nil, func() {
						p.or(2, []alternative{
							{
								alt: func() {
									p.consume(tokLparen)
									p.subrule(p.nodeLabel)
									p.consume(tokRparen)
								},
							},
							{
								alt: func() {
									p.consume(tokLbracket)
									p.subrule(p.nodeLabel)
									p.consume(tokRbracket)
								},
							},
						})
					})
				},
			},
			{alt: func() { p.consume(tokPlainString) }},
			{alt: func() { p.consume(tokMarkdownString) }},
		})
		p.option(2, nil, func() { p.subrule(p.useCaseMetadata) })
		p.option(3, nil, func() { p.subrule(p.stereotype) })
		p.option(4, nil, func() { p.subrule(p.classSuffix) })
	})
}

func (p *usecaseParser) nodeLabel() *cstNode {
	return p.rule("nodeLabel", func() {
		p.or(0, []alternative{
			{alt: func() { p.consume(tokPlainString) }},
			{alt: func() { p.consume(tokMarkdownString) }},
			// Unquoted labels run until a delimiter, operator, or suffix marker, none of
			// which belong to `LabelText`. The visitor rebuilds the text from the source
			// slice, so the internal token split never reaches the model.
			{alt: func() { p.atLeastOne(0, func() { p.consume(tokLabelText) }) }},
		})
	})
}

func (p *usecaseParser) useCaseMetadata() *cstNode {
	return p.rule("useCaseMetadata", func() {
		p.subrule(p.metadata)
	})
}

func (p *usecaseParser) relationTail() *cstNode {
	return p.rule("relationTail", func() {
		p.option(0,
			func() bool { return p.la(1).typ == tokIdentifier && p.la(2).typ == tokAt },
			func() {
				p.consume(tokIdentifier)
				p.consume(tokAt)
			},
		)
		p.subrule(p.arrow)
		p.subrule(p.entityName)
	})
}

func (p *usecaseParser) arrow() *cstNode {
	return p.rule("arrow", func() {
		p.or(0, []alternative{
			{alt: func() { p.subrule(p.semanticRelation) }},
			{alt: func() { p.subrule(p.forwardSolidOperator) }},
			{alt: func() { p.subrule(p.backwardSolidOperator) }},
			{alt: func() { p.subrule(p.markerlessSolidOperator) }},
			{alt: func() { p.subrule(p.forwardCircleOperator) }},
			{alt: func() { p.subrule(p.backwardCircleOperator) }},
			{alt: func() { p.subrule(p.forwardCrossOperator) }},
			{alt: func() { p.subrule(p.backwardCrossOperator) }},
		})
	})
}

func (p *usecaseParser) forwardSolidOperator() *cstNode {
	return p.rule("forwardSolidOperator", func() {
		p.consume(tokForwardSolid)
	})
}

func (p *usecaseParser) backwardSolidOperator() *cstNode {
	return p.rule("backwardSolidOperator", func() {
		p.consume(tokBackwardSolid)
		p.option(0,
			func() bool { return p.la(0).image == "<--" && p.hasLabeledRight(tokMarkerlessSolid) },
			func() {
				p.subrule(p.edgeLabel)
				p.consume(tokMarkerlessSolid)
			},
		)
	})
}

func (p *usecaseParser) markerlessSolidOperator() *cstNode {
	return p.rule("markerlessSolidOperator", func() {
		p.consume(tokMarkerlessSolid)
		p.option(0,
			func() bool {
				return p.la(0).image == "--" &&
					p.hasLabeledRight(tokForwardSolid, tokMarkerlessSolid, tokForwardCircle, tokForwardCross)
			},
			func() {
				p.subrule(p.edgeLabel)
				p.or(0, []alternative{
					{alt: func() { p.consume(tokForwardSolid) }},
					{alt: func() { p.consume(tokMarkerlessSolid) }},
					{alt: func() { p.consume(tokForwardCircle) }},
					{alt: func() { p.consume(tokForwardCross) }},
				})
			},
		)
	})
}

func (p *usecaseParser) forwardCircleOperator() *cstNode {
	return p.rule("forwardCircleOperator", func() {
		p.consume(tokForwardCircle)
	})
}

func (p *usecaseParser) backwardCircleOperator() *cstNode {
	return p.rule("backwardCircleOperator", func() {
		p.consume(tokBackwardCircle)
		p.option(0,
			func() bool { return p.hasLabeledRight(tokMarkerlessSolid) },
			func() {
				p.subrule(p.edgeLabel)
				p.consume(tokMarkerlessSolid)
			},
		)
	})
}

func (p *usecaseParser) forwardCrossOperator() *cstNode {
	return p.rule("forwardCrossOperator", func() {
		p.consume(tokForwardCross)
	})
}

func (p *usecaseParser) backwardCrossOperator() *cstNode {
	return p.rule("backwardCrossOperator", func() {
		p.consume(tokBackwardCross)
		p.option(0,
			func() bool { return p.hasLabeledRight(tokMarkerlessSolid) },
			func() {
				p.subrule(p.edgeLabel)
				p.consume(tokMarkerlessSolid)
			},
		)
	})
}

func (p *usecaseParser) edgeLabel() *cstNode {
	return p.rule("edgeLabel", func() {
		p.or(0, []alternative{
			{alt: func() { p.consume(tokPlainString) }},
			{alt: func() { p.consume(tokMarkdownString) }},
			{alt: func() { p.atLeastOne(0, func() { p.consume(tokLabelText) }) }},
		})
	})
}

func (p *usecaseParser) semanticRelation() *cstNode {
	return p.rule("semanticRelation", func() {
		p.or(0, []alternative{
			{
				alt: func() {
					p.consume(tokDependencyArrow)
					p.consume(tokColon)
					p.or(2, []alternative{
						{alt: func() { p.consume(tokInclude) }},
						{alt: func() { p.consume(tokExtend) }},
					})
				},
			},
			{alt: func() { p.consume(tokGeneralization) }},
		})
	})
}

func (p *usecaseParser) metadata() *cstNode {
	return p.rule("metadata", func() {
		p.consume(tokMetadataStart)
		p.many(0, func() { p.consume(tokNewline) })
		p.option(0, nil, func() {
			p.subrule(p.metadataProperty)
			p.many(2, func() {
				p.subrule(p.metadataSeparator)
				p.subrule(p.metadataProperty)
			})
			p.option(2, nil, func() { p.consume(tokComma) })
			p.many(3, func() { p.consume(tokNewline) })
		})
		p.consume(tokRbrace)
	})
}

func (p *usecaseParser) metadataProperty() *cstNode {
	return p.rule("metadataProperty", func() {
		p.or(0, []alternative{
			{alt: func() { p.consume(tokIdentifier) }},
			{alt: func() { p.consume(tokPlainString) }},
		})
		p.consume(tokColon)
		p.or(2, []alternative{
			{alt: func() { p.consume(tokIdentifier) }},
			{alt: func() { p.consume(tokPlainString) }},
			{alt: func() { p.consume(tokTrue) }},
			{alt: func() { p.consume(tokFalse) }},
		})
	})
}

func (p *usecaseParser) metadataSeparator() *cstNode {
	return p.rule("metadataSeparator", func() {
		p.or(0, []alternative{
			{
				alt: func() {
					p.consume(tokComma)
					p.many(0, func() { p.consume(tokNewline) })
				},
			},
			{
				alt: func() {
					p.atLeastOne(0, func() { p.consume(tokNewline) })
					p.option(0, nil, func() { p.consume(tokComma) })
					p.many(2, func() { p.consume(tokNewline) })
				},
			},
		})
	})
}

func (p *usecaseParser) systemBoundaryStatement() *cstNode {
	return p.rule("systemBoundaryStatement", func() {
		p.consume(tokSystemBoundary)
		p.subrule(p.systemBoundaryName)
		// Same suffix order as actor and use case declarations: metadata, then classes.
		p.option(0, nil, func() { p.subrule(p.metadata) })
		p.option(2, nil, func() { p.subrule(p.classSuffix) })
		p.subrule(p.lineEnd)
		p.subrule(p.systemBoundaryContent)
		p.consume(tokEnd)
		p.subrule(p.lineEnd)
	})
}

func (p *usecaseParser) systemBoundaryName() *cstNode {
	return p.rule("systemBoundaryName", func() {
		p.or(0, []alternative{
			{
				alt: func() {
					p.consume(tokIdentifier)
					// Mirrors `entityName`, so a short id can carry a long title. Unlike a use
					// case, the bracket form does not select a shape; boundary geometry comes
					// from `type` metadata only.
					p.option(0, nil, func() {
						p.or(2, []alternative{
							{
								alt: func() {
									p.consume(tokLparen)
									p.subrule(p.nodeLabel)
									p.consume(tokRparen)
								},
							},
							{
								alt: func() {
									p.consume(tokLbracket)
									p.subrule(p.nodeLabel)
									p.consume(tokRbracket)
								},
							},
						})
					})
				},
			},
			{alt: func() { p.consume(tokPlainString) }},
			{alt: func() { p.consume(tokMarkdownString) }},
		})
	})
}

func (p *usecaseParser) systemBoundaryContent() *cstNode {
	return p.rule("systemBoundaryContent", func() {
		p.many(0, func() {
			p.or(0, []alternative{
				{alt: func() { p.subrule(p.blankLine) }},
				{alt: func() { p.subrule(p.commentLine) }},
				{alt: func() { p.subrule(p.boundaryElement) }},
			})
		})
	})
}

func (p *usecaseParser) boundaryElement() *cstNode {
	return p.rule("boundaryElement", func() {
		p.or(0, []alternative{
			{alt: func() { p.subrule(p.actorDeclarationOnly) }},
			{
				alt: func() {
					p.subrule(p.entityName)
					p.subrule(p.lineEnd)
				},
			},
		})
	})
}

func (p *usecaseParser) metadataAssignmentStatement() *cstNode {
	return p.rule("metadataAssignmentStatement", func() {
		p.subrule(p.metadataAssignmentTarget)
		p.subrule(p.metadata)
		p.subrule(p.lineEnd)
	})
}

func (p *usecaseParser) metadataAssignmentTarget() *cstNode {
	return p.rule("metadataAssignmentTarget", func() {
		p.or(0, []alternative{
			{alt: func() { p.consume(tokIdentifier) }},
			{alt: func() { p.consume(tokPlainString) }},
			{alt: func() { p.consume(tokMarkdownString) }},
		})
	})
}

func (p *usecaseParser) noteStatement() *cstNode {
	return p.rule("noteStatement", func() {
		p.consume(tokNote)
		p.consume(tokFor)
		p.consume(tokIdentifier)
		p.subrule(p.nodeLabel)
		p.subrule(p.lineEnd)
	})
}

func (p *usecaseParser) stereotype() *cstNode {
	return p.rule("stereotype", func() {
		p.consume(tokStereotypeStart)
		p.consume(tokStereotypeText)
		p.consume(tokStereotypeEnd)
	})
}

func (p *usecaseParser) classSuffix() *cstNode {
	return p.rule("classSuffix", func() {
		p.consume(tokClassSeparator)
		p.consume(tokIdentifier)
		p.many(0, func() {
			p.consume(tokComma)
			p.consume(tokIdentifier)
		})
	})
}

func (p *usecaseParser) jsonStatement() *cstNode {
	return p.rule("jsonStatement", func() {
		p.consume(tokJsonDeclarationStart)
		p.consume(tokJsonObjectLiteral)
		p.option(0, nil, func() { p.subrule(p.classSuffix) })
		p.subrule(p.lineEnd)
	})
}

func (p *usecaseParser) directionStatement() *cstNode {
	return p.rule("directionStatement", func() {
		p.consume(tokDirection)
		p.or(0, []alternative{
			{alt: func() { p.consume(tokTd) }},
			{alt: func() { p.consume(tokTb) }},
			{alt: func() { p.consume(tokBt) }},
			{alt: func() { p.consume(tokLr) }},
			{alt: func() { p.consume(tokRl) }},
		})
		p.subrule(p.lineEnd)
	})
}

func (p *usecaseParser) classDefStatement() *cstNode {
	return p.rule("classDefStatement", func() {
		p.consume(tokClassDef)
		p.consume(tokIdentifier)
		p.many(0, func() {
			p.consume(tokComma)
			p.consume(tokIdentifier)
		})
		p.subrule(p.styles)
		p.subrule(p.lineEnd)
	})
}

func (p *usecaseParser) classStatement() *cstNode {
	return p.rule("classStatement", func() {
		p.consume(tokClass)
		p.consume(tokIdentifier)
		p.many(0, func() {
			p.consume(tokComma)
			p.consume(tokIdentifier)
		})
		p.consume(tokIdentifier)
		p.many(2, func() {
			p.consume(tokComma)
			p.consume(tokIdentifier)
		})
		p.subrule(p.lineEnd)
	})
}

func (p *usecaseParser) styleStatement() *cstNode {
	return p.rule("styleStatement", func() {
		p.consume(tokStyle)
		p.consume(tokIdentifier)
		p.subrule(p.styles)
		p.subrule(p.lineEnd)
	})
}

func (p *usecaseParser) styles() *cstNode {
	return p.rule("styles", func() {
		p.subrule(p.styleValue)
		p.many(0, func() {
			p.consume(tokComma)
			p.subrule(p.styleValue)
		})
	})
}

func (p *usecaseParser) styleValue() *cstNode {
	return p.rule("styleValue", func() {
		p.or(0, []alternative{
			{alt: func() { p.consume(tokWord) }},
			{alt: func() { p.consume(tokCssIdentifier) }},
			{
				alt: func() {
					p.consume(tokMarkerlessSolid)
					p.consume(tokWord)
				},
			},
		})
		p.consume(tokColon)
		p.atLeastOne(0, func() { p.subrule(p.styleComponent) })
	})
}

func (p *usecaseParser) styleComponent() *cstNode {
	return p.rule("styleComponent", func() {
		p.or(0, []alternative{
			{alt: func() { p.consume(tokWord) }},
			{alt: func() { p.consume(tokPlainString) }},
			{alt: func() { p.consume(tokNumber) }},
			{alt: func() { p.consume(tokHashColor) }},
			{alt: func() { p.consume(tokCssIdentifier) }},
			{alt: func() { p.consume(tokCssEscapedComma) }},
			{alt: func() { p.consume(tokDash) }},
			{alt: func() { p.consume(tokDot) }},
			{alt: func() { p.consume(tokPercent) }},
			{alt: func() { p.consume(tokCssPunctuation) }},
			{alt: func() { p.consume(tokColon) }},
			{alt: func() { p.consume(tokLparen) }},
			{alt: func() { p.consume(tokRparen) }},
			{alt: func() { p.consume(tokLbracket) }},
			{alt: func() { p.consume(tokRbracket) }},
			{alt: func() { p.consume(tokLbrace) }},
			{alt: func() { p.consume(tokRbrace) }},
			{alt: func() { p.consume(tokAt) }},
			{alt: func() { p.consume(tokMarkerlessSolid) }},
		})
	})
}

func (p *usecaseParser) isMetadataAssignment() bool {
	target := p.la(1).typ
	return (target == tokIdentifier || target == tokPlainString || target == tokMarkdownString) &&
		p.la(2).typ == tokMetadataStart
}

func (p *usecaseParser) isStatementStart() bool {
	tokType := p.la(1).typ
	return tokType != tokEof && tokType != tokNewline && tokType != tokComment
}

func (p *usecaseParser) isForbiddenPlantUmlStatement() bool {
	// toLowerCase of an identifier: its characters are ASCII.
	return p.la(1).typ == tokIdentifier &&
		forbiddenPlantUmlStatements[strings.ToLower(p.la(1).image)]
}

func (p *usecaseParser) hasLabeledRight(allowed ...tokenType) bool {
	if !isLabelToken(p.la(1)) {
		return false
	}
	for index := 2; ; index++ {
		tok := p.la(index)
		for _, typ := range allowed {
			if tok.typ == typ {
				return true
			}
		}
		if !isLabelToken(tok) {
			return false
		}
	}
}
