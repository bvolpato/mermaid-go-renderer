// The calls that Go makes into cytoscape. Each function runs the statements
// of the mermaid function that it stands for, in the same order, against the
// environment of env.js, and returns what mermaid reads afterwards.
//
// Ported from mermaid 12.2.0:
//   rendering-util/layout-algorithms/cose-bilkent/cytoscape-setup.ts
//   diagrams/architecture/architectureRenderer.ts (layoutArchitecture)
//   diagrams/architecture/architectureSeed.ts
//   diagrams/architecture/svgDraw.ts (the reads of drawEdges and drawGroups)
//
// modules is the result of cytoscape.bundle.js, env the result of env.js.
(function (modules, env) {
  'use strict';
  var document = env.document;
  var cytoscape = modules.cytoscape();

  // cytoscape.use(coseBilkent) and cytoscape.use(fcose): mermaid registers a
  // layout when it loads the chunk of the diagram.
  var used = {};
  function use(name) {
    if (!used[name]) {
      used[name] = true;
      cytoscape.use(modules[name]());
    }
  }

  // createContainer is
  //   select('body').append('div').attr('id', 'cy').attr('style', 'display:none')
  function createContainer() {
    var renderEl = document.createElement('div');
    document.body.appendChild(renderEl);
    renderEl.setAttribute('id', 'cy');
    renderEl.setAttribute('style', 'display:none');
    return renderEl;
  }

  // afterAwait is the point where mermaid continues after
  // `await new Promise(...)`: the microtasks that were queued before run
  // first. The removal of the container queued one: the mutation observer of
  // the renderer, which destroys the renderer.
  function afterAwait() {
    env.notifyMutationObservers();
  }

  function point(p) {
    return { x: p.x, y: p.y };
  }

  // ---- cose-bilkent (cytoscape-setup.ts) ----

  function addNodes(nodes, cy) {
    nodes.forEach(function (node) {
      var nodeData = {
        id: node.id,
        labelText: node.label,
        height: node.height,
        width: node.width,
        padding: node.padding != null ? node.padding : 0,
      };

      // Add any additional properties from the node
      Object.keys(node).forEach(function (key) {
        if (['id', 'label', 'height', 'width', 'padding', 'x', 'y'].indexOf(key) < 0) {
          nodeData[key] = node[key];
        }
      });

      cy.add({
        group: 'nodes',
        data: nodeData,
        position: {
          x: node.x != null ? node.x : 0,
          y: node.y != null ? node.y : 0,
        },
      });
    });
  }

  function addEdges(edges, cy) {
    edges.forEach(function (edge) {
      var edgeData = {
        id: edge.id,
        source: edge.start,
        target: edge.end,
      };

      // Add any additional properties from the edge
      Object.keys(edge).forEach(function (key) {
        if (['id', 'start', 'end'].indexOf(key) < 0) {
          edgeData[key] = edge[key];
        }
      });

      cy.add({
        group: 'edges',
        data: edgeData,
      });
    });
  }

  function createCytoscapeInstance(data) {
    // Add temporary render element
    var renderEl = createContainer();

    var cy = cytoscape({
      container: document.getElementById('cy'), // container to render in
      style: [
        {
          selector: 'edge',
          style: {
            'curve-style': 'bezier',
          },
        },
      ],
    });

    // Remove element after layout
    renderEl.remove();

    // Add all nodes and edges to cytoscape using the generic functions
    addNodes(data.nodes, cy);
    addEdges(data.edges, cy);

    // Make cytoscape care about the dimensions of the nodes
    cy.nodes().forEach(function (n) {
      n.layoutDimensions = function () {
        var nodeData = n.data();
        return { w: nodeData.width, h: nodeData.height };
      };
    });

    // Configure and run the cose-bilkent layout
    var layoutConfig = {
      name: 'cose-bilkent',
      quality: 'proof',
      styleEnabled: false,
      animate: false,
    };

    cy.layout(layoutConfig).run();

    var resolved;
    cy.ready(function () {
      resolved = cy;
    });
    return resolved;
  }

  // coseBilkent is executeCoseBilkentLayout without the validation: it
  // returns the fields of the positioned nodes and edges that render.ts reads.
  function coseBilkent(data) {
    use('coseBilkent');
    var cy = createCytoscapeInstance(data);
    afterAwait();
    if (!cy) {
      throw new Error('cytoscape did not become ready');
    }
    // extractPositionedNodes
    var nodes = cy.nodes().map(function (node) {
      var data = node.data();
      var position = node.position();
      return { id: data.id, x: position.x, y: position.y };
    });
    // extractPositionedEdges
    var edges = cy.edges().map(function (edge) {
      var data = edge.data();
      var rscratch = edge._private.rscratch;
      return {
        id: data.id,
        source: data.source,
        target: data.target,
        startX: rscratch.startX,
        startY: rscratch.startY,
        midX: rscratch.midX,
        midY: rscratch.midY,
        endX: rscratch.endX,
        endY: rscratch.endY,
      };
    });
    return { nodes: nodes, edges: edges, pendingTasks: env.pendingTasks() };
  }

  // ---- fcose (architectureRenderer.ts) ----

  // withSeededRandom is architectureSeed.ts.
  function withSeededRandom(seed, fn) {
    if (seed === 0) {
      return fn();
    }
    var original = Math.random;
    var state = seed >>> 0;
    Math.random = function () {
      state = (state + 0x6d2b79f5) >>> 0;
      var t = state;
      t = Math.imul(t ^ (t >>> 15), t | 1);
      t ^= t + Math.imul(t ^ (t >>> 7), t | 61);
      return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
    };
    try {
      return fn();
    } finally {
      Math.random = original;
    }
  }

  function isArchitectureDirectionX(x) {
    return x === 'L' || x === 'R';
  }
  function isArchitectureDirectionY(x) {
    return x === 'T' || x === 'B';
  }
  function isArchitectureDirectionXY(a, b) {
    return isArchitectureDirectionX(a) !== isArchitectureDirectionX(b);
  }
  function nodeData(node) {
    return node[0]._private.data;
  }
  function edgeData(edge) {
    return edge[0]._private.data;
  }

  // noteAdded keeps the index of the input item for an element that
  // cytoscape accepted. cy.add returns an empty collection for an element
  // that it drops. The table is outside of cytoscape: a write to an element
  // would change its state.
  var inputIndex = {};
  function noteAdded(added, index) {
    if (added.length > 0) {
      inputIndex[added[0].id()] = index;
    }
  }

  function addServices(services, cy, iconSize) {
    services.forEach(function (service, index) {
      var added = cy.add({
        group: 'nodes',
        data: {
          type: 'service',
          id: service.id,
          icon: service.icon,
          label: service.title,
          parent: service.in,
          width: iconSize,
          height: iconSize,
        },
        classes: 'node-service',
      });
      noteAdded(added, index);
    });
  }

  function addJunctions(junctions, cy, iconSize) {
    junctions.forEach(function (junction, index) {
      var added = cy.add({
        group: 'nodes',
        data: {
          type: 'junction',
          id: junction.id,
          parent: junction.in,
          width: iconSize,
          height: iconSize,
        },
        classes: 'node-junction',
      });
      noteAdded(added, index);
    });
  }

  function addGroups(groups, cy) {
    groups.forEach(function (group, index) {
      var added = cy.add({
        group: 'nodes',
        data: {
          type: 'group',
          id: group.id,
          icon: group.icon,
          label: group.title,
          parent: group.in,
        },
        classes: 'node-group',
      });
      noteAdded(added, index);
    });
  }

  function endpointOf(dir) {
    return dir === 'L' ? '0 50%' : dir === 'R' ? '100% 50%' : dir === 'T' ? '50% 0' : '50% 100%';
  }

  function addArchitectureEdges(edges, cy) {
    edges.forEach(function (parsedEdge, index) {
      var lhsId = parsedEdge.lhsId, rhsId = parsedEdge.rhsId, lhsInto = parsedEdge.lhsInto, lhsGroup = parsedEdge.lhsGroup;
      var rhsInto = parsedEdge.rhsInto, lhsDir = parsedEdge.lhsDir, rhsDir = parsedEdge.rhsDir, rhsGroup = parsedEdge.rhsGroup, title = parsedEdge.title;
      var edgeType = isArchitectureDirectionXY(parsedEdge.lhsDir, parsedEdge.rhsDir) ? 'segments' : 'straight';
      var edge = {
        id: lhsId + '-' + rhsId,
        label: title,
        source: lhsId,
        sourceDir: lhsDir,
        sourceArrow: lhsInto,
        sourceGroup: lhsGroup,
        sourceEndpoint: endpointOf(lhsDir),
        target: rhsId,
        targetDir: rhsDir,
        targetArrow: rhsInto,
        targetGroup: rhsGroup,
        targetEndpoint: endpointOf(rhsDir),
      };
      var added = cy.add({
        group: 'edges',
        data: edge,
        classes: edgeType,
      });
      noteAdded(added, index);
    });
  }

  function layoutArchitecture(input) {
    var conf = input.config;
    var renderEl = createContainer();
    var cy = cytoscape({
      container: document.getElementById('cy'),
      style: [
        {
          selector: 'edge',
          style: {
            'curve-style': 'straight',
            'source-endpoint': 'data(sourceEndpoint)',
            'target-endpoint': 'data(targetEndpoint)',
          },
        },
        {
          selector: 'edge[label]',
          style: {
            label: 'data(label)',
          },
        },
        {
          selector: 'edge.segments',
          style: {
            'curve-style': 'segments',
            'segment-weights': '0',
            'segment-distances': [0.5],
            'edge-distances': 'endpoints',
            'source-endpoint': 'data(sourceEndpoint)',
            'target-endpoint': 'data(targetEndpoint)',
          },
        },
        {
          selector: 'node',
          style: {
            'compound-sizing-wrt-labels': 'include',
          },
        },
        {
          selector: 'node[label]',
          style: {
            'text-valign': 'bottom',
            'text-halign': 'center',
            'font-size': conf.fontSize + 'px',
          },
        },
        {
          selector: '.node-service',
          style: {
            label: 'data(label)',
            width: 'data(width)',
            height: 'data(height)',
          },
        },
        {
          selector: '.node-junction',
          style: {
            width: 'data(width)',
            height: 'data(height)',
          },
        },
        {
          selector: '.node-group',
          style: {
            padding: conf.padding + 'px',
          },
        },
      ],
      layout: {
        name: 'grid',
        boundingBox: {
          x1: 0,
          x2: 100,
          y1: 0,
          y2: 100,
        },
      },
    });
    // Remove element after layout
    renderEl.remove();

    addGroups(input.groups, cy);
    addServices(input.services, cy, conf.iconSize);
    addJunctions(input.junctions, cy, conf.iconSize);
    addArchitectureEdges(input.edges, cy);
    var alignmentConstraint = input.alignmentConstraint;
    var relativePlacementConstraint = input.relativePlacementConstraint;

    var iconSize = conf.iconSize;
    var sameGroupIdealLength = conf.idealEdgeLengthMultiplier * iconSize;
    var crossGroupIdealLength = 0.5 * iconSize;
    var sameGroupElasticity = conf.edgeElasticity;
    var seed = conf.seed;

    var layout = cy.layout({
      name: 'fcose',
      quality: 'proof',
      randomize: conf.randomize,
      nodeSeparation: conf.nodeSeparation,
      numIter: conf.numIter,
      styleEnabled: false,
      animate: false,
      nodeDimensionsIncludeLabels: false,
      idealEdgeLength: function (edge) {
        var nodes = edge.connectedNodes();
        var parentA = nodeData(nodes[0]).parent;
        var parentB = nodeData(nodes[1]).parent;
        return parentA === parentB ? sameGroupIdealLength : crossGroupIdealLength;
      },
      edgeElasticity: function (edge) {
        var nodes = edge.connectedNodes();
        var parentA = nodeData(nodes[0]).parent;
        var parentB = nodeData(nodes[1]).parent;
        return parentA === parentB ? sameGroupElasticity : 0.001;
      },
      alignmentConstraint: alignmentConstraint,
      relativePlacementConstraint: relativePlacementConstraint,
    });

    // Once the diagram has been generated and the service's position cords are set, adjust the XY edges to have a 90deg bend
    layout.one('layoutstop', function () {
      function getSegmentWeights(source, target, pointX, pointY) {
        var W, D;
        var sX = source.x, sY = source.y;
        var tX = target.x, tY = target.y;

        D =
          (pointY - sY + ((sX - pointX) * (sY - tY)) / (sX - tX)) /
          Math.sqrt(1 + Math.pow((sY - tY) / (sX - tX), 2));
        W = Math.sqrt(Math.pow(pointY - sY, 2) + Math.pow(pointX - sX, 2) - Math.pow(D, 2));

        var distAB = Math.sqrt(Math.pow(tX - sX, 2) + Math.pow(tY - sY, 2));
        W = W / distAB;

        var delta1 = (tX - sX) * (pointY - sY) - (tY - sY) * (pointX - sX);
        switch (true) {
          case delta1 >= 0:
            delta1 = 1;
            break;
          case delta1 < 0:
            delta1 = -1;
            break;
        }
        var delta2 = (tX - sX) * (pointX - sX) + (tY - sY) * (pointY - sY);
        switch (true) {
          case delta2 >= 0:
            delta2 = 1;
            break;
          case delta2 < 0:
            delta2 = -1;
            break;
        }

        D = Math.abs(D) * delta1;
        W = W * delta2;

        return {
          distances: D,
          weights: W,
        };
      }
      cy.startBatch();
      var values = Object.values(cy.edges());
      for (var i = 0; i < values.length; i++) {
        var edge = values[i];
        if (edge.data != null && edge.data()) {
          var sPos = edge.source().position();
          var tPos = edge.target().position();
          var sX = sPos.x, sY = sPos.y;
          var tX = tPos.x, tY = tPos.y;
          if (sX !== tX && sY !== tY) {
            var sEP = edge.sourceEndpoint();
            var tEP = edge.targetEndpoint();
            var sourceDir = edgeData(edge).sourceDir;
            var pointX, pointY;
            if (isArchitectureDirectionY(sourceDir)) {
              pointX = sEP.x;
              pointY = tEP.y;
            } else {
              pointX = tEP.x;
              pointY = sEP.y;
            }
            var sw = getSegmentWeights(sEP, tEP, pointX, pointY);
            edge.style('segment-distances', sw.distances);
            edge.style('segment-weights', sw.weights);
          }
        }
      }
      cy.endBatch();
      withSeededRandom(seed, function () {
        return layout.run();
      });
    });
    try {
      withSeededRandom(seed, function () {
        return layout.run();
      });
    } catch (err) {
      // V8 says "Invalid array length". goja says the same for a length that
      // is not an array length, and "Invalid array or string length" for a
      // length that it cannot allocate.
      if (err instanceof RangeError && (err.message.indexOf('Invalid array length') >= 0 || err.message.indexOf('Invalid array or string length') >= 0)) {
        throw new Error(
          'Architecture layout failed: a declared `align row|column` directive ' +
            'likely contradicts the edge directions, or two declared alignments ' +
            'overlap on a shared node. Check that the order of members in each ' +
            '`align` chain is consistent with the edges between them, and that ' +
            'no node appears in two `align` directives along the same axis.'
        );
      }
      throw err;
    }

    var resolved;
    cy.ready(function () {
      resolved = cy;
    });
    return resolved;
  }

  // architecture is layoutArchitecture and the reads of drawEdges, drawGroups
  // and positionNodes, in the order of the draw function.
  function architecture(input) {
    use('fcose');
    var cy = layoutArchitecture(input);
    afterAwait();
    if (!cy) {
      throw new Error('cytoscape did not become ready');
    }
    // drawEdges
    var edges = cy.edges().map(function (edge) {
      var data = edgeData(edge);
      var s = edge[0].sourceEndpoint();
      var m = edge[0].midpoint();
      var t = edge[0].targetEndpoint();
      return {
        id: data.id,
        index: inputIndex[data.id],
        sourceEndpoint: point(s),
        midpoint: point(m),
        targetEndpoint: point(t),
        hasRscratch: !!edge[0]._private.rscratch,
      };
    });
    // drawGroups
    var nodes = cy.nodes().map(function (node) {
      var data = nodeData(node);
      var out = { id: data.id, type: data.type, index: inputIndex[data.id] };
      if (data.type === 'group') {
        var bb = node.boundingBox();
        out.boundingBox = { x1: bb.x1, y1: bb.y1, x2: bb.x2, y2: bb.y2, w: bb.w, h: bb.h };
      }
      return out;
    });
    // positionNodes
    cy.nodes().map(function (node, i) {
      var data = nodeData(node);
      if (data.type === 'group') {
        return;
      }
      nodes[i].x = node.position().x;
      nodes[i].y = node.position().y;
    });
    return { nodes: nodes, edges: edges, pendingTasks: env.pendingTasks() };
  }

  // ---- the general call ----

  // run creates an instance in the way of mermaid (a container with
  // display:none that leaves the document right after the constructor), adds
  // the elements, runs the layouts in order and reads the geometry.
  //
  // spec is {
  //   style, layout          the options of the constructor (optional)
  //   elements               the arguments of cy.add, added one by one
  //   dataDimensions         give each node layoutDimensions from data.width
  //                          and data.height, as cytoscape-setup.ts does
  //   layouts                the options of cy.layout(...).run(), in order.
  //                          An option value { $perElement: { id: value } }
  //                          becomes a function of the element.
  //   seed                   run each layout inside withSeededRandom
  //   settle                 run the timers and frames that wait before the
  //                          read (at most this number)
  // }
  function run(spec) {
    var i;
    var layouts = spec.layouts || [];
    for (i = 0; i < layouts.length; i++) {
      if (layouts[i].name === 'cose-bilkent') {
        use('coseBilkent');
      } else if (layouts[i].name === 'fcose') {
        use('fcose');
      }
    }
    var renderEl = createContainer();
    var options = { container: document.getElementById('cy') };
    if (spec.style !== undefined) {
      options.style = spec.style;
    }
    if (spec.layout !== undefined) {
      options.layout = spec.layout;
    }
    var cy = cytoscape(options);
    renderEl.remove();

    var elements = spec.elements || [];
    for (i = 0; i < elements.length; i++) {
      cy.add(elements[i]);
    }
    if (spec.dataDimensions) {
      cy.nodes().forEach(function (n) {
        n.layoutDimensions = function () {
          var nodeData = n.data();
          return { w: nodeData.width, h: nodeData.height };
        };
      });
    }
    var seed = spec.seed === undefined ? 0 : spec.seed;
    for (i = 0; i < layouts.length; i++) {
      var layoutOptions = {};
      Object.keys(layouts[i]).forEach(function (key) {
        var value = layouts[i][key];
        if (value !== null && typeof value === 'object' && value.$perElement) {
          var table = value.$perElement;
          layoutOptions[key] = function (ele) {
            return table[ele.id()];
          };
        } else {
          layoutOptions[key] = value;
        }
      });
      var layout = cy.layout(layoutOptions);
      withSeededRandom(seed, function () {
        return layout.run();
      });
    }
    var resolved;
    cy.ready(function () {
      resolved = cy;
    });
    afterAwait();
    if (!resolved) {
      throw new Error('cytoscape did not become ready');
    }
    var pending = env.pendingTasks();
    if (spec.settle > 0) {
      pending = env.flush(spec.settle);
    }

    // Field reads first: they have no effect on the instance.
    var edges = cy.edges().map(function (edge) {
      var data = edge.data();
      var rs = edge._private.rscratch;
      return {
        id: data.id,
        source: data.source,
        target: data.target,
        startX: rs.startX,
        startY: rs.startY,
        midX: rs.midX,
        midY: rs.midY,
        endX: rs.endX,
        endY: rs.endY,
      };
    });
    var nodes = cy.nodes().map(function (node) {
      var position = node.position();
      return { id: node.id(), x: position.x, y: position.y, isParent: node.isParent() };
    });
    cy.edges().forEach(function (edge, i) {
      edges[i].sourceEndpoint = point(edge.sourceEndpoint());
      edges[i].midpoint = point(edge.midpoint());
      edges[i].targetEndpoint = point(edge.targetEndpoint());
    });
    cy.nodes().forEach(function (node, i) {
      var bb = node.boundingBox();
      nodes[i].boundingBox = { x1: bb.x1, y1: bb.y1, x2: bb.x2, y2: bb.y2, w: bb.w, h: bb.h };
    });
    return { nodes: nodes, edges: edges, pendingTasks: pending };
  }

  return {
    version: cytoscape.version,
    coseBilkent: coseBilkent,
    architecture: architecture,
    run: run,
  };
})
