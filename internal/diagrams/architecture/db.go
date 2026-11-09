// Ported from mermaid 12.2.0, diagrams/architecture/architectureDb.ts (MIT).

package architecture

import (
	"errors"
	"fmt"

	"github.com/bvolpato/mermaid-go-renderer/internal/config"
	"github.com/bvolpato/mermaid-go-renderer/internal/js"
	"github.com/bvolpato/mermaid-go-renderer/internal/render"
	"github.com/bvolpato/mermaid-go-renderer/internal/utils"
)

// typeError is a TypeError of JavaScript.
type typeError string

func (e typeError) Error() string { return string(e) }

// ArchitectureDB is the db of the architecture diagram.
type ArchitectureDB struct {
	render.CommonDB
	ctx *render.Context

	nodes  *js.Map[string, *ArchitectureNode]
	groups *js.Map[string, *ArchitectureGroup]
	edges  []*ArchitectureEdge
	// layoutHints is never nil: upstream starts with an empty array.
	layoutHints []*ArchitectureLayoutHint
	// registeredIds maps an id to "node" or "group".
	registeredIds  *js.Map[string, string]
	dataStructures *ArchitectureDataStructures
	elements       *js.Map[string, d3Element]
	diagramID      string
}

// NewDB is new ArchitectureDB(): the constructor calls clear().
func NewDB(ctx *render.Context) *ArchitectureDB {
	db := &ArchitectureDB{ctx: ctx}
	db.Sanitize = ctx.SanitizeText
	db.Clear()
	return db
}

// SetDiagramID is setDiagramId(id).
func (db *ArchitectureDB) SetDiagramID(id string) {
	db.diagramID = id
}

// GetDiagramID is getDiagramId().
func (db *ArchitectureDB) GetDiagramID() string {
	return db.diagramID
}

// Clear is clear().
func (db *ArchitectureDB) Clear() {
	db.nodes = js.NewMap[string, *ArchitectureNode]()
	db.groups = js.NewMap[string, *ArchitectureGroup]()
	db.edges = []*ArchitectureEdge{}
	db.layoutHints = []*ArchitectureLayoutHint{}
	db.registeredIds = js.NewMap[string, string]()
	db.dataStructures = nil
	db.elements = js.NewMap[string, d3Element]()
	db.diagramID = ""
	db.ClearCommon()
}

// checkParent holds the checks that addService, addJunction and addGroup
// make on the parent of a new element. kind is "service", "junction" or
// "group".
func (db *ArchitectureDB) checkParent(kind, id string, parent *string) error {
	if db.registeredIds.Has(id) {
		return fmt.Errorf("The %s id [%s] is already in use by another %s", kind, id, db.registeredIds.Get(id))
	}
	if parent != nil {
		if id == *parent {
			return fmt.Errorf("The %s [%s] cannot be placed within itself", kind, id)
		}
		if !db.registeredIds.Has(*parent) {
			return fmt.Errorf("The %s [%s]'s parent does not exist. Please make sure the parent is created before this %s", kind, id, kind)
		}
		if db.registeredIds.Get(*parent) == "node" {
			return fmt.Errorf("The %s [%s]'s parent is not a group", kind, id)
		}
	}
	return nil
}

// AddService is addService({ id, icon, in: parent, title, iconText }).
func (db *ArchitectureDB) AddService(id string, icon, parent, title, iconText *string) error {
	if err := db.checkParent("service", id, parent); err != nil {
		return err
	}

	db.registeredIds.Set(id, "node")

	db.nodes.Set(id, &ArchitectureNode{
		ID:       id,
		Type:     "service",
		Icon:     icon,
		IconText: iconText,
		Title:    title,
		Edges:    []*ArchitectureEdge{},
		In:       parent,
	})
	return nil
}

// GetServices is getServices().
func (db *ArchitectureDB) GetServices() []*ArchitectureNode {
	out := []*ArchitectureNode{}
	for _, n := range db.nodes.Values() {
		if isArchitectureService(n) {
			out = append(out, n)
		}
	}
	return out
}

// AddJunction is addJunction({ id, in: parent }).
func (db *ArchitectureDB) AddJunction(id string, parent *string) error {
	if err := db.checkParent("junction", id, parent); err != nil {
		return err
	}

	db.registeredIds.Set(id, "node")

	db.nodes.Set(id, &ArchitectureNode{
		ID:    id,
		Type:  "junction",
		Edges: []*ArchitectureEdge{},
		In:    parent,
	})
	return nil
}

// GetJunctions is getJunctions().
func (db *ArchitectureDB) GetJunctions() []*ArchitectureNode {
	out := []*ArchitectureNode{}
	for _, n := range db.nodes.Values() {
		if isArchitectureJunction(n) {
			out = append(out, n)
		}
	}
	return out
}

// GetNodes is getNodes().
func (db *ArchitectureDB) GetNodes() []*ArchitectureNode {
	return db.nodes.Values()
}

// GetNode is getNode(id). The result is nil when the id is not a node.
func (db *ArchitectureDB) GetNode(id string) *ArchitectureNode {
	return db.nodes.Get(id)
}

// AddGroup is addGroup({ id, icon, in: parent, title }).
func (db *ArchitectureDB) AddGroup(id string, icon, parent, title *string) error {
	if err := db.checkParent("group", id, parent); err != nil {
		return err
	}

	db.registeredIds.Set(id, "group")

	db.groups.Set(id, &ArchitectureGroup{
		ID:    id,
		Icon:  icon,
		Title: title,
		In:    parent,
	})
	return nil
}

// GetGroups is getGroups().
func (db *ArchitectureDB) GetGroups() []*ArchitectureGroup {
	return db.groups.Values()
}

// AddEdge is addEdge(edge).
func (db *ArchitectureDB) AddEdge(e ArchitectureEdge) error {
	lhsID, rhsID := e.LhsID, e.RhsID
	if !isArchitectureDirection(e.LhsDir) {
		return fmt.Errorf("Invalid direction given for left hand side of edge %s--%s. Expected (L,R,T,B) got %s", lhsID, rhsID, e.LhsDir)
	}
	if !isArchitectureDirection(e.RhsDir) {
		return fmt.Errorf("Invalid direction given for right hand side of edge %s--%s. Expected (L,R,T,B) got %s", lhsID, rhsID, e.RhsDir)
	}

	if !db.nodes.Has(lhsID) && !db.groups.Has(lhsID) {
		return fmt.Errorf("The left-hand id [%s] does not yet exist. Please create the service/group before declaring an edge to it.", lhsID)
	}
	if !db.nodes.Has(rhsID) && !db.groups.Has(rhsID) {
		return fmt.Errorf("The right-hand id [%s] does not yet exist. Please create the service/group before declaring an edge to it.", rhsID)
	}

	// this.nodes.get(lhsId)!.in: the id of a group is not a node, and the
	// read of the property throws.
	lhsNode := db.nodes.Get(lhsID)
	if lhsNode == nil {
		return typeError("Cannot read properties of undefined (reading 'in')")
	}
	rhsNode := db.nodes.Get(rhsID)
	if rhsNode == nil {
		return typeError("Cannot read properties of undefined (reading 'in')")
	}
	lhsGroupID := lhsNode.In
	rhsGroupID := rhsNode.In
	sameGroup := truthy(lhsGroupID) && truthy(rhsGroupID) && *lhsGroupID == *rhsGroupID
	if e.LhsGroup && sameGroup {
		return fmt.Errorf("The left-hand id [%s] is modified to traverse the group boundary, but the edge does not pass through two groups.", lhsID)
	}
	if e.RhsGroup && sameGroup {
		return fmt.Errorf("The right-hand id [%s] is modified to traverse the group boundary, but the edge does not pass through two groups.", rhsID)
	}

	edge := &ArchitectureEdge{
		LhsID:    lhsID,
		LhsDir:   e.LhsDir,
		LhsInto:  e.LhsInto,
		LhsGroup: e.LhsGroup,
		RhsID:    rhsID,
		RhsDir:   e.RhsDir,
		RhsInto:  e.RhsInto,
		RhsGroup: e.RhsGroup,
		Title:    e.Title,
	}

	db.edges = append(db.edges, edge)
	lhsNode.Edges = append(lhsNode.Edges, db.edges[len(db.edges)-1])
	rhsNode.Edges = append(rhsNode.Edges, db.edges[len(db.edges)-1])
	return nil
}

// truthy tells whether an optional string is truthy: defined and not empty.
func truthy(s *string) bool {
	return s != nil && *s != ""
}

// GetEdges is getEdges().
func (db *ArchitectureDB) GetEdges() []*ArchitectureEdge {
	return db.edges
}

// AddLayoutHint is addLayoutHint(hint).
func (db *ArchitectureDB) AddLayoutHint(hint *ArchitectureLayoutHint) error {
	if len(hint.Members) < 2 {
		return fmt.Errorf("An align directive requires at least two members; got %d", len(hint.Members))
	}
	seen := js.NewSet[string]()
	for _, id := range hint.Members {
		if kind, _ := db.registeredIds.Lookup(id); kind != "node" {
			return fmt.Errorf("align %s references [%s], which is not a service or junction", hint.Direction, id)
		}
		if seen.Has(id) {
			return fmt.Errorf("align %s lists [%s] more than once", hint.Direction, id)
		}
		seen.Add(id)
	}
	db.layoutHints = append(db.layoutHints, hint)
	return nil
}

// GetLayoutHints is getLayoutHints().
func (db *ArchitectureDB) GetLayoutHints() []*ArchitectureLayoutHint {
	return db.layoutHints
}

// GetDataStructures is getDataStructures(): it returns the adjacency list,
// the spatial maps and the group alignments of the diagram. If they have
// not been created, it runs the algorithms to generate them.
func (db *ArchitectureDB) GetDataStructures() (*ArchitectureDataStructures, error) {
	if db.dataStructures == nil {
		// Tracks how groups are aligned with one another. Generated while creating the adj list
		groupAlignments := js.NewMap[string, ArchitectureAlignment]()

		// Create an adjacency list of the diagram to perform BFS on
		// Outer Map applied on all services
		// Inner Map applied on the edges for a service
		adjList := js.NewMap[string, ArchitectureDirectionPairMap]()
		for _, id := range db.nodes.Keys() {
			service := db.nodes.Get(id)
			directionMap := js.NewMap[ArchitectureDirectionPair, string]()
			for _, edge := range service.Edges {
				// track the direction groups connect to one another
				var lhsGroupID, rhsGroupID *string
				if n := db.GetNode(edge.LhsID); n != nil {
					lhsGroupID = n.In
				}
				if n := db.GetNode(edge.RhsID); n != nil {
					rhsGroupID = n.In
				}
				if truthy(lhsGroupID) && truthy(rhsGroupID) && *lhsGroupID != *rhsGroupID {
					alignment := getArchitectureDirectionAlignment(edge.LhsDir, edge.RhsDir)
					if alignment != "bend" {
						groupAlignments.Set(architectureGroupAlignmentKey(*lhsGroupID, *rhsGroupID), alignment)
					}
				}

				if edge.LhsID == id {
					// source is LHS
					if pair, ok := getArchitectureDirectionPair(edge.LhsDir, edge.RhsDir); ok {
						directionMap.Set(pair, edge.RhsID)
					}
				} else {
					// source is RHS
					if pair, ok := getArchitectureDirectionPair(edge.RhsDir, edge.LhsDir); ok {
						directionMap.Set(pair, edge.LhsID)
					}
				}
			}

			adjList.Set(id, directionMap)
		}

		visited := js.NewSet[string]()
		// If a key is present in this set, it has not been visited
		notVisited := js.NewSet(adjList.Keys()...)

		// Perform BFS on the adjacency list
		bfs := func(startingID string) (ArchitectureSpatialMap, error) {
			spatialMap := js.NewMap[string, [2]float64]()
			spatialMap.Set(startingID, [2]float64{0, 0})
			queue := []string{startingID}
			for len(queue) > 0 {
				id := queue[0]
				queue = queue[1:]
				if id != "" {
					visited.Add(id)
					notVisited.Delete(id)
					adj, ok := adjList.Lookup(id)
					if !ok {
						return nil, fmt.Errorf("BFS error: adjacency list for id %s not found. Please report this as a bug.", id)
					}
					pos, ok := spatialMap.Lookup(id)
					if !ok {
						return nil, fmt.Errorf("BFS error: position for id %s not found in spatial map. Please report this as a bug.", id)
					}
					posX, posY := pos[0], pos[1]
					adj.Each(func(dir ArchitectureDirectionPair, rhsID string) {
						if !visited.Has(rhsID) {
							spatialMap.Set(rhsID, shiftPositionByArchitectureDirectionPair([2]float64{posX, posY}, dir))
							queue = append(queue, rhsID)
						}
					})
				}
			}
			return spatialMap, nil
		}
		spatialMaps := []ArchitectureSpatialMap{}

		// Keep performing BFS until all nodes have been visited.
		// If our diagram is disconnected, keep adding additional spatial maps until all disconnected graphs have been found
		for notVisited.Len() > 0 {
			firstID := notVisited.Values()[0]
			spatialMap, err := bfs(firstID)
			if err != nil {
				return nil, err
			}
			spatialMaps = append(spatialMaps, spatialMap)
		}

		db.dataStructures = &ArchitectureDataStructures{
			AdjList:         adjList,
			SpatialMaps:     spatialMaps,
			GroupAlignments: groupAlignments,
		}
	}
	return db.dataStructures, nil
}

// SetElementForID is setElementForId(id, element).
func (db *ArchitectureDB) SetElementForID(id string, element d3Element) {
	db.elements.Set(id, element)
}

// GetElementByID is getElementById(id). The result is nil for an id with no
// element.
func (db *ArchitectureDB) GetElementByID(id string) d3Element {
	return db.elements.Get(id)
}

// GetConfig is getConfig(): cleanAndMerge({ ...DEFAULT_ARCHITECTURE_CONFIG,
// ...commonGetConfig().architecture }).
func (db *ArchitectureDB) GetConfig() *js.Obj[any] {
	spread := js.NewObj[any]()
	js.AsObj(config.DefaultSection("architecture")).Each(func(key string, v any) { spread.Set(key, v) })
	if user := js.AsObj(db.ctx.Config.Get("architecture")); user != nil {
		user.Each(func(key string, v any) { spread.Set(key, v) })
	}
	return utils.CleanAndMerge(spread, nil)
}

// GetConfigField is getConfigField(field): a dynamic value of the package
// js. A field that the config does not have is nil.
func (db *ArchitectureDB) GetConfigField(field string) any {
	return db.GetConfig().Get(field)
}

// errNotArchitectureDB is the error of parser.parse for a db of another
// class.
var errNotArchitectureDB = errors.New("parser.parser?.yy was not a ArchitectureDB. This is due to a bug within Mermaid, please report this issue at https://github.com/mermaid-js/mermaid/issues.")
