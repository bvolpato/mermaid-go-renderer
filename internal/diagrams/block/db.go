// Ported from mermaid 12.2.0, diagrams/block/blockDB.ts (MIT).

package block

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
)

const (
	colorKeyword  = "color"
	fillKeyword   = "fill"
	bgFill        = "bgFill"
	styleclassSep = ","
)

// jsError is a JavaScript exception that upstream throws for an input, with
// the name of its class and its message.
type jsError struct {
	Name    string
	Message string
}

func (e *jsError) Error() string { return e.Message }

func typeError(message string) error {
	return &jsError{Name: "TypeError", Message: message}
}

// blockKey is a key of the block database: the id of a block, which can be
// undefined.
type blockKey struct {
	id        string
	undefined bool
}

// maxMapSize is the largest number of entries of a Map in V8. A Map that
// grows past it throws a RangeError.
const maxMapSize = 1 << 24

// maxSpaceBlocks is the largest number of blocks that the space statements
// of one diagram can make. Upstream has no limit below maxMapSize: it makes
// one block for each column of a space statement, and the renderer measures
// every one. A larger number is an error here, so that a short text cannot
// use all the memory.
const maxSpaceBlocks = 1 << 14

// DB is the block db: the module-level state of blockDB.ts.
type DB struct {
	ctx *render.Context

	// blockDatabase is the node database for simple lookups.
	blockDatabase *js.Map[blockKey, *Block]
	edgeList      []*Block
	edgeCount     map[string]int
	classes       *js.Map[string, *ClassDef]
	diagramID     string
	// nextColorIndex is the next palette slot to hand out. Composites only,
	// and one counter across the whole parse.
	nextColorIndex int
	// spaceBlocks is the number of blocks that the space statements made
	// (see maxSpaceBlocks).
	spaceBlocks float64
	blocks      []*Block
	rootBlock   *Block

	// beforeLayout, when not nil, runs at the start of layout(). The tests
	// use it to read the block sizes that the renderer measured.
	beforeLayout func()
}

// NewDB returns a db in the state after clear().
func NewDB(ctx *render.Context) *DB {
	d := &DB{ctx: ctx}
	d.Clear()
	return d
}

// Clear is clear().
func (d *DB) Clear() {
	d.rootBlock = newBlock("id", "type", "children", "columns")
	d.rootBlock.ID = "root"
	d.rootBlock.Type = typeComposite
	d.rootBlock.Children = []*Block{}
	d.rootBlock.Columns = -1
	d.blockDatabase = js.NewMap[blockKey, *Block]()
	d.blockDatabase.Set(blockKey{id: "root"}, d.rootBlock)
	d.blocks = []*Block{}
	d.classes = js.NewMap[string, *ClassDef]()

	d.edgeList = []*Block{}
	d.edgeCount = map[string]int{}
	d.diagramID = ""
	d.nextColorIndex = 0
	d.spaceBlocks = 0
}

// sanitizeText is sanitizeText(txt): common.sanitizeText(txt, getConfig()).
func (d *DB) sanitizeText(txt string) (string, error) {
	return d.ctx.Sanitizer.SanitizeTextErr(txt, d.ctx.Config)
}

// AddStyleClass is addStyleClass(id, styleAttributes). The parser calls it
// for a (style) class definition:
//
//	classDef my-style fill:#f96;
//
// id is the id of the class, and styleAttributes is the string with one or
// more style attributes, each separated by a comma.
func (d *DB) AddStyleClass(id string, styleAttributes string) {
	// create a new style class object with this id
	foundClass, ok := d.classes.Lookup(id)
	if !ok {
		foundClass = &ClassDef{ID: id, Styles: []string{}, TextStyles: []string{}}
		d.classes.Set(id, foundClass) // This is a classDef
	}
	for _, attrib := range strings.Split(styleAttributes, styleclassSep) {
		// remove any trailing ;
		// attrib.replace(/([^;]*);/, '$1') removes the first ";".
		fixedAttrib := js.TrimSpace(strings.Replace(attrib, ";", "", 1))

		// replace some style keywords
		if strings.Contains(attrib, colorKeyword) {
			newStyle1 := strings.Replace(fixedAttrib, fillKeyword, bgFill, 1)
			newStyle2 := strings.Replace(newStyle1, colorKeyword, fillKeyword, 1)
			foundClass.TextStyles = append(foundClass.TextStyles, newStyle2)
		}
		foundClass.Styles = append(foundClass.Styles, fixedAttrib)
	}
}

// AddStyle2Node is addStyle2Node(id, styles). The parser calls it for a
// style definition:
//
//	style my-block-id fill:#f96;
//
// id is the id of the block to style, and styles is the string with one or
// more style attributes, each separated by a comma.
func (d *DB) AddStyle2Node(id string, styles string) error {
	foundBlock, ok := d.blockDatabase.Lookup(blockKey{id: id})
	if !ok {
		return typeError("Cannot set properties of undefined (setting 'styles')")
	}
	foundBlock.Styles = strings.Split(styles, styleclassSep)
	foundBlock.define("styles")
	return nil
}

// SetCssClass is setCssClass(itemIds, cssClassName): it adds a CSS class to
// the blocks with the given ids. itemIds is one id or a list of ids with
// commas. A block that is not in the list of known blocks is added.
func (d *DB) SetCssClass(itemIds string, cssClassName string) {
	for _, id := range strings.Split(itemIds, ",") {
		foundBlock, ok := d.blockDatabase.Lookup(blockKey{id: id})
		if !ok {
			trimmedID := js.TrimSpace(id)
			foundBlock = newBlock("id", "type", "children")
			foundBlock.ID = trimmedID
			foundBlock.Type = typeNA
			foundBlock.Children = []*Block{}
			d.blockDatabase.Set(blockKey{id: trimmedID}, foundBlock)
		}
		if foundBlock.Classes == nil {
			foundBlock.Classes = []string{}
			foundBlock.define("classes")
		}
		foundBlock.Classes = append(foundBlock.Classes, cssClassName)
	}
}

// flat is _blockList.flat(): a statement with links is a list of blocks.
func flat(blockList []any) []*Block {
	out := make([]*Block, 0, len(blockList))
	for _, e := range blockList {
		switch x := e.(type) {
		case *Block:
			out = append(out, x)
		case []*Block:
			out = append(out, x...)
		}
	}
	return out
}

// populateBlockDatabase is populateBlockDatabase(_blockList, parent).
func (d *DB) populateBlockDatabase(_blockList []any, parent *Block) error {
	blockList := flat(_blockList)
	children := []*Block{}
	// The column setting of the list is read for a warning only.
	for _, block := range blockList {
		if block.HasLabel && block.Label != "" {
			label, err := d.sanitizeText(block.Label)
			if err != nil {
				return err
			}
			block.Label = label
		}
		if block.Type == typeClassDef {
			d.AddStyleClass(block.ID, block.CSS)
			continue
		}
		if block.Type == typeApplyClass {
			d.SetCssClass(block.ID, block.StyleClass)
			continue
		}
		if block.Type == typeApplyStyles {
			if block.StylesStr != "" {
				if err := d.AddStyle2Node(block.ID, block.StylesStr); err != nil {
					return err
				}
			}
			continue
		}
		if block.Type == typeColumnSetting {
			parent.Columns = nn(block.Columns, -1)
			parent.define("columns")
		} else if block.Type == typeEdge {
			count := d.edgeCount[block.ID] + 1
			d.edgeCount[block.ID] = count
			block.ID = strconv.Itoa(count) + "-" + block.ID
			d.edgeList = append(d.edgeList, block)
		} else {
			if !block.HasLabel || block.Label == "" {
				if block.Type == typeComposite {
					block.Label, block.HasLabel = "", true
				} else {
					// block.label = block.id
					block.Label, block.HasLabel = block.ID, !block.IDUndefined
				}
				block.define("label")
			}
			existingBlock, exists := d.blockDatabase.Lookup(block.key())

			if !exists {
				// Assigned here, before the recursion into `block.children` below, so an outer
				// composite takes a lower slot than any composite nested inside it.
				if block.Type == typeComposite {
					block.ColorIndex = float64(d.nextColorIndex)
					block.define("colorIndex")
					d.nextColorIndex++
				}
				d.blockDatabase.Set(block.key(), block)
			} else {
				// Add newer relevant data to aggregated node
				if block.Type != typeNA {
					existingBlock.Type = block.Type
					existingBlock.define("type")
				}
				if !block.labelEqualsID() {
					existingBlock.Label, existingBlock.HasLabel = block.Label, block.HasLabel
					existingBlock.define("label")
				}
			}

			if block.hasChildren() {
				if err := d.populateBlockDatabase(block.document, block); err != nil {
					return err
				}
			}
			if block.Type == typeSpace {
				w := nn(block.Width, 1)
				if float64(d.blockDatabase.Len())+w > maxMapSize {
					return &jsError{Name: "RangeError", Message: "Map maximum size exceeded"}
				}
				d.spaceBlocks += w
				if d.spaceBlocks > maxSpaceBlocks {
					return fmt.Errorf("block: the space statements span %s columns, the limit is %d", js.Str(d.spaceBlocks), maxSpaceBlocks)
				}
				for j := 0; float64(j) < w; j++ {
					newBlock := block.clone()
					newBlock.ID = newBlock.idString() + "-" + strconv.Itoa(j)
					newBlock.IDUndefined = false
					d.blockDatabase.Set(newBlock.key(), newBlock)
					children = append(children, newBlock)
				}
			} else if !exists {
				children = append(children, block)
			}
		}
	}
	parent.Children = children
	parent.document, parent.hasDocument = nil, false
	parent.define("children")
	return nil
}

// TypeStr2Type is typeStr2Type(typeStr): the block type for the start and
// the end of a node shape. An undefined typeStr is "": it gives "na", as
// every text that is not a shape does.
func TypeStr2Type(typeStr string) string {
	switch typeStr {
	case "[]":
		return typeSquare
	case "()":
		return typeRound
	case "(())":
		return typeCircle
	case ">]":
		return typeRectLeftInv
	case "{}":
		return typeDiamond
	case "{{}}":
		return typeHexagon
	case "([])":
		return typeStadium
	case "[[]]":
		return typeSubroutine
	case "[()]":
		return typeCylinder
	case "((()))":
		return typeDoubleCircle
	case "[//]":
		return typeLeanRight
	case `[\\]`:
		return typeLeanLeft
	case `[/\]`:
		return typeTrapezoid
	case `[\/]`:
		return typeInvTrapezoid
	case "<[]>":
		return typeBlockArrow
	default:
		return typeNA
	}
}

// EdgeTypeStr2Type is edgeTypeStr2Type(typeStr).
func EdgeTypeStr2Type(typeStr string) string {
	switch typeStr {
	case "==":
		return "thick"
	default:
		return "normal"
	}
}

// EdgeStrToEdgeData is edgeStrToEdgeData(typeStr): the arrow type at the
// end of a link.
func EdgeStrToEdgeData(typeStr string) string {
	trimmed := js.TrimSpace(typeStr)
	lastChar := js.StringSliceFrom(trimmed, -1)
	switch lastChar {
	case "x":
		return "arrow_cross"
	case "o":
		return "arrow_circle"
	case ">":
		return "arrow_point"
	default:
		return ""
	}
}

// EdgeStrToEdgeStartData is edgeStrToEdgeStartData(typeStr): the arrow type
// at the start of a link.
func EdgeStrToEdgeStartData(typeStr string) string {
	firstChar := js.CharAt(js.TrimSpace(typeStr), 0)
	switch firstChar {
	case "x":
		return "arrow_cross"
	case "o":
		return "arrow_circle"
	case "<":
		return "arrow_point"
	default:
		return "arrow_open"
	}
}

// EdgeStrToThickness is edgeStrToThickness(typeStr).
func EdgeStrToThickness(typeStr string) string {
	if strings.Contains(typeStr, "==") {
		return "thick"
	}
	return "normal"
}

// EdgeStrToPattern is edgeStrToPattern(typeStr).
func EdgeStrToPattern(typeStr string) string {
	if strings.Contains(typeStr, ".-") {
		return "dotted"
	}
	return "solid"
}

// GenerateId is generateId(): a random id with a counter at the end. The
// counter is the module-level cnt of blockDB.ts, which clear() does not
// reset.
func (d *DB) GenerateId() string {
	cnt := d.ctx.Counter("block.generateId") + 1
	return "id-" + js.Substr(js.ToStringRadix(d.ctx.Random(), 36), 2, 12) + "-" + strconv.Itoa(cnt)
}

// SetHierarchy is setHierarchy(block): the parser gives the statements of
// the diagram to the db.
func (d *DB) SetHierarchy(block []any) error {
	d.rootBlock.document, d.rootBlock.hasDocument = block, true
	if err := d.populateBlockDatabase(block, d.rootBlock); err != nil {
		return err
	}
	d.blocks = d.rootBlock.Children
	return nil
}

// GetColumns is getColumns(blockId).
func (d *DB) GetColumns(blockID string) float64 {
	return d.getColumns(blockKey{id: blockID})
}

func (d *DB) getColumns(key blockKey) float64 {
	block, ok := d.blockDatabase.Lookup(key)
	if !ok {
		return -1
	}
	if js.Truthy(block.Columns) {
		return block.Columns
	}
	if !block.hasChildren() {
		return -1
	}
	return float64(len(block.Children))
}

// GetBlocksFlat is getBlocksFlat(): all the blocks as a flat list.
func (d *DB) GetBlocksFlat() []*Block {
	return d.blockDatabase.Values()
}

// GetBlocks is getBlocks(): the hierarchy of blocks.
func (d *DB) GetBlocks() []*Block {
	return d.blocks
}

// GetEdges is getEdges().
func (d *DB) GetEdges() []*Block {
	return d.edgeList
}

// GetBlock is getBlock(id). The result is nil when the db has no such
// block.
func (d *DB) GetBlock(id string) *Block {
	return d.blockDatabase.Get(blockKey{id: id})
}

// getBlock is getBlock(id) for an id that can be undefined.
func (d *DB) getBlock(key blockKey) *Block {
	return d.blockDatabase.Get(key)
}

// SetBlock is setBlock(block).
func (d *DB) SetBlock(block *Block) {
	d.blockDatabase.Set(block.key(), block)
}

// SetDiagramId is setDiagramId(id).
func (d *DB) SetDiagramId(id string) {
	d.diagramID = id
}

// GetDiagramId is getDiagramId().
func (d *DB) GetDiagramId() string { return d.diagramID }

// GetClasses is getClasses(): all of the style classes.
func (d *DB) GetClasses() *js.Map[string, *ClassDef] {
	return d.classes
}

// GetConfig is getConfig(): the block part of the config.
func (d *DB) GetConfig() *js.Obj[any] {
	return js.AsObj(d.ctx.Cfg("block"))
}
