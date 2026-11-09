// The browser environment that cytoscape reads when mermaid runs a layout.
//
// mermaid creates the cytoscape instance with a container: a div with the
// style display:none that is in the document during the constructor and is
// removed right after it. cytoscape then uses its canvas renderer, and the
// renderer computes the edge geometry that mermaid reads. So the script needs
// a window, a document, a container element and a canvas. Nothing is drawn:
// a frame never runs between the layout and the moment mermaid reads the
// result.
//
// This file gives exactly the parts that cytoscape 3.34.0 touches on that
// path, with the values of Chromium for such a container (see the probe
// tools/reference/probes/cytoscape-env.js). A read of something else throws,
// so that a gap shows as an error and not as a wrong layout.
//
// host is the object of Go functions (see runtime.go).
(function (host) {
  'use strict';
  var g = globalThis;

  // ---- Math, Date, sort ----
  //
  // Every Math function that is not exact by definition comes from the host,
  // which has the results of Chromium. Math.random is the generator of the
  // render context. The clock does not move.
  var mathNames = Object.keys(host.math);
  for (var i = 0; i < mathNames.length; i++) {
    Object.defineProperty(Math, mathNames[i], { value: host.math[mathNames[i]], writable: true, configurable: true, enumerable: false });
  }
  Object.defineProperty(Math, 'random', { value: host.random, writable: true, configurable: true, enumerable: false });

  var NativeDate = Date;
  var fixedNow = host.now();
  function FixedDate() {
    if (!(this instanceof FixedDate)) {
      return new NativeDate(fixedNow).toString();
    }
    var a = arguments;
    switch (a.length) {
      case 0: return new NativeDate(fixedNow);
      case 1: return new NativeDate(a[0]);
      case 2: return new NativeDate(a[0], a[1]);
      case 3: return new NativeDate(a[0], a[1], a[2]);
      case 4: return new NativeDate(a[0], a[1], a[2], a[3]);
      case 5: return new NativeDate(a[0], a[1], a[2], a[3], a[4]);
      case 6: return new NativeDate(a[0], a[1], a[2], a[3], a[4], a[5]);
      default: return new NativeDate(a[0], a[1], a[2], a[3], a[4], a[5], a[6]);
    }
  }
  FixedDate.prototype = NativeDate.prototype;
  FixedDate.now = function () { return fixedNow; };
  FixedDate.parse = NativeDate.parse;
  FixedDate.UTC = NativeDate.UTC;
  g.Date = FixedDate;

  // Array.prototype.sort with a comparator runs the sort of V8 in the host:
  // the order of the comparator calls decides the result when the comparator
  // is not consistent. Without a comparator every stable sort agrees.
  var nativeSort = Array.prototype.sort;
  if (host.sort) {
    Object.defineProperty(Array.prototype, 'sort', {
      value: function sort(comparefn) {
        if (typeof comparefn !== 'function' || !Array.isArray(this)) {
          return nativeSort.call(this, comparefn);
        }
        return host.sort(this, comparefn);
      },
      writable: true,
      configurable: true,
      enumerable: false,
    });
  }

  // parseInt of goja adds the digits one by one, which rounds more than one
  // time for an integer above 2^53. V8 rounds the decimal text one time.
  // Number(text) of goja does that too, so a long run of decimal digits goes
  // through it. The layouts call parseInt on numbers: the common case stays
  // with the builtin.
  var nativeParseInt = g.parseInt;
  var longDecimal = /^[\s\uFEFF\xA0]*([+-]?)(\d{16,})/;
  var fixedParseInt = function parseInt(string, radix) {
    if (typeof string === 'number' && string > -1e15 && string < 1e15) {
      return nativeParseInt(string, radix);
    }
    var r = radix | 0;
    if (r === 0 || r === 10) {
      var m = longDecimal.exec(String(string));
      if (m !== null) {
        var value = Number(m[2]);
        return m[1] === '-' ? -value : value;
      }
    }
    return nativeParseInt(string, radix);
  };
  g.parseInt = fixedParseInt;
  Number.parseInt = fixedParseInt;

  // ---- timers ----
  //
  // A timer or an animation frame is a task, and a task never runs before
  // mermaid reads the layout. The queue keeps them. flush runs them with a
  // virtual clock, for a caller that asks for it.
  var tasks = [];
  var nextTaskId = 1;
  var virtualTime = 0;
  function addTask(kind, fn, delay, args) {
    var id = nextTaskId++;
    tasks.push({ id: id, kind: kind, fn: fn, at: virtualTime + (delay > 0 ? +delay : 0), args: args });
    return id;
  }
  function removeTask(id) {
    for (var i = 0; i < tasks.length; i++) {
      if (tasks[i].id === id) {
        tasks.splice(i, 1);
        return;
      }
    }
  }
  g.setTimeout = function (fn, delay) {
    return addTask('timeout', fn, delay, Array.prototype.slice.call(arguments, 2));
  };
  g.clearTimeout = removeTask;
  g.setInterval = function (fn, delay) {
    return addTask('interval', fn, delay, Array.prototype.slice.call(arguments, 2));
  };
  g.clearInterval = removeTask;
  g.requestAnimationFrame = function (fn) {
    return addTask('frame', fn, 0, null);
  };
  g.cancelAnimationFrame = removeTask;
  g.performance = {
    now: function () { return virtualTime; },
  };

  // ---- DOM ----
  function unsupported(what) {
    throw new Error('cytoscape environment: ' + what + ' is not supported');
  }

  // The observers with records, in the order of their creation.
  var observers = [];
  function MutationObserver(callback) {
    this._callback = callback;
    this._records = [];
    this._targets = [];
    observers.push(this);
  }
  MutationObserver.prototype.observe = function (target, options) {
    options = options || {};
    for (var i = 0; i < this._targets.length; i++) {
      if (this._targets[i].target === target) {
        this._targets[i].options = options;
        return;
      }
    }
    this._targets.push({ target: target, options: options });
    target._observers.push(this);
  };
  MutationObserver.prototype.disconnect = function () {
    for (var i = 0; i < this._targets.length; i++) {
      var list = this._targets[i].target._observers;
      var at = list.indexOf(this);
      if (at >= 0) {
        list.splice(at, 1);
      }
    }
    this._targets = [];
    this._records = [];
  };
  MutationObserver.prototype.takeRecords = function () {
    var r = this._records;
    this._records = [];
    return r;
  };
  MutationObserver.prototype._options = function (target) {
    for (var i = 0; i < this._targets.length; i++) {
      if (this._targets[i].target === target) {
        return this._targets[i].options;
      }
    }
    return null;
  };
  function queueRecord(target, record) {
    for (var i = 0; i < target._observers.length; i++) {
      var mo = target._observers[i];
      var options = mo._options(target);
      if (record.type === 'childList' ? options.childList : options.attributes) {
        mo._records.push(record);
      }
    }
  }
  // notifyMutationObservers is the microtask "notify mutation observers" of
  // the DOM standard.
  function notifyMutationObservers() {
    var list = observers.slice();
    for (var i = 0; i < list.length; i++) {
      var mo = list[i];
      var records = mo._records;
      mo._records = [];
      if (records.length > 0) {
        mo._callback.call(mo, records, mo);
      }
    }
  }

  function ResizeObserver(callback) {
    this._callback = callback;
  }
  // The first notification of a resize observer comes with a frame.
  ResizeObserver.prototype.observe = function () {};
  ResizeObserver.prototype.unobserve = function () {};
  ResizeObserver.prototype.disconnect = function () {};

  function EventTarget() {}
  EventTarget.prototype.addEventListener = function (type, listener, options) {
    // The browser reads the members of the options. cytoscape detects passive
    // listeners with a getter.
    if (options !== null && typeof options === 'object') {
      void options.capture;
      void options.passive;
      void options.once;
    }
  };
  EventTarget.prototype.removeEventListener = function () {};

  function Node() {}
  Node.prototype = Object.create(EventTarget.prototype);

  function HTMLElement(tagName, doc) {
    this.tagName = tagName.toUpperCase();
    this.nodeName = this.tagName;
    this.nodeType = 1;
    this.ownerDocument = doc;
    this.parentNode = null;
    this.children = [];
    this.style = {};
    this._attributes = {};
    this._observers = [];
    this._className = '';
    this.id = '';
    this.textContent = '';
  }
  HTMLElement.prototype = Object.create(Node.prototype);
  HTMLElement.prototype.constructor = HTMLElement;
  Object.defineProperty(HTMLElement.prototype, 'className', {
    get: function () { return this._className; },
    set: function (v) {
      this._className = String(v);
      queueRecord(this, { type: 'attributes', target: this, attributeName: 'class', addedNodes: [], removedNodes: [] });
    },
  });
  Object.defineProperty(HTMLElement.prototype, 'isConnected', {
    get: function () {
      var n = this;
      while (n.parentNode) {
        n = n.parentNode;
      }
      return n === this.ownerDocument.documentElement;
    },
  });
  HTMLElement.prototype.getRootNode = function () {
    var n = this;
    while (n.parentNode) {
      n = n.parentNode;
    }
    return n === this.ownerDocument.documentElement ? this.ownerDocument : n;
  };
  // The layout box of every element here is empty: the container has
  // display:none, and what cytoscape adds is inside it.
  Object.defineProperty(HTMLElement.prototype, 'clientWidth', { get: function () { return 0; } });
  Object.defineProperty(HTMLElement.prototype, 'clientHeight', { get: function () { return 0; } });
  Object.defineProperty(HTMLElement.prototype, 'offsetWidth', { get: function () { return 0; } });
  Object.defineProperty(HTMLElement.prototype, 'offsetHeight', { get: function () { return 0; } });
  HTMLElement.prototype.getBoundingClientRect = function () {
    return { x: 0, y: 0, left: 0, top: 0, right: 0, bottom: 0, width: 0, height: 0 };
  };
  // The attributes id and class are the properties id and className. The
  // style attribute is kept as text: nothing reads it back.
  HTMLElement.prototype.setAttribute = function (name, value) {
    value = String(value);
    if (name === 'id') {
      this.id = value;
    } else if (name === 'class') {
      this._className = value;
    } else {
      this._attributes[name] = value;
    }
    queueRecord(this, { type: 'attributes', target: this, attributeName: name, addedNodes: [], removedNodes: [] });
  };
  HTMLElement.prototype.getAttribute = function (name) {
    if (name === 'id') {
      return this.id === '' ? null : this.id;
    }
    if (name === 'class') {
      return this._className === '' ? null : this._className;
    }
    return Object.prototype.hasOwnProperty.call(this._attributes, name) ? this._attributes[name] : null;
  };
  HTMLElement.prototype.removeAttribute = function (name) {
    delete this._attributes[name];
  };
  function detach(child) {
    var parent = child.parentNode;
    if (parent) {
      parent.children.splice(parent.children.indexOf(child), 1);
      child.parentNode = null;
      queueRecord(parent, { type: 'childList', target: parent, addedNodes: [], removedNodes: [child] });
    }
  }
  HTMLElement.prototype.appendChild = function (child) {
    detach(child);
    this.children.push(child);
    child.parentNode = this;
    queueRecord(this, { type: 'childList', target: this, addedNodes: [child], removedNodes: [] });
    return child;
  };
  HTMLElement.prototype.insertBefore = function (child, ref) {
    detach(child);
    var at = ref == null ? -1 : this.children.indexOf(ref);
    if (at < 0) {
      this.children.push(child);
    } else {
      this.children.splice(at, 0, child);
    }
    child.parentNode = this;
    queueRecord(this, { type: 'childList', target: this, addedNodes: [child], removedNodes: [] });
    return child;
  };
  HTMLElement.prototype.removeChild = function (child) {
    if (child.parentNode !== this) {
      throw new Error('removeChild: the node is not a child of this node');
    }
    detach(child);
    return child;
  };
  HTMLElement.prototype.remove = function () {
    detach(this);
  };
  Object.defineProperty(HTMLElement.prototype, 'childNodes', { get: function () { return this.children; } });
  Object.defineProperty(HTMLElement.prototype, 'firstChild', { get: function () { return this.children[0] || null; } });

  // A 2D context. cytoscape uses it before a frame only to measure a label.
  function CanvasRenderingContext2D(canvas) {
    this.canvas = canvas;
    this.font = '10px sans-serif';
  }
  CanvasRenderingContext2D.prototype.measureText = function (text) {
    var width = host.measureText(String(this.font), String(text));
    return {
      width: width,
      get actualBoundingBoxAscent() { return unsupported('TextMetrics.actualBoundingBoxAscent'); },
      get actualBoundingBoxDescent() { return unsupported('TextMetrics.actualBoundingBoxDescent'); },
    };
  };

  function HTMLCanvasElement(doc) {
    HTMLElement.call(this, 'canvas', doc);
    this.width = 300;
    this.height = 150;
    this._context = null;
  }
  HTMLCanvasElement.prototype = Object.create(HTMLElement.prototype);
  HTMLCanvasElement.prototype.constructor = HTMLCanvasElement;
  HTMLCanvasElement.prototype.getContext = function (type) {
    if (type !== '2d') {
      return null;
    }
    if (!this._context) {
      this._context = new CanvasRenderingContext2D(this);
    }
    return this._context;
  };

  function Document() {
    this.nodeType = 9;
    this.defaultView = g;
    this.documentElement = new HTMLElement('html', this);
    this.head = new HTMLElement('head', this);
    this.body = new HTMLElement('body', this);
    this.documentElement.appendChild(this.head);
    this.documentElement.appendChild(this.body);
    this.activeElement = this.body;
  }
  Document.prototype = Object.create(Node.prototype);
  Document.prototype.createElement = function (tagName) {
    tagName = String(tagName).toLowerCase();
    if (tagName === 'canvas') {
      return new HTMLCanvasElement(this);
    }
    return new HTMLElement(tagName, this);
  };
  Document.prototype.getElementById = function (id) {
    var stack = [this.documentElement];
    while (stack.length > 0) {
      var n = stack.pop();
      if (n.id === id) {
        return n;
      }
      for (var i = n.children.length - 1; i >= 0; i--) {
        stack.push(n.children[i]);
      }
    }
    return null;
  };

  var document = new Document();

  // getComputedStyle for the properties that cytoscape reads. The values are
  // the values of Chromium for the container of mermaid: an element with no
  // style of its own but display:none, and position:relative from the style
  // sheet that cytoscape adds for its container class. An element that is not
  // in the document has no computed style: every value is the empty string.
  var computedDefaults = {
    'position': 'static',
    'padding-left': '0px',
    'padding-right': '0px',
    'padding-top': '0px',
    'padding-bottom': '0px',
    'border-left-width': '0px',
    'border-right-width': '0px',
    'border-top-width': '0px',
    'border-bottom-width': '0px',
    'font-size': '16px',
  };
  g.getComputedStyle = function (element) {
    return {
      getPropertyValue: function (name) {
        if (!element.isConnected) {
          return '';
        }
        if (name === 'position' && document.getElementById('__________cytoscape_stylesheet') !== null && (' ' + element.className + ' ').indexOf(' __________cytoscape_container ') >= 0) {
          return 'relative';
        }
        if (!Object.prototype.hasOwnProperty.call(computedDefaults, name)) {
          return unsupported('getComputedStyle property ' + name);
        }
        return computedDefaults[name];
      },
    };
  };

  g.window = g;
  g.document = document;
  g.navigator = { userAgent: host.userAgent };
  g.devicePixelRatio = 1;
  g.HTMLElement = HTMLElement;
  g.HTMLCanvasElement = HTMLCanvasElement;
  g.MutationObserver = MutationObserver;
  g.ResizeObserver = ResizeObserver;
  g.addEventListener = EventTarget.prototype.addEventListener;
  g.removeEventListener = EventTarget.prototype.removeEventListener;
  document.addEventListener = EventTarget.prototype.addEventListener;
  document.removeEventListener = EventTarget.prototype.removeEventListener;

  var silent = function () {};
  g.console = { log: silent, info: silent, warn: silent, error: silent, debug: silent, trace: silent, group: silent, groupCollapsed: silent, groupEnd: silent, assert: silent, time: silent, timeEnd: silent };

  return {
    document: document,
    notifyMutationObservers: notifyMutationObservers,
    // pendingTasks is the number of timers and frames that wait.
    pendingTasks: function () { return tasks.length; },
    // flush runs the timers and frames in the order of their time, and a
    // frame every 16 ms, until none is left or the limit is reached. It
    // returns the number of tasks that still wait.
    flush: function (limit) {
      var steps = 0;
      while (tasks.length > 0 && steps < limit) {
        var best = 0;
        for (var i = 1; i < tasks.length; i++) {
          var a = tasks[i], b = tasks[best];
          var at = a.kind === 'frame' ? virtualTime : a.at;
          var bt = b.kind === 'frame' ? virtualTime : b.at;
          if (at < bt) {
            best = i;
          }
        }
        var task = tasks.splice(best, 1)[0];
        if (task.kind === 'frame') {
          // The frames that wait now run together, as in one browser frame.
          var frames = [task];
          for (var j = 0; j < tasks.length;) {
            if (tasks[j].kind === 'frame') {
              frames.push(tasks.splice(j, 1)[0]);
            } else {
              j++;
            }
          }
          virtualTime += 16;
          for (var k = 0; k < frames.length; k++) {
            frames[k].fn.call(g, virtualTime);
          }
        } else {
          if (task.at > virtualTime) {
            virtualTime = task.at;
          }
          if (task.kind === 'interval') {
            unsupported('setInterval in flush');
          }
          task.fn.apply(g, task.args || []);
        }
        steps++;
      }
      return tasks.length;
    },
  };
})
