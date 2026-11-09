// The debug hooks of the layouts, for tests. The script wraps methods of the
// layout classes of cytoscape.bundle.js and reports the state of a layout at
// stage boundaries. The Go ports of internal/cytoscape/native report the
// same values at the same points, and the tests compare them bit for bit.
//
// modules is the result of cytoscape.bundle.js. emit(stage, values) receives
// a stage name and an array of numbers. A hook only reads: it does not call
// a method that caches, so the layout is the layout without the hooks.
(function (modules, emit) {
  'use strict';

  // nodesOf is the order of LGraphManager.getAllNodes, without its cache.
  function nodesOf(gm) {
    var list = [];
    var graphs = gm.getGraphs();
    for (var i = 0; i < graphs.length; i++) {
      list = list.concat(graphs[i].getNodes());
    }
    return list;
  }

  function traced(iteration) {
    return iteration <= 20 || iteration % 50 === 0;
  }

  function num(v) {
    return typeof v === 'number' ? v : NaN;
  }

  function wrap(proto, name, before, after) {
    var original = proto[name];
    proto[name] = function () {
      if (before) {
        before.call(this);
      }
      var result = original.apply(this, arguments);
      if (after) {
        after.call(this, result);
      }
      return result;
    };
  }

  // graphRecord is the graph of a layout: the rectangle of each node, and
  // the ends of each edge. With lengths, each edge also has its ideal length
  // and its elasticity, and the edges between two graphs follow.
  function graphRecord(layout, lengths) {
    var nodes = nodesOf(layout.graphManager);
    var out = [nodes.length];
    var i, j;
    for (i = 0; i < nodes.length; i++) {
      out.push(nodes[i].rect.x, nodes[i].rect.y, nodes[i].rect.width, nodes[i].rect.height);
    }
    function edge(e) {
      out.push(nodes.indexOf(e.source), nodes.indexOf(e.target));
      if (lengths) {
        out.push(num(e.idealLength), num(e.edgeElasticity));
      }
    }
    var graphs = layout.graphManager.getGraphs();
    for (i = 0; i < graphs.length; i++) {
      var edges = graphs[i].getEdges();
      for (j = 0; j < edges.length; j++) {
        edge(edges[j]);
      }
    }
    if (lengths) {
      for (i = 0; i < layout.graphManager.edges.length; i++) {
        edge(layout.graphManager.edges[i]);
      }
    }
    return out;
  }

  function rects(layout) {
    var nodes = nodesOf(layout.graphManager);
    var out = [];
    for (var i = 0; i < nodes.length; i++) {
      out.push(nodes[i].rect.x, nodes[i].rect.y, nodes[i].rect.width, nodes[i].rect.height);
    }
    return out;
  }

  function positions(layout, head) {
    var nodes = nodesOf(layout.graphManager);
    for (var i = 0; i < nodes.length; i++) {
      head.push(nodes[i].rect.x, nodes[i].rect.y);
    }
    return head;
  }

  function forces(layout) {
    var nodes = nodesOf(layout.graphManager);
    var out = [layout.totalIterations];
    for (var i = 0; i < nodes.length; i++) {
      var n = nodes[i];
      out.push(num(n.springForceX), num(n.springForceY), num(n.repulsionForceX), num(n.repulsionForceY), num(n.gravitationForceX), num(n.gravitationForceY));
    }
    return out;
  }

  // ---- cose-base 1.0.3: the layout of cytoscape-cose-bilkent ----
  (function () {
    var proto = modules.coseBase1().CoSELayout.prototype;
    wrap(proto, 'runLayout', function () {
      emit('cose.graph', graphRecord(this, false));
    }, function () {
      emit('cose.final', rects(this));
    });
    wrap(proto, 'initSpringEmbedder', null, function () {
      emit('cose.initial', positions(this, [this.coolingFactor, this.maxIterations, this.totalDisplacementThreshold, this.repulsionRange]));
    });
    wrap(proto, 'moveNodes', function () {
      if (traced(this.totalIterations)) {
        emit('cose.forces', forces(this));
      }
    }, null);
    wrap(proto, 'tick', null, function (ended) {
      if (traced(this.totalIterations) || ended) {
        emit('cose.tick', positions(this, [this.totalIterations, this.coolingFactor, this.totalDisplacement, ended ? 1 : 0]));
      }
    });
  })();

  // ---- cose-base 2.2.0: the layout of cytoscape-fcose ----
  (function () {
    var coseBase = modules.coseBase2();
    var proto = coseBase.CoSELayout.prototype;
    wrap(proto, 'runLayout', function () {
      emit('cose.graph', graphRecord(this, true));
    }, function () {
      emit('cose.final', rects(this));
    });
    var handleConstraints = coseBase.ConstraintHandler.handleConstraints;
    coseBase.ConstraintHandler.handleConstraints = function (layout) {
      var result = handleConstraints.apply(this, arguments);
      emit('cose.constraints', rects(layout));
      return result;
    };
    wrap(proto, 'initSpringEmbedder', null, function () {
      emit('cose.initial', positions(this, [this.coolingFactor, this.maxIterations, this.totalDisplacementThreshold, this.repulsionRange]));
    });
    wrap(proto, 'moveNodes', function () {
      if (traced(this.totalIterations)) {
        emit('cose.forces', forces(this));
      }
    }, null);
    wrap(proto, 'tick', null, function (ended) {
      if (traced(this.totalIterations) || ended) {
        emit('cose.tick', positions(this, [this.totalIterations, this.coolingFactor, this.totalDisplacement, ended ? 1 : 0]));
      }
    });
  })();

  // ---- cytoscape, while fcose runs ----
  //
  // cy.center is the bounding box of all elements at the start of a run of
  // fcose, cy.fit the state of the elements after the fit at its end, and
  // cy.bypass a number that a style bypass sets.
  (function () {
    var real = modules.cytoscape();
    var active = false;
    var pendingCenter = false;
    var patched = false;

    function state(cy) {
      var out = [];
      cy.nodes().forEach(function (n) {
        var p = n._private;
        out.push(num(p.position.x), num(p.position.y), num(p.autoWidth), num(p.autoHeight), num(p.autoPadding),
          num(p.rstyle.labelX), num(p.rstyle.labelY), num(p.rstyle.labelWidth), num(p.rstyle.labelHeight));
      });
      cy.edges().forEach(function (e) {
        var rs = e._private.rscratch;
        var rstyle = e._private.rstyle;
        out.push(num(rs.startX), num(rs.startY), num(rs.endX), num(rs.endY), num(rs.arrowStartX), num(rs.arrowStartY), num(rs.arrowEndX), num(rs.arrowEndY), num(rs.midX), num(rs.midY),
          num(rstyle.labelX), num(rstyle.labelY), num(rstyle.labelWidth), num(rstyle.labelHeight));
      });
      return out;
    }

    function patch(cy) {
      var coreProto = Object.getPrototypeOf(cy);
      var collectionProto = Object.getPrototypeOf(cy.collection());
      var fit = coreProto.fit;
      coreProto.fit = function () {
        var result = fit.apply(this, arguments);
        if (active) {
          emit('cy.fit', state(this));
        }
        return result;
      };
      var boundingBox = collectionProto.boundingBox;
      collectionProto.boundingBox = function () {
        if (!pendingCenter) {
          return boundingBox.apply(this, arguments);
        }
        pendingCenter = false;
        var bb = boundingBox.apply(this, arguments);
        emit('cy.center', [bb.x1, bb.y1, bb.x2, bb.y2, bb.w, bb.h]);
        return bb;
      };
      var style = collectionProto.style;
      collectionProto.style = function (name, value) {
        if (active && typeof name === 'string' && typeof value === 'number') {
          emit('cy.bypass', [value]);
        }
        return style.apply(this, arguments);
      };
      var Fcose = real('layout', 'fcose');
      if (Fcose) {
        var run = Fcose.prototype.run;
        Fcose.prototype.run = function () {
          active = true;
          pendingCenter = true;
          try {
            return run.apply(this, arguments);
          } finally {
            pendingCenter = false;
          }
        };
      }
    }

    function wrapped() {
      var result = real.apply(this, arguments);
      if (!patched && result !== null && typeof result === 'object' && typeof result.collection === 'function') {
        patched = true;
        patch(result);
      }
      return result;
    }
    wrapped.use = function () {
      real.use.apply(real, arguments);
      return wrapped;
    };
    wrapped.warnings = real.warnings;
    wrapped.version = real.version;
    wrapped.stylesheet = wrapped.Stylesheet = real.stylesheet;
    modules.cytoscape = function () {
      return wrapped;
    };
  })();
})
